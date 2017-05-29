package cmd

import (
	"errors"
	"fmt"
	"log"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Removes a branch from a database
var branchRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Removes a branch from a database",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		resp, _, errs := rq.New().Post(cloud+"/branch_remove").
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
		if resp.StatusCode != 204 {
			if resp.StatusCode == 404 {
				return errors.New("Requested database or commit not found")
			}
			return errors.New(fmt.Sprintf("Branch removal failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Println("Branch remove succeeded")
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchRemoveCmd)
	branchRemoveCmd.Flags().StringVar(&branch, "branch", "master", "Remote branch to operate on")
}
