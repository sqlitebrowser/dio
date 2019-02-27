package cmd

import (
	"encoding/json"
	"errors"
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
			return errors.New("no database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("only one database can be worked with at a time (for now)")
		}

		// Retrieve the branch history
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
			return errors.New("error when retrieving branch history")
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("requested database or branch not found")
			}
			return errors.New(fmt.Sprintf("Branch history failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}
		var history branchEntries
		err := json.Unmarshal([]byte(body), &history)
		if err != nil {
			return err
		}

		// Retrieve the list of known licences
		l, err := getLicences()
		if err != nil {
			return err
		}

		// Map the license sha256's to their friendly name for easy lookup
		licList := make(map[string]string)
		for _, j := range l {
			licList[j.Sha256] = j.FullName
		}

		// Display the branch history
		fmt.Printf("Branch \"%s\" history for %s:\n\n", history.Branch, file)
		for _, j := range history.Entries {
			fmt.Printf(createCommitText(j, licList))
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
func createCommitText(c commitEntry, licList map[string]string) string {
	s := fmt.Sprintf("  commit %s\n", c.ID)
	s += fmt.Sprintf("  Author: %s <%s>\n", c.AuthorName, c.AuthorEmail)
	s += fmt.Sprintf("  Date: %v\n", c.Timestamp.Format(time.UnixDate))
	if c.Tree.Entries[0].Licence != "" {
		s += fmt.Sprintf("  Licence: %s\n", licList[c.Tree.Entries[0].Licence])
	}
	if c.Message != "" {
		s += fmt.Sprintf("\n    %s\n", c.Message)
	}
	return s
}
