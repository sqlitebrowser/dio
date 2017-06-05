package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Displays the list of tags for a remote database
var tagListCmd = &cobra.Command{
	Use:   "tags [database name]",
	Short: "Displays a list of tags for a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a database file was given
		if len(args) == 0 {
			return errors.New("No database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one database can be worked with at a time (for now)")
		}

		// Retrieve the list of tags
		file := args[0]
		resp, body, errs := rq.New().Get(cloud+"/tag_list").
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when retrieving tag list:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when retrieving tag list")
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database not found")
			}
			return errors.New(fmt.Sprintf("Tag list failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}
		list := make(map[string]tagEntry)
		err := json.Unmarshal([]byte(body), &list)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Printf("Database %s has no tags\n", file)
			return nil
		}

		// Sort the list alphabetically
		var sortedKeys []string
		for k := range list {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Strings(sortedKeys)

		// Display the list of tags
		fmt.Printf("Tags for %s:\n\n", file)
		for _, i := range sortedKeys {
			if list[i].TagType == SIMPLE {
				fmt.Printf("  * %s : commit %s\n", i, list[i].Commit)
			} else {
				fmt.Printf("  * %s : commit %s\n\n", i, list[i].Commit)
				fmt.Printf("      Author: %s <%s>\n", list[i].TaggerName, list[i].TaggerEmail)
				fmt.Printf("      Date: %s\n", list[i].Date.Format(time.UnixDate))
				fmt.Printf("      Message: %s\n", list[i].Message)
			}
		}
		fmt.Println()
		return nil
	},
}

func init() {
	RootCmd.AddCommand(tagListCmd)
}
