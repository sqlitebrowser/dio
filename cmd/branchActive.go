package cmd

import (
	"github.com/spf13/cobra"
)

var branchActiveCmd = &cobra.Command{
	Use:   "active",
	Short: "Get and set the active branch for a database",
}

func init() {
	branchCmd.AddCommand(branchActiveCmd)
}
