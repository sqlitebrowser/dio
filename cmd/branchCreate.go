package cmd

import (
	"errors"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Creates a branch for a database
var branchCreateCmd = &cobra.Command{
	Use:   "create [database name] --branch xxx --commit yyy",
	Short: "Create a branch for a database",
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
		req := rq.New().Post(cloud+"/branch_create").
			Set("branch", branch).
			Set("commit", commit).
			Set("database", file)
		if msg != "" {
			req.Set("desc", msg)
		}
		resp, _, errs := req.End()
		if errs != nil {
			log.Print("Errors when creating branch:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when creating branch")
		}
		if resp.StatusCode != http.StatusNoContent {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database or commit not found")
			}
			if resp.StatusCode == http.StatusConflict {
				return errors.New("Requested branch already exists")
			}
			return errors.New(fmt.Sprintf("Branch creation failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Printf("Branch '%s' created\n", branch)
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchCreateCmd)
	branchCreateCmd.Flags().StringVar(&branch, "branch", "", "Name of remote branch to create")
	branchCreateCmd.Flags().StringVar(&commit, "commit", "", "Commit ID for the new branch head")
	branchCreateCmd.Flags().StringVar(&msg, "description", "", "Description of the branch")
}
