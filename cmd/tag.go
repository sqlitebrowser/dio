package cmd

import (
	"github.com/spf13/cobra"
)

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Manipulate tags for a database",
}

func init() {
	RootCmd.AddCommand(tagCmd)
}
