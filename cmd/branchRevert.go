package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	branchRevertBranch, branchRevertCommit, branchRevertTag string
	branchRevertForce                                       *bool
)

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

		// Unless --force is specified, check whether the file has changed since the last commit, and let the user know
		if *branchRevertForce == false {
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

		// If a tag was given, make sure it exists
		if branchRevertTag != "" {
			tagData, ok := meta.Tags[branchRevertTag]
			if !ok {
				return errors.New("That tag doesn't exist")
			}

			// Use the commit associated with the tag
			branchRevertCommit = tagData.Commit
		}

		// If no branch name was passed, use the active branch
		if branchRevertBranch == "" {
			branchRevertBranch = meta.ActiveBranch
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
			return errors.New("The given commit or tag doesn't seem to exist on the selected branch")
		}

		// Abort if the database for the requested commit isn't in the local cache
		var shaSum string
		var lastMod time.Time
		if branchRevertCommit != "" {
			shaSum = meta.Commits[branchRevertCommit].Tree.Entries[0].Sha256
			lastMod = meta.Commits[branchRevertCommit].Tree.Entries[0].LastModified
			// Fetch the database from DBHub.io if it's not in the local cache
			if _, err = os.Stat(filepath.Join(".dio", db, "db", shaSum)); os.IsNotExist(err) {
				_, body, err := retrieveDatabase(db, pullCmdBranch, pullCmdCommit)
				if err != nil {
					return err
				}

				// Verify the SHA256 checksum of the new download
				s := sha256.Sum256([]byte(body))
				thisSum := hex.EncodeToString(s[:])
				if thisSum != shaSum {
					// The newly downloaded database file doesn't have the expected checksum.  Abort.
					return errors.New(fmt.Sprintf("Aborting: newly downloaded database file should have "+
						"checksum '%s', but data with checksum '%s' received\n", shaSum, thisSum))
				}

				// Write the database file to disk in the cache directory
				err = ioutil.WriteFile(filepath.Join(".dio", db, "db", shaSum), []byte(body), 0644)
				if err != nil {
					return err
				}
			}
		}

		// TODO: * Check if there would be isolated tags or releases if this revert is done.  If so, let the user
		//         know they'll need to remove the tags first

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

		// Copy the file from local cache to the working directory
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
	branchRevertCmd.Flags().StringVar(&branchRevertBranch, "branch", "",
		"Branch to operate on")
	branchRevertCmd.Flags().StringVar(&branchRevertCommit, "commit", "",
		"Commit ID for the to revert to")
	branchRevertForce = branchRevertCmd.Flags().BoolP("force", "f", false,
		"Overwrite unsaved changes to the database?")
	branchRevertCmd.Flags().StringVar(&branchRevertTag, "tag", "", "Name of tag to revert to")
}
