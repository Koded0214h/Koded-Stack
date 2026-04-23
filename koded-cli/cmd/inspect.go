package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Koded0214h/koded-cli/pkg/types"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <package>",
	Short: "Show metadata for a package",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgName := args[0]

		// Flags
		osFlag, _ := cmd.Flags().GetString("os")
		archFlag, _ := cmd.Flags().GetString("arch")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		versionFlag, _ := cmd.Flags().GetString("version")

		// Determine OS/Arch
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
			fmt.Printf("Error: could not find manifest for package '%s'\n", pkgName)
			return
		}

		var manifest types.Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			fmt.Println("Error: could not parse manifest")
			return
		}

		// Version check
		if versionFlag != "" && versionFlag != manifest.Version {
			fmt.Printf("Error: version %s not found (manifest version: %s)\n", versionFlag, manifest.Version)
			return
		}

		// Resolve source
		key := fmt.Sprintf("%s-%s", targetOS, targetArch)
		source, ok := manifest.Sources[key]

		if !ok {
			fmt.Printf("Warning: no source for OS/Arch %s\n", key)
			fmt.Println("Available sources:")
			for k := range manifest.Sources {
				fmt.Printf(" - %s\n", k)
			}
			// fallback: pick the first available source
			for _, s := range manifest.Sources {
				source = s
				break
			}
			fmt.Println("Using first available source as fallback.")
		}

		// Output
		if jsonFlag {
			out := map[string]interface{}{
				"name":    manifest.Name,
				"version": manifest.Version,
				"os":      targetOS,
				"arch":    targetArch,
				"size":    types.HumanSize(manifest.Size),
				"url":     source.URL,
				"bin":     manifest.Install.Bin,
			}
			encoded, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(encoded))
		} else {
			fmt.Printf("Package: %s\n", manifest.Name)
			fmt.Printf("Version: %s\n", manifest.Version)
			fmt.Printf("OS/Arch: %s/%s\n", targetOS, targetArch)
			fmt.Printf("Size: %s\n", types.HumanSize(manifest.Size))
			fmt.Printf("URL: %s\n", source.URL)
			fmt.Printf("Binaries: %v\n", manifest.Install.Bin)
		}
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)
	inspectCmd.Flags().String("os", "", "Override OS for inspection")
	inspectCmd.Flags().String("arch", "", "Override architecture for inspection")
	inspectCmd.Flags().String("version", "", "Specific package version")
	inspectCmd.Flags().Bool("json", false, "Output in JSON format")
}