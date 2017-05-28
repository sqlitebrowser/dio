package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// tagListCmd represents the tagList command
var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "Displays a list of tags for a database",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("tag list called")
	},
}

func init() {
	tagCmd.AddCommand(tagListCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tagListCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tagListCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
