package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// branchCreateCmd represents the branchCreate command
var branchCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a branch for a database",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("branch create called")
	},
}

func init() {
	branchCmd.AddCommand(branchCreateCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// branchCreateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// branchCreateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
