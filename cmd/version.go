package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version number
const version = "v0.1.2"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of hulak",
	Long:  `Print the version number of hulak`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
