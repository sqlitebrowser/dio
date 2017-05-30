package cmd

import (
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Create and remove tags for a database",
}

func init() {
	RootCmd.AddCommand(tagCmd)
}
