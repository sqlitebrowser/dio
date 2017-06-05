package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

var logBranch string

// Retrieves the commit history for a database branch
var branchLog = &cobra.Command{
	Use:   "log [database name]",
	Short: "Displays the history for a database branch",
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

		// Retrieve the branch history
		var history struct {
			Branch  string
			Entries []commitEntry
		}
		file := args[0]
		resp, body, errs := rq.New().Get(cloud+"/branch_history").
			Set("branch", logBranch).
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when retrieving branch history:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when retrieving branch history")
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database or branch not found")
			}
			return errors.New(fmt.Sprintf("Branch history failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}
		err := json.Unmarshal([]byte(body), &history)
		if err != nil {
			return err
		}

		// Display the branch history
		fmt.Printf("Branch \"%s\" history for %s:\n\n", history.Branch, file)
		for _, j := range history.Entries {
			fmt.Printf(createCommitText(j))
			if j.Message != "" {
				fmt.Println()
			}
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(branchLog)
	branchLog.Flags().StringVar(&logBranch, "branch", "", "Remote branch to retrieve history of")
}

// Creates the user visible commit text for a commit.
func createCommitText(c commitEntry) string {
	s := fmt.Sprintf("  commit %s\n", c.ID)
	s += fmt.Sprintf("  Author: %s <%s>\n", c.AuthorName, c.AuthorEmail)
	s += fmt.Sprintf("  Date: %v\n\n", c.Timestamp.Format(time.UnixDate))
	if c.Message != "" {
		s += fmt.Sprintf("      %s\n", c.Message)
	}
	return s
}
