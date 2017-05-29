package cmd

import (
	"github.com/spf13/cobra"
)

var branchDefaultCmd = &cobra.Command{
	Use:   "default",
	Short: "Get and set the default branch for a database",
}

func init() {
	branchCmd.AddCommand(branchDefaultCmd)
}
