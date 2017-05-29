package cmd

import (
	"errors"
	"fmt"
	"log"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Creates a branch for a database
var branchCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a branch for a database",
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

		// Ensure a new branch name and commit ID were given
		if branch == "" {
			return errors.New("No branch name given")
		}
		if commit == "" {
			return errors.New("No commit ID given")
		}

		// Create the branch
		file := args[0]
		resp, _, errs := rq.New().Post(cloud+"/branch_create").
			Set("branch", branch).
			Set("commit", commit).
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when creating branch:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when creating branch")
		}
		if resp.StatusCode != 204 {
			if resp.StatusCode == 404 {
				return errors.New("Requested database or commit not found")
			}
			if resp.StatusCode == 409 {
				return errors.New("Requested branch already exists")
			}
			return errors.New(fmt.Sprintf("Branch creation failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Println("Branch creation succeeded")
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchCreateCmd)
	branchCreateCmd.Flags().StringVar(&branch, "branch", "master", "Remote branch to operate on")
	branchCreateCmd.Flags().StringVar(&commit, "commit", "", "Commit ID for the new branch head")
}
