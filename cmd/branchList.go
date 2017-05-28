package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// branchListCmd represents the branchList command
var branchListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the branches for your database on the cloud",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("branch list called")
	},
}

func init() {
	branchCmd.AddCommand(branchListCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// branchListCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// branchListCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
