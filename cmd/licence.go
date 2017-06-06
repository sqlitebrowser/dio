package cmd

import (
	"github.com/spf13/cobra"
)

// licenceCmd represents the licence command
var licenceCmd = &cobra.Command{
	Use:   "licence",
	Short: "Work with licences for a database",
}

func init() {
	RootCmd.AddCommand(licenceCmd)
}
