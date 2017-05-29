package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/parnurzeal/gorequest"
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

		dbList := getDBList()
		for _, j := range dbList {
			fmt.Printf("%s\t%d bytes\t%s\n", j.Database, j.Size, j.LastModified)
		}
		fmt.Println()
	},
}

func init() {
	RootCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// listCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// listCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func getDBList() []dbListEntry {
	resp, body, errs := gorequest.New().Get(cloud + "/db_list").End()
	if errs != nil {
		log.Print("Errors when retrieving the database list:")
		for _, err := range errs {
			log.Print(err.Error())
		}
		return []dbListEntry{}
	}
	defer resp.Body.Close()
	var dbList []dbListEntry
	err := json.Unmarshal([]byte(body), &dbList)
	if err != nil {
		log.Printf("Error retrieving database list: '%v'\n", err.Error())
		return []dbListEntry{}
	}
	return dbList
}
