package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var branchRevertBranch, branchRevertCommit, branchRevertTag string

// Reverts a database to a prior commit in its history
var branchRevertCmd = &cobra.Command{
	Use:   "revert [database name] --branch xxx --commit yyy",
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

		// Ensure the required info was given
		if branchRevertBranch == "" {
			return errors.New("No branch name given")
		}
		if branchRevertCommit == "" && branchRevertTag == "" {
			return errors.New("Either a commit ID or tag must be given.")
		}

		// Ensure we were given only a commit ID OR a tag
		if branchRevertCommit != "" && branchRevertTag != "" {
			return errors.New("Either a commit ID or tag must be given.  Not both!")
		}

		// Load the metadata
		db := args[0]
		meta, err := loadMetadata(db)
		if err != nil {
			return err
		}

		// Make sure the branch exists
		matchFound := false
		head, ok := meta.Branches[branchRevertBranch]
		if ok == false {
			return errors.New("That branch doesn't exist")
		}
		if head.Commit == branchRevertCommit {
			matchFound = true
		}

		// Build a list of commits in the branch
		commitList := []string{head.Commit}
		c, ok := meta.Commits[head.Commit]
		if ok == false {
			return errors.New("Something has gone wrong.  Head commit for the branch isn't in the commit list")
		}
		for c.Parent != "" {
			c = meta.Commits[c.Parent]
			if c.ID == branchRevertCommit {
				matchFound = true
			}
			commitList = append(commitList, c.ID)
		}

		// Make sure the requested commit exists on the selected branch
		if !matchFound {
			return errors.New("The given commit id doesn't seem to exist on the selected branch")
		}

		// TODO: * Check if there would be isolated tags or releases if this revert is done.  If so, let the user
		//         know they'll need to remove the tags first

		// TODO: Get the tag list for the database

		// Count the number of commits in the updated branch
		var commitCount int
		listLen := len(commitList) - 1
		for i := 0; i <= listLen; i++ {
			commitCount++
			if commitList[listLen-i] == branchRevertCommit {
				break
			}
		}

		// Revert the branch
		// TODO: Remove the no-longer-referenced commits (if any) caused by this reverting
		//       * One alternative would be to leave them, and only clean up with with some kind of garbage collection
		//         operation.  Even a "dio gc" to manually trigger it
		newHead := branchEntry{
			Commit:      branchRevertCommit,
			CommitCount: commitCount,
			Description: head.Description,
		}
		meta.Branches[branchRevertBranch] = newHead

		// Save the updated metadata back to disk
		err = saveMetadata(db, meta)
		if err != nil {
			return err
		}

		fmt.Println("Branch reverted")
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchRevertCmd)
	branchRevertCmd.Flags().StringVar(&branchRevertBranch, "branch", "master", "Remote branch to operate on")
	branchRevertCmd.Flags().StringVar(&branchRevertCommit, "commit", "", "Commit ID for the to revert to")
	branchRevertCmd.Flags().StringVar(&branchRevertTag, "tag", "", "Name of tag to revert to")
}
