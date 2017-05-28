package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// branchRemoveCmd represents the branchRemove command
var branchRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Removes a branch from a database",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("branch remove called")
	},
}

func init() {
	branchCmd.AddCommand(branchRemoveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// branchRemoveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// branchRemoveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
