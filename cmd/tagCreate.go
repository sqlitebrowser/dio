package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// tagCreateCmd represents the tagCreate command
var tagCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a tag for a database",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("tag create called")
	},
}

func init() {
	tagCmd.AddCommand(tagCreateCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tagCreateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tagCreateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
