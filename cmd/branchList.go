package cmd

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sort"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Displays the list of branches for a remote database
var branchListCmd = &cobra.Command{
	Use:   "list [database name]",
	Short: "List the branches for your database on a DBHub.io cloud",
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

		// Retrieve the list of branches
		file := args[0]
		resp, body, errs := rq.New().Get(cloud+"/branch_list").
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when retrieving branch list:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when retrieving branch list")
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database not found")
			}
			return errors.New(fmt.Sprintf("Branch list failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		list := make(map[string]branchEntry)
		err := json.Unmarshal([]byte(body), &list)
		if err != nil {
			return err
		}

		// Sort the list alphabetically
		var sortedKeys []string
		for k := range list {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Strings(sortedKeys)

		// Display the list of branches
		fmt.Printf("Branches for %s:\n\n", file)
		for _, i := range sortedKeys {
			fmt.Printf("  * %s - Commit: %s\n", i, list[i].Commit)
			if list[i].Description != "" {
				fmt.Printf("\n      %s\n\n", list[i].Description)
			}
		}
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchListCmd)
}
