// pkg/types/types.go
package types

import (
	"fmt"
	"time"
)

// DownloadState represents the state of a resumable download
type DownloadState struct {
	CompletedChunks map[int]bool `json:"completed_chunks"`
	TotalChunks     int          `json:"total_chunks"`
	URL             string       `json:"url,omitempty"`
	StartedAt       time.Time    `json:"started_at,omitempty"`
	LastUpdated     time.Time    `json:"last_updated,omitempty"`
}

// Source defines a download source for a specific OS/Architecture
type Source struct {
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	Format  string `json:"format,omitempty"`   // tar.gz, zip, binary, etc.
	OS      string `json:"os,omitempty"`       // Override OS
	Arch    string `json:"arch,omitempty"`     // Override Arch
}

// Install defines how to install a package after downloading
type Install struct {
	Type     string   `json:"type"`               // archive, binary, script
	Bin      []string `json:"bin"`                // binary names to extract
	PostInst string   `json:"post_inst,omitempty"` // post-installation script
	Test     string   `json:"test,omitempty"`     // test command to verify installation
}

// Manifest is the main package definition
type Manifest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	License     string            `json:"license,omitempty"`
	Author      string            `json:"author,omitempty"`
	Size        int64             `json:"size"` // total size in bytes
	Sources     map[string]Source `json:"sources"`
	Install     Install           `json:"install"`
	Tags        []string          `json:"tags,omitempty"` // e.g., ["compiler", "tools"]
}

// PackageInfo represents installed package information
type PackageInfo struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Path        string    `json:"path"`
	InstalledAt time.Time `json:"installed_at"`
	Size        int64     `json:"size"`
	SourceURL   string    `json:"source_url"`
	Checksum    string    `json:"checksum"`
}

// DownloadProgress represents real-time download progress
type DownloadProgress struct {
	BytesDownloaded int64   `json:"bytes_downloaded"`
	BytesTotal      int64   `json:"bytes_total"`
	Percentage      float64 `json:"percentage"`
	Speed           int64   `json:"speed"`   // bytes per second
	TimeRemaining   int64   `json:"time_remaining"` // seconds
	ActiveChunks    int     `json:"active_chunks"`
}

// Platform represents an OS/Architecture combination
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// Config holds koded configuration
type Config struct {
	CacheDir        string `json:"cache_dir"`
	InstallDir      string `json:"install_dir"`
	MaxConcurrent   int    `json:"max_concurrent"`
	ChunkSize       int64  `json:"chunk_size"`
	EnableChecksum  bool   `json:"enable_checksum"`
	DefaultPlatform Platform `json:"default_platform"`
}

// ErrorResponse standardizes error responses
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

func HumanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	if exp > 5 {
		exp = 5
	}
	pre := "KMGTPE"[exp : exp+1]
	return fmt.Sprintf("%.2f %sB", float64(bytes)/float64(div), pre)
}

// PlatformKey creates a platform key string (e.g., "darwin-arm64")
func PlatformKey(os, arch string) string {
	return fmt.Sprintf("%s-%s", os, arch)
}

// ParsePlatformKey parses a platform key into OS and Arch
func ParsePlatformKey(key string) (os, arch string, ok bool) {
	for i := 0; i < len(key); i++ {
		if key[i] == '-' {
			return key[:i], key[i+1:], true
		}
	}
	return "", "", false
}

// ValidateManifest validates a manifest's required fields
func ValidateManifest(m Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest missing required field: name")
	}
	if m.Version == "" {
		return fmt.Errorf("manifest missing required field: version")
	}
	if len(m.Sources) == 0 {
		return fmt.Errorf("manifest missing required field: sources")
	}
	if m.Install.Type == "" {
		return fmt.Errorf("manifest missing required field: install.type")
	}
	return nil
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		CacheDir:       ".koded/cache",
		InstallDir:     ".koded/bin",
		MaxConcurrent:  4,
		ChunkSize:      8 * 1024 * 1024, // 8MB
		EnableChecksum: true,
		DefaultPlatform: Platform{
			OS:   "darwin",
			Arch: "arm64",
		},
	}
}