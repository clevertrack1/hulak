// Package cmd contains the command-line interface for the application.
// This package uses both cobra for command-line parsing and viper for configuration management.
//
// Viper is used to read configuration values from multiple sources in the following order of precedence:
// 1. Command-line flags (highest precedence)
// 2. Environment variables (prefixed with HULAK_)
// 3. Configuration file (hulak.yaml in $HOME/.hulak/ or current directory)
// 4. Default values (lowest precedence)
//
// Example configuration file (hulak.yaml):
//
//	env: global
//	debug: true
//	dir: /path/to/directory
//
// Example environment variables:
//
//	HULAK_ENV=global
//	HULAK_DEBUG=true
//	HULAK_DIR=/path/to/directory
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	userflags "github.com/xaaha/hulak/pkg/userFlags"
	"github.com/xaaha/hulak/pkg/utils"
)

// Declare these functions to be imported from main package
var (
	InitializeProject func(env string) map[string]any
	HandleAPIRequests func(secretsMap map[string]any, debug bool, filePathList []string, dir, dirseq, fp string)
)

var (
	// Flags
	env    string
	fp     string
	file   string
	debug  bool
	dir    string
	dirseq string

	// Root command
	rootCmd = &cobra.Command{
		Use:   "hulak",
		Short: "Hulak is a CLI tool for making API requests",
		Long: `Hulak is a CLI tool for making API requests.
It supports various file formats and environments.
For more information, visit https://github.com/xaaha/hulak`,
		Run: func(cmd *cobra.Command, args []string) {
			// Get values from viper (which will use environment variables if available)
			envValue := viper.GetString("env")
			fpValue := viper.GetString("fp")
			fileValue := viper.GetString("file")
			debugValue := viper.GetBool("debug")
			dirValue := viper.GetString("dir")
			dirseqValue := viper.GetString("dirseq")

			// Check if any file or directory flag is provided
			hasDirFlags := dirValue != "" || dirseqValue != ""
			hasFileFlags := fpValue != "" || fileValue != ""

			if !hasFileFlags && !hasDirFlags {
				utils.PrintWarning("No file or directory specified. Use -file, -fp, -dir, or -dirseq flags.")
				_ = cmd.Help()
				return
			}

			// Initialize project environment
			envMap := InitializeProject(envValue)

			var filePathList []string
			var err error

			if hasFileFlags {
				filePathList, err = userflags.GenerateFilePathList(fileValue, fpValue)
				if err != nil {
					// Only panic if no directory flags are provided
					if !hasDirFlags {
						utils.PanicRedAndExit("%v", err)
					} else {
						// When directory flags are present, just warn about the file flag error
						utils.PrintWarning(fmt.Sprintf("Warning with file flags: %v", err))
					}
				}
			}

			// Handle API requests
			HandleAPIRequests(envMap, debugValue, filePathList, dirValue, dirseqValue, fpValue)
		},
	}
)

// Execute executes the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Define flags
	rootCmd.PersistentFlags().StringVarP(&env, "env", "e", utils.DefaultEnvVal, "environment file to use during the call")
	rootCmd.PersistentFlags().StringVarP(&fp, "fp", "", "", "Relative (or absolute) file path (fp) of the request file from the environment directory")
	rootCmd.PersistentFlags().StringVarP(&file, "file", "f", "", "File name for making an api request. File name is case-insensitive")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug mode to get the entire request, response, headers, and other info for the API call")
	rootCmd.PersistentFlags().StringVarP(&dir, "dir", "", "", "Directory path to run concurrent")
	rootCmd.PersistentFlags().StringVarP(&dirseq, "dirseq", "", "", "Directory path to run in alphabetical order")

	// Initialize viper
	viper.SetEnvPrefix("HULAK") // Environment variables will be prefixed with HULAK_
	viper.AutomaticEnv()        // Read environment variables

	// Set up config file support
	viper.SetConfigName("hulak")        // Name of config file (without extension)
	viper.SetConfigType("yaml")         // YAML format for config file
	viper.AddConfigPath("$HOME/.hulak") // Look for config in the home directory
	viper.AddConfigPath(".")            // Look for config in the current directory

	// Read in config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// But log other errors
			_, _ = fmt.Fprintf(os.Stderr, "Error reading config file: %s\n", err)
		}
	}

	// Bind flags to viper
	_ = viper.BindPFlag("env", rootCmd.PersistentFlags().Lookup("env"))
	_ = viper.BindPFlag("fp", rootCmd.PersistentFlags().Lookup("fp"))
	_ = viper.BindPFlag("file", rootCmd.PersistentFlags().Lookup("file"))
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("dir", rootCmd.PersistentFlags().Lookup("dir"))
	_ = viper.BindPFlag("dirseq", rootCmd.PersistentFlags().Lookup("dirseq"))
}
