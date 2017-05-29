package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Displays the list of branches for a remote database
var branchListCmd = &cobra.Command{
	Use:   "list",
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
		if resp.StatusCode != 200 {
			if resp.StatusCode == 404 {
				return errors.New("Requested database not found")
			}
			return errors.New(fmt.Sprintf("Branch list failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}
		list := make(map[string]string)
		err := json.Unmarshal([]byte(body), &list)
		if err != nil {
			return err
		}

		// Display the list of branches
		fmt.Printf("Branches for %s:\n\n", file)
		for i, j := range list {
			fmt.Printf("* %s : commit %s\n", i, j)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchListCmd)
}