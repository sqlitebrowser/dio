package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// tagRemoveCmd represents the tagRemove command
var tagRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a tag from a database",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("tag remove called")
	},
}

func init() {
	tagCmd.AddCommand(tagRemoveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tagRemoveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tagRemoveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
