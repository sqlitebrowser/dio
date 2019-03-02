package cmd

import (
	"github.com/spf13/cobra"
)

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Create and remove releases for a database",
}

func init() {
	RootCmd.AddCommand(releaseCmd)
}
