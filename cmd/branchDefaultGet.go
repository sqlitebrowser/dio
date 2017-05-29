package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Returns the name of the default branch for a database
var branchDefaultGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get the default branch name for a database",
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

		// Retrieve the default branch name
		file := args[0]
		resp, body, errs := rq.New().Get(cloud+"/branch_default_get").
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when retrieving default branch:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when retrieving default branch")
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database not found")
			}
			return errors.New(fmt.Sprintf(
				"Retrieving default branch failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Printf("Default branch: %s\n", body)
		return nil
	},
}

func init() {
	branchDefaultCmd.AddCommand(branchDefaultGetCmd)
}
