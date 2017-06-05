package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

var branchDefaultBranch string

// Sets the default branch for a database
var branchDefaultSetCmd = &cobra.Command{
	Use:   "set [database name] --branch xxx",
	Short: "Set the default branch for a database",
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
		if branchDefaultBranch == "" {
			return errors.New("No branch name given")
		}

		// Set the default branch
		file := args[0]
		resp, _, errs := rq.New().Post(cloud+"/branch_default_change").
			Set("branch", branchDefaultBranch).
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when setting default branch:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when setting default branch")
		}
		if resp.StatusCode != http.StatusNoContent {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database or branch not found")
			}
			return errors.New(fmt.Sprintf(
				"Setting default branch failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Printf("Branch '%s' set as default for '%s'\n", branchDefaultBranch, file)
		return nil
	},
}

func init() {
	branchDefaultCmd.AddCommand(branchDefaultSetCmd)
	branchDefaultSetCmd.Flags().StringVar(&branchDefaultBranch, "branch", "",
		"Remote branch to set as default")
}
