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
		// Ensure a database file was given
		if len(args) == 0 {
			return errors.New("No database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
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
		db := args[0]
		meta, err := loadMetadata(db)
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
		fmt.Println("Branch updated")
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchUpdateCmd)
	branchUpdateCmd.Flags().StringVar(&branchUpdateBranch, "branch", "",
		"Name of remote branch to create")
	descDel = branchUpdateCmd.Flags().BoolP("delete", "d", false,
		"Delete the branch description")
	branchUpdateCmd.Flags().StringVar(&branchUpdateMsg, "description", "", "Description of the branch")
}
