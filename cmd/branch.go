package cmd

import (
	"github.com/spf13/cobra"
)

// branchCmd represents the branch command
var branchCmd = &cobra.Command{
	Use:   "branch",
	Short: "Work with branches for a database",
}

func init() {
	RootCmd.AddCommand(branchCmd)
}
