package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Creates a tag for a database
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays the version of dio being run",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("dio version %s\n", DIO_VERSION)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
