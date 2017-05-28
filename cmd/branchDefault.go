package cmd

import (
	"github.com/spf13/cobra"
)

// branchDefaultCmd represents the branchDefault command
var branchDefaultCmd = &cobra.Command{
	Use:   "default",
	Short: "Get and set the default branch for a database",
}

func init() {
	branchCmd.AddCommand(branchDefaultCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// branchDefaultCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// branchDefaultCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
