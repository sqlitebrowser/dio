package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// branchHistoryCmd represents the branchHistory command
var branchHistoryCmd = &cobra.Command{
	Use:   "log",
	Short: "Displays the history for a database branch",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("branch log called")
	},
}

func init() {
	branchCmd.AddCommand(branchHistoryCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// branchHistoryCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// branchHistoryCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
