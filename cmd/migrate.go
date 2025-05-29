package cmd

import (
	"github.com/spf13/cobra"
	"github.com/xaaha/hulak/pkg/migration"
	"github.com/xaaha/hulak/pkg/utils"
)

// migrateCmd represents the migrate command
var migrateCmd = &cobra.Command{
	Use:   "migrate [file1] [file2] ...",
	Short: "Migrate Postman collections and environments",
	Long:  `Migrate Postman collections and environments to hulak format`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		err := migration.CompleteMigration(args)
		if err != nil {
			utils.PanicRedAndExit("%v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}