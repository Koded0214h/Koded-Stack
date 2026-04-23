package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.0.1"

var (
	short bool
	jsonOut bool
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the koded version",
	Run: func(cmd *cobra.Command, args []string) {

		// --short
		if short {
			fmt.Println(Version)
			return
		}

		// --json
		if jsonOut {
			out := map[string]string{
				"name":    "koded",
				"version": Version,
			}
			data, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(data))
			return
		}

		// default
		fmt.Printf("koded version %s\n", Version)
	},
}

func init() {
	versionCmd.Flags().BoolVar(&short, "short", false, "Print only the version number")
	versionCmd.Flags().BoolVar(&jsonOut, "json", false, "Print version info as JSON")

	rootCmd.AddCommand(versionCmd)
}
