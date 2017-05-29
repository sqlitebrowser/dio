package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type dbListEntry struct {
	Database     string    `json:"database"`
	LastModified time.Time `json:"last_modified"`
	Size         int       `json:"size"`
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Returns a list of databases available on a remote cloud",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Databases on %s\n\n", cloud)
		fmt.Println("Name\tSize\t\tLast Modified")
		fmt.Println("****\t****\t\t*************")

		list := getDBList()
		for _, j := range list {
			fmt.Printf("%s\t%d bytes\t%s\n", j.Database, j.Size, j.LastModified)
		}
		fmt.Println()
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
