package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var branchUpdateBranch, branchUpdateMsg string
var descDel *bool

// Updates the description text for a branch
var branchUpdateCmd = &cobra.Command{
	Use:   "update [database name]",
	Short: "Update the description for a branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		return branchUpdate(args)
	},
}

func init() {
	branchCmd.AddCommand(branchUpdateCmd)
	branchUpdateCmd.Flags().StringVar(&branchUpdateBranch, "branch", "",
		"Name of branch to update")
	descDel = branchUpdateCmd.Flags().BoolP("delete", "d", false,
		"Delete the branch description")
	branchUpdateCmd.Flags().StringVar(&branchUpdateMsg, "description", "",
		"New description for the branch")
}

func branchUpdate(args []string) error {
	// Ensure a database file was given
	var db string
	var err error
	var meta metaData
	if len(args) == 0 {
		db, err = getDefaultDatabase()
		if err != nil {
			return err
		}
		if db == "" {
			// No database name was given on the command line, and we don't have a default database selected
			return errors.New("No database file specified")
		}
	} else {
		db = args[0]
	}
	if len(args) > 1 {
		return errors.New("Only one database can be changed at a time (for now)")
	}

	// Ensure a branch name and description text were given
	if branchUpdateBranch == "" {
		return errors.New("No branch name given")
	}
	if branchUpdateMsg == "" && *descDel == false {
		return errors.New("No description text given")
	}

	// Load the metadata
	meta, err = loadMetadata(db)
	if err != nil {
		return err
	}

	// Make sure the branch exists
	branch, ok := meta.Branches[branchUpdateBranch]
	if ok == false {
		return errors.New("That branch doesn't exist")
	}

	// Update the branch
	if *descDel == false {
		branch.Description = branchUpdateMsg
	} else {
		branch.Description = ""
	}
	meta.Branches[branchUpdateBranch] = branch

	// Save the updated metadata back to disk
	err = saveMetadata(db, meta)
	if err != nil {
		return err
	}

	// Inform the user
	_, err = fmt.Fprintln(fOut, "Branch updated")
	return err
}
