package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	branchActiveSetBranch string
	branchActiveSetForce  *bool
)

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
		head, ok := meta.Branches[branchActiveSetBranch]
		if ok == false {
			return errors.New("That branch name doesn't exist for this database")
		}

		// Unless --force is specified, check whether the file has changed since the last commit, and let the user know
		if *branchActiveSetForce == false {
			changed, err := dbChanged(db, meta)
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("%s has been changed since the last commit.  Use --force if you really want to "+
					"overwrite it\n", db)
				return nil
			}
		}

		// Get the details of the head commit for the target branch
		commit, ok := meta.Commits[head.Commit]
		if ok == false {
			return errors.New("Something has gone wrong.  Head commit for the branch isn't in the commit list")
		}
		shaSum := commit.Tree.Entries[0].Sha256
		lastMod := commit.Tree.Entries[0].LastModified

		// Make sure the correct database from the target branch is in local cache
		err = checkDBCache(db, shaSum)
		if err != nil {
			return err
		}

		// Copy the database from local cache, so it matchesthe new branch head commit
		var b []byte
		b, err = ioutil.ReadFile(filepath.Join(".dio", db, "db", shaSum))
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(db, b, 0644)
		if err != nil {
			return err
		}
		err = os.Chtimes(db, time.Now(), lastMod)
		if err != nil {
			return err
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
	branchActiveSetForce = branchActiveSetCmd.Flags().BoolP("force", "f", false,
		"Overwrite unsaved changes to the database?")

}
