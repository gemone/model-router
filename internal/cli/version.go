package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  `Display the version, commit, and build date of model-router.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(GetVersion())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
