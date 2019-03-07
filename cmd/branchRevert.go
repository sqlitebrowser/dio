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
		delList := map[string]struct{}{}
		if !matchFound {
			delList[head.Commit] = struct{}{} // Start creating a list of the branch commits to be deleted
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
			if !matchFound {
				delList[c.ID] = struct{}{} // Only commits prior to matchFound should be deleted
			}
			commitList = append(commitList, c.ID)
		}

		// Make sure the requested commit exists on the selected branch
		if !matchFound {
			return errors.New("The given commit or tag doesn't seem to exist on the selected branch")
		}

		// Make sure the correct database from the target branch is in local cache
		var shaSum string
		var lastMod time.Time
		if branchRevertCommit != "" {
			shaSum = meta.Commits[branchRevertCommit].Tree.Entries[0].Sha256
			lastMod = meta.Commits[branchRevertCommit].Tree.Entries[0].LastModified

			// Fetch the database from DBHub.io if it's not in the local cache
			err = checkDBCache(db, shaSum)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("Haven't been able to determine branch name.  This shouldn't happen")
		}

		// Check if deleting the commits would leave isolated tags or releases.  If so, abort and warn the user
		type isolCheck struct {
			safe   bool
			commit string
		}
		var isolatedTags []string
		var isolatedReleases []string
		commitTags := map[string]isolCheck{}
		commitReleases := map[string]isolCheck{}
		for delCommit := range delList {
			// Ensure that deleting this commit won't result in any isolated/unreachable tags
			for tName, tEntry := range meta.Tags {
				// Scan through the database tag list, checking if any of the tags is for the commit we're deleting
				if tEntry.Commit == delCommit {
					commitTags[tName] = isolCheck{safe: false, commit: delCommit}
				}
			}

			// Ensure that deleting this commit won't result in any isolated/unreachable releases
			for rName, rEntry := range meta.Releases {
				// Scan through the database release list, checking if any of the releases is for the commit we're
				// deleting
				if rEntry.Commit == delCommit {
					commitReleases[rName] = isolCheck{safe: false, commit: delCommit}
				}
			}
		}

		if len(commitTags) > 0 {
			// If a commit we're deleting has a tag on it, we need to check whether the commit is on other branches too
			//   * If it is, we're ok to proceed as the tag can still be reached from the other branch(es)
			//   * If it isn't, we need to abort this deletion (and tell the user), as the tag would become unreachable
			for bName, bEntry := range meta.Branches {
				if bName == branchRevertBranch {
					// We only run this comparison from "other branches", not the branch whose history we're changing
					continue
				}
				c, ok = meta.Commits[bEntry.Commit]
				if !ok {
					return fmt.Errorf("Broken commit history encountered when checking for isolated tags "+
						"while reverting in branch '%s' of database '%s'\n", branchRevertBranch, db)
				}
				for tName, tEntry := range commitTags {
					if c.ID == tEntry.commit {
						// The commit is also on another branch, so we're ok to delete the commit
						tmp := commitTags[tName]
						tmp.safe = true
						commitTags[tName] = tmp
					}
				}
				for c.Parent != "" {
					c, ok = meta.Commits[c.Parent]
					if !ok {
						return fmt.Errorf("Broken commit history encountered when checking for isolated tags "+
							"while reverting in branch '%s' of database '%s'\n", branchRevertBranch, db)
					}
					for tName, tEntry := range commitTags {
						if c.ID == tEntry.commit {
							// The commit is also on another branch, so we're ok to delete the commit
							tmp := commitTags[tName]
							tmp.safe = true
							commitTags[tName] = tmp
						}
					}
				}
			}

			// Create a list of would-be-isolated tags
			for tName, tEntry := range commitTags {
				if tEntry.safe == false {
					isolatedTags = append(isolatedTags, tName)
				}
			}
		}

		if len(commitReleases) > 0 {
			// If a commit we're deleting has a release on it, we need to check whether the commit is on other branches too
			//   * If it is, we're ok to proceed as the release can still be reached from the other branch(es)
			//   * If it isn't, we need to abort this deletion (and tell the user), as the release would become unreachable
			for bName, bEntry := range meta.Branches {
				if bName == branchRevertBranch {
					// We only run this comparison from "other branches", not the branch whose history we're changing
					continue
				}
				c, ok = meta.Commits[bEntry.Commit]
				if !ok {
					return fmt.Errorf("Broken commit history encountered when checking for isolated releases "+
						"while reverting in branch '%s' of database '%s'\n", branchRevertBranch, db)
				}
				for rName, rEntry := range commitReleases {
					if c.ID == rEntry.commit {
						// The commit is also on another branch, so we're ok to delete the commit
						tmp := commitReleases[rName]
						tmp.safe = true
						commitReleases[rName] = tmp
					}
				}
				for c.Parent != "" {
					c, ok = meta.Commits[c.Parent]
					if !ok {
						return fmt.Errorf("Broken commit history encountered when checking for isolated "+
							"releases while reverting in branch '%s' of database '%s'\n", branchRevertBranch, db)
					}
					for rName, rEntry := range commitReleases {
						if c.ID == rEntry.commit {
							// The commit is also on another branch, so we're ok to delete the commit
							tmp := commitReleases[rName]
							tmp.safe = true
							commitReleases[rName] = tmp
						}
					}
				}
			}

			// Create a list of would-be-isolated releases
			for rName, rEntry := range commitReleases {
				if rEntry.safe == false {
					isolatedReleases = append(isolatedReleases, rName)
				}
			}
		}

		// If any tags or releases would be isolated, abort
		if len(isolatedTags) > 0 || len(isolatedReleases) > 0 {
			e := fmt.Sprint("You need to remove the following tags and releases before reverting to this " +
				"commit:\n\n")
			for _, j := range isolatedTags {
				e = fmt.Sprintf("%s  * tag '%s'\n", e, j)
			}
			for _, j := range isolatedReleases {
				e = fmt.Sprintf("%s  * release '%s'\n", e, j)
			}
			return errors.New(e)
		}

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
