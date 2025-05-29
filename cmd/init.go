package cmd

import (
	"embed"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xaaha/hulak/pkg/envparser"
	"github.com/xaaha/hulak/pkg/utils"
)

//go:embed apiOptions.yaml
var embeddedFiles embed.FS

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize hulak",
	Long:  `Initialize hulak by creating default environment files and apiOptions.yaml`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if -env flag is present
		if createEnvs {
			if len(args) > 0 {
				for _, env := range args {
					if err := envparser.CreateDefaultEnvs(&env); err != nil {
						utils.PrintRed(err.Error())
					}
				}
			} else {
				utils.PrintWarning("No environment names provided after -env flag")
			}
		} else {
			if err := envparser.CreateDefaultEnvs(nil); err != nil {
				utils.PrintRed(err.Error())
			}

			apiOptionsFile := "apiOptions.yaml"
			content, err := embeddedFiles.ReadFile("apiOptions.yaml")
			if err != nil {
				utils.PrintRed(fmt.Sprintf("Error reading embedded file: %v", err))
				return
			}

			root, err := utils.CreatePath(apiOptionsFile)
			if err != nil {
				utils.PrintRed(fmt.Sprintf("Error creating path: %v", err))
				return
			}

			if err := os.WriteFile(root, content, utils.FilePer); err != nil {
				utils.PrintRed(fmt.Sprintf("Error on writing '%s' file: %s", apiOptionsFile, err))
				return
			}

			utils.PrintGreen(fmt.Sprintf("Created '%s': %s", apiOptionsFile, utils.CheckMark))
			utils.PrintGreen("Done " + utils.CheckMark)
		}
	},
}

var createEnvs bool

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVar(&createEnvs, "env", false, "Create environment files based on following arguments")
}
