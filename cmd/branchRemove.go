package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// Removes a branch from a database
var branchRemoveCmd = &cobra.Command{
	Use:   "remove [database name] --branch xxx",
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

		// Load the metadata
		db := args[0]
		meta, err := loadMetadata(db)
		if err != nil {
			return err
		}

		// Check if the branch exists
		if _, ok := meta.Branches[branch]; ok != true {
			return errors.New("A branch with that name doesn't exist")
		}

		// If the branch is the currently active one, then abort
		if branch == meta.ActiveBranch {
			return errors.New("Can't remove the currently active branch.  You need to switch branches first")
		}

		// Remove the branch
		delete(meta.Branches, branch)

		// Save the updated metadata back to disk
		err = saveMetadata(db, meta)
		if err != nil {
			return err
		}

		fmt.Printf("Branch '%s' removed\n", branch)
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchRemoveCmd)
	branchRemoveCmd.Flags().StringVar(&branch, "branch", "", "Name of remote branch to remove")
}
