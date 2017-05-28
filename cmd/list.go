package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Returns a list of databases available on a remote cloud",
	Run: func(cmd *cobra.Command, args []string) {
		dbList := getDBList()
		for _, j := range dbList {
			fmt.Printf("Database: %s\n", j)
		}
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

func getDBList() []string {
	resp, err := http.Get(cloud + "db_list")
	if err != nil {
		log.Printf("Error retrieving database list: '%v'\n", err.Error())
		return []string{}
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error retrieving database list: '%v'\n", err.Error())
		return []string{}
	}

	var dbList []string
	err = json.Unmarshal(body, &dbList)
	if err != nil {
		log.Printf("Error retrieving database list: '%v'\n", err.Error())
		return []string{}
	}
	return dbList
}
