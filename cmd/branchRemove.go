package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Removes a branch from a database
var branchRemoveCmd = &cobra.Command{
	Use:   "remove [database name] --branch xxx",
	Short: "Removes a branch from a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		var errorInfo struct {
			Condition string   `json:"error_condition"`
			Tags      []string `json:"tags"`
		}

		// Ensure a database file was given
		if len(args) == 0 {
			return errors.New("No database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one database can be changed at a time (for now)")
		}

		// Ensure a branch name was given
		if branch == "" {
			return errors.New("No branch name given")
		}

		// Remove the branch
		file := args[0]
		resp, body, errs := rq.New().Post(cloud+"/branch_remove").
			Set("branch", branch).
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when removing branch:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when removing branch")
		}
		if resp.StatusCode != http.StatusNoContent {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database or branch not found")
			}
			if resp.StatusCode == http.StatusConflict {
				// Check for a message body indicating there would be isolated tags
				err := json.Unmarshal([]byte(body), &errorInfo)
				if err != nil {
					return err
				}
				if errorInfo.Condition == "isolated_tags" {
					// Yep, isolated tags would exist if this branch is removed.  Let the user know they'll need to
					// remove the tags first
					// TODO: Add some kind of --force option which removes the tags itself then removes the branch
					e := "The following tags only exist on that branch.  You'll need to remove them first:\n\n"
					for _, j := range errorInfo.Tags {
						e += fmt.Sprintf(" * %s\n", j)
					}
					//e += fmt.Sprintf("\n")
					return errors.New(e)
				}

				// Nope, it wasn't an "isolated tags" problem.  Lets assume it was a "try to delete the default branch"
				// problem instead
				return errors.New("Default branch can't be removed.  Change the default branch first")
			}
			return errors.New(fmt.Sprintf("Branch removal failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Printf("Branch '%s' removed\n", branch)
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchRemoveCmd)
	branchRemoveCmd.Flags().StringVar(&branch, "branch", "", "Name of remote branch to remove")
}
