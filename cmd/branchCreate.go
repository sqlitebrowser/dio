package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var branchCreateBranch, branchCreateCommit, branchCreateMsg string

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
		if branchCreateBranch == "" {
			return errors.New("No branch name given")
		}
		if branchCreateCommit == "" {
			return errors.New("No commit ID given")
		}

		// Load the metadata
		db := args[0]
		meta, err := loadMetadata(db)
		if err != nil {
			return err
		}

		// Ensure a branch with the same name doesn't already exist
		if _, ok := meta.Branches[branchCreateBranch]; ok == true {
			return errors.New("A branch with that name already exists")
		}

		// Make sure the target commit exists in our commit list
		c, ok := meta.Commits[branchCreateCommit]
		if ok != true {
			return errors.New("That commit isn't in the database commit list")
		}

		// Count the number of commits in the new branch
		numCommits := 1
		for c.Parent != "" {
			numCommits++
			c = meta.Commits[c.Parent]
		}

		// Generate the new branch info locally
		newBranch := branchEntry{
			Commit:      branchCreateCommit,
			CommitCount: numCommits,
			Description: branchCreateMsg,
		}

		// Add the new branch to the local metadata cache
		meta.Branches[branchCreateBranch] = newBranch

		// Save the updated metadata back to disk
		err = saveMetadata(db, meta)
		if err != nil {
			return err
		}

		fmt.Printf("Branch '%s' created\n", branchCreateBranch)
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchCreateCmd)
	branchCreateCmd.Flags().StringVar(&branchCreateBranch, "branch", "", "Name of remote branch to create")
	branchCreateCmd.Flags().StringVar(&branchCreateCommit, "commit", "", "Commit ID for the new branch head")
	branchCreateCmd.Flags().StringVar(&branchCreateMsg, "description", "", "Description of the branch")
}
