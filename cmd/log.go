package cmd

import (
	"errors"
	"fmt"
	"time"

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

		// If there is a local metadata cache for the requested database, use that.  Otherwise, retrieve it from the
		// server first (without storing it)
		db := args[0]
		meta, err := localFetchMetadata(db, true)
		if err != nil {
			return err
		}

		// If a branch name was given by the user, check if it exists
		if logBranch != "" {
			if _, ok := meta.Branches[logBranch]; ok == false {
				return errors.New("That branch doesn't exist for the database")
			}
		} else {
			logBranch = meta.ActiveBranch
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

		// Display the commits for the branch
		headID := meta.Branches[logBranch].Commit
		localCommit := meta.Commits[headID]
		fmt.Printf("Branch \"%s\" history for %s:\n\n", logBranch, db)
		fmt.Printf(createCommitText(meta.Commits[localCommit.ID], licList))
		for localCommit.Parent != "" {
			localCommit = meta.Commits[localCommit.Parent]
			fmt.Printf(createCommitText(meta.Commits[localCommit.ID], licList))
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
	s := fmt.Sprintf("  * Commit: %s\n", c.ID)
	s += fmt.Sprintf("    Author: %s <%s>\n", c.AuthorName, c.AuthorEmail)
	s += fmt.Sprintf("    Date: %v\n", c.Timestamp.Format(time.UnixDate))
	if c.Tree.Entries[0].LicenceSHA != "" {
		s += fmt.Sprintf("    Licence: %s\n\n", licList[c.Tree.Entries[0].LicenceSHA])
	} else {
		s += fmt.Sprintf("\n")
	}
	if c.Message != "" {
		s += fmt.Sprintf("      %s\n\n", c.Message)
	}
	return s
}
