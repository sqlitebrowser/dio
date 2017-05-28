package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// branchRevertCmd represents the branchRevert command
var branchRevertCmd = &cobra.Command{
	Use:   "revert",
	Short: "Resets a database branch back to a previous commit",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("branch revert called")
	},
}

func init() {
	branchCmd.AddCommand(branchRevertCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// branchRevertCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// branchRevertCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
