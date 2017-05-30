package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Reverts a database to a prior commit in its history
var branchRevertCmd = &cobra.Command{
	Use:   "revert",
	Short: "Resets a database branch back to a previous commit",
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

		// Ensure a branch name and commit ID were given
		if branch == "" {
			return errors.New("No branch name given")
		}
		if commit == "" {
			return errors.New("No commit ID given")
		}

		// Revert the branch
		file := args[0]
		resp, _, errs := rq.New().Post(cloud+"/branch_revert").
			Set("branch", branch).
			Set("commit", commit).
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when reverting branch:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when reverting branch")
		}
		if resp.StatusCode != http.StatusNoContent {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database or commit not found")
			}
			return errors.New(fmt.Sprintf("Branch revertion failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Println("Branch reverted")
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchRevertCmd)
	branchRevertCmd.Flags().StringVar(&branch, "branch", "master", "Remote branch to operate on")
	branchRevertCmd.Flags().StringVar(&commit, "commit", "", "Commit ID for the to revert to")
}
