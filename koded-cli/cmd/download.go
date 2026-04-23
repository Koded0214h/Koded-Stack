package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Koded0214h/koded-cli/pkg/types"
	"github.com/spf13/cobra"
)

// -------------------
// Cobra command
// -------------------

var downloadCmd = &cobra.Command{
	Use:   "download <package>",
	Short: "Download a package from manifest (supports pause/resume)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgName := args[0]

		osFlag, _ := cmd.Flags().GetString("os")
		archFlag, _ := cmd.Flags().GetString("arch")
		outputFlag, _ := cmd.Flags().GetString("output")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		// Resolve OS/arch
		targetOS := runtime.GOOS
		targetArch := runtime.GOARCH
		if osFlag != "" {
			targetOS = osFlag
		}
		if archFlag != "" {
			targetArch = archFlag
		}

		// Load manifest
		manifestPath := filepath.Join("manifests", pkgName+".json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			fmt.Printf("❌ Error: could not find manifest for package '%s'\n", pkgName)
			fmt.Printf("💡 Available packages: ")
			files, _ := filepath.Glob("manifests/*.json")
			for _, f := range files {
				fmt.Printf("%s ", strings.TrimSuffix(filepath.Base(f), ".json"))
			}
			fmt.Println()
			return
		}

		var manifest types.Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			fmt.Println("❌ Error: could not parse manifest")
			return
		}

		// Resolve source
		key := fmt.Sprintf("%s-%s", targetOS, targetArch)
		source, ok := manifest.Sources[key]
		if !ok {
			fmt.Printf("⚠️  Warning: no source for OS/Arch %s\n", key)
			fmt.Println("📦 Available sources:")
			for k := range manifest.Sources {
				fmt.Printf("   - %s\n", k)
			}
			// Fallback to first available
			for _, s := range manifest.Sources {
				source = s
				break
			}
			fmt.Printf("📥 Using: %s\n", key)
		}

		// Determine output file
		cacheDir := filepath.Join(".koded", "cache")
		os.MkdirAll(cacheDir, os.ModePerm)
		outputFile := outputFlag
		if outputFile == "" {
			outputFile = filepath.Join(cacheDir, fmt.Sprintf("%s-%s.tar.gz", pkgName, manifest.Version))
		}

		// Dry run mode
		if dryRun {
			fmt.Println("🚀 DRY RUN - No files will be downloaded")
			fmt.Println("📦 Package:", manifest.Name)
			fmt.Println("🏷️  Version:", manifest.Version)
			fmt.Println("💾 Size:", types.HumanSize(source.Size))
			fmt.Println("🌐 URL:", source.URL)
			fmt.Println("📁 Output:", outputFile)
			fmt.Println("🔐 SHA256:", source.SHA256[:16]+"...")
			return
		}

		// Start download
		fmt.Printf("📦 Downloading %s v%s (%s)...\n",
			manifest.Name, manifest.Version, types.HumanSize(source.Size))
		fmt.Printf("🌐 From: %s\n", source.URL)

		if err := downloadWithResume(source.URL, outputFile, source.Size, source.SHA256); err != nil {
			fmt.Printf("❌ Download failed: %v\n", err)
			return
		}

		fmt.Printf("✅ Download completed: %s\n", outputFile)
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
	downloadCmd.Flags().String("os", "", "Override OS for download")
	downloadCmd.Flags().String("arch", "", "Override architecture for download")
	downloadCmd.Flags().String("output", "", "Target output file (optional)")
	downloadCmd.Flags().Bool("dry-run", false, "Show what would be downloaded without downloading")
}

// -------------------
// Download logic with fixed progress bar
// -------------------

type DownloadState struct {
	CompletedChunks map[int]bool `json:"completed_chunks"`
	TotalChunks     int          `json:"total_chunks"`
}

func downloadWithResume(url, output string, totalSize int64, sha256sum string) error {
	const chunkSize = 2 * 1024 * 1024 // 2MB chunks for better progress updates
	totalChunks := int(totalSize / chunkSize)
	if totalSize%chunkSize != 0 {
		totalChunks++
	}

	stateFile := output + ".state.json"
	state := &DownloadState{
		CompletedChunks: make(map[int]bool),
		TotalChunks:     totalChunks,
	}

	// Load existing state
	if _, err := os.Stat(stateFile); err == nil {
		data, _ := os.ReadFile(stateFile)
		json.Unmarshal(data, &state)
		fmt.Println("🔄 Resuming previous download...")
	}

	var wg sync.WaitGroup
	var stateMutex sync.Mutex
	semaphore := make(chan struct{}, 4) // 4 parallel downloads
	retryLimit := 3

	// Create a progress ticker
	stopProgress := make(chan bool)
	startTime := time.Now()

	// Progress display goroutine
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stateMutex.Lock()
				done := len(state.CompletedChunks)
				stateMutex.Unlock()

				if done <= totalChunks {
					printCoolProgress(done, totalChunks, totalSize, startTime)
				}
			case <-stopProgress:
				// Final update
				stateMutex.Lock()
				done := len(state.CompletedChunks)
				stateMutex.Unlock()
				printCoolProgress(done, totalChunks, totalSize, startTime)
				return
			}
		}
	}()

	downloadErrors := make(chan error, totalChunks)
	completedChunks := 0

	for i := 0; i < totalChunks; i++ {
		if state.CompletedChunks[i] {
			completedChunks++
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(chunk int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			start := int64(chunk) * chunkSize
			end := start + chunkSize - 1
			if end >= totalSize {
				end = totalSize - 1
			}

			partFile := fmt.Sprintf("%s.part%d", output, chunk)
			var err error
			for attempt := 1; attempt <= retryLimit; attempt++ {
				err = downloadChunk(url, partFile, start, end)
				if err == nil {
					break
				}
				if attempt < retryLimit {
					time.Sleep(time.Duration(attempt) * time.Second)
				}
			}
			if err != nil {
				downloadErrors <- fmt.Errorf("chunk %d failed: %v", chunk, err)
				return
			}

			stateMutex.Lock()
			state.CompletedChunks[chunk] = true
			saveState(stateFile, state)
			completedChunks = len(state.CompletedChunks)
			stateMutex.Unlock()
		}(i)
	}

	// Wait for all downloads to complete
	wg.Wait()
	close(downloadErrors)
	stopProgress <- true

	// Check for errors
	var errors []string
	for err := range downloadErrors {
		errors = append(errors, err.Error())
	}
	if len(errors) > 0 {
		return fmt.Errorf("download failed: %s", strings.Join(errors, ", "))
	}

	// Final progress update
	printCoolProgress(completedChunks, totalChunks, totalSize, startTime)
	fmt.Println()

	// Merge chunks
	fmt.Print("🔗 Merging chunks... ")
	if err := mergeChunks(output, totalChunks); err != nil {
		fmt.Println("❌")
		return err
	}
	fmt.Println("✅")

	// Verify SHA256
	if sha256sum != "" {
		fmt.Print("🔐 Verifying SHA256... ")
		if ok := verifySHA256(output, sha256sum); !ok {
			fmt.Println("❌")
			return fmt.Errorf("SHA256 verification failed")
		}
		fmt.Println("✅")
	} else {
		fmt.Println("⚠️  No checksum provided, skipping verification")
	}

	// Cleanup
	fmt.Print("🧹 Cleaning up temporary files... ")
	for i := 0; i < totalChunks; i++ {
		os.Remove(fmt.Sprintf("%s.part%d", output, i))
	}
	os.Remove(stateFile)
	fmt.Println("✅")

	// Show stats
	duration := time.Since(startTime)
	speed := float64(totalSize) / duration.Seconds()
	fmt.Printf("⏱️  Time: %v | 📊 Speed: %s/s\n",
		duration.Round(time.Second),
		types.HumanSize(int64(speed)))

	return nil
}

func downloadChunk(url, output string, start, end int64) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %s for range %d-%d", resp.Status, start, end)
	}

	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func saveState(stateFile string, state *DownloadState) {
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(stateFile, data, 0644)
}

func mergeChunks(output string, totalChunks int) error {
	outFile, err := os.Create(output)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for i := 0; i < totalChunks; i++ {
		partFile := fmt.Sprintf("%s.part%d", output, i)
		f, err := os.Open(partFile)
		if err != nil {
			return err
		}
		_, err = io.Copy(outFile, f)
		f.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

func printCoolProgress(done, totalChunks int, totalBytes int64, startTime time.Time) {
	const chunkSize = 2 * 1024 * 1024

	// Calculate completed bytes
	completedBytes := int64(done) * chunkSize
	if completedBytes > totalBytes {
		completedBytes = totalBytes
	}

	// Calculate percentage (0-1)
	percent := 0.0
	if totalBytes > 0 {
		percent = float64(completedBytes) / float64(totalBytes)
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}

	// Build progress bar (40 characters wide)
	width := 40
	filled := int(float64(width) * percent)
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	bar := "["
	if filled > 0 {
		bar += strings.Repeat("=", filled)
	}
	if filled < width {
		bar += strings.Repeat(" ", width-filled)
	}
	bar += "]"

	// Calculate speed and ETA
	speedStr := "---"
	etaStr := "---"

	if percent > 0 && time.Since(startTime).Seconds() > 0 {
		elapsed := time.Since(startTime).Seconds()
		speed := float64(completedBytes) / elapsed
		speedStr = types.HumanSize(int64(speed)) + "/s"

		if speed > 0 {
			remainingBytes := totalBytes - completedBytes
			eta := time.Duration(float64(remainingBytes)/speed) * time.Second
			if eta < time.Hour {
				etaStr = fmt.Sprintf("%02d:%02d", int(eta.Minutes()), int(eta.Seconds())%60)
			} else {
				etaStr = fmt.Sprintf("%02d:%02d:%02d", int(eta.Hours()), int(eta.Minutes())%60, int(eta.Seconds())%60)
			}
		}
	}

	fmt.Printf("\r%s %.1f%% | 📦 %s/%s | 🚀 %s | ⏱️ %s",
		bar,
		percent*100,
		types.HumanSize(completedBytes),
		types.HumanSize(totalBytes),
		speedStr,
		etaStr)
}

func verifySHA256(filePath, expected string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}

	sum := hex.EncodeToString(h.Sum(nil))
	return sum == expected
}