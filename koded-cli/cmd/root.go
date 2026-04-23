package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "koded",
	Short: "The Koded Stack CLI — one console for the whole stack",
	Long: `koded is the unified command-line tool for the Koded Stack.

  koded db         → KodedDB interactive query shell
  koded protocol   → HTTP/K.0 protocol tester
  koded download   → Download packages from manifest
  koded inspect    → Inspect package manifests
  koded version    → Print version info

Built in Go. Part of the Koded Stack — protocol, database, browser, CLI.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags go here
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}