package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var branchActiveSetBranch string

// Sets the active branch for a database
var branchActiveSetCmd = &cobra.Command{
	Use:   "set [database name] --branch xxx",
	Short: "Set the active branch for a database",
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
		if branchActiveSetBranch == "" {
			return errors.New("No branch name given")
		}

		// If there's no local metadata cache, then create one
		db := args[0]
		meta, err := loadMetadata(db)
		if err != nil {
			return err
		}

		// Make sure the given branch name exists
		if _, ok := meta.Branches[branchActiveSetBranch]; ok == false {
			return errors.New("That branch name doesn't exist for this database")
		}

		// Set the active branch
		meta.ActiveBranch = branchActiveSetBranch

		// Save the updated metadata
		err = saveMetadata(db, meta)
		if err != nil {
			return err
		}

		fmt.Printf("Branch '%s' set as active for '%s'\n", branchActiveSetBranch, db)
		return nil
	},
}

func init() {
	branchActiveCmd.AddCommand(branchActiveSetCmd)
	branchActiveSetCmd.Flags().StringVar(&branchActiveSetBranch, "branch", "",
		"Remote branch to set as active")
}
