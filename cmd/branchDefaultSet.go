package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// branchDefaultSetCmd represents the branchDefaultSet command
var branchDefaultSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set the default branch for a database",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("branch default set called")
	},
}

func init() {
	branchDefaultCmd.AddCommand(branchDefaultSetCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// branchDefaultSetCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// branchDefaultSetCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
