package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	commitCmdBranch, commitCmdCommit, commitCmdEmail string
	commitCmdLicence, commitCmdMsg, commitCmdName    string
	commitCmdForce, commitCmdPublic                  bool
)

// Create a commit for the database on the currently active branch
var commitCmd = &cobra.Command{
	Use:   "commit [database file]",
	Short: "Creates a new commit for the database",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a database file was given
		if len(args) == 0 {
			return errors.New("No database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one database can be uploaded at a time (for now)")
		}

		// Ensure the database file exists
		db := args[0]
		fi, err := os.Stat(db)
		if err != nil {
			return err
		}

		// Grab author name & email from the dio config file, but allow command line flags to override them
		var commitAuthor, commitEmail string
		u, ok := viper.Get("user.name").(string)
		if ok {
			commitAuthor = u
		}
		v, ok := viper.Get("user.email").(string)
		if ok {
			commitEmail = v
		}
		if commitCmdName != "" {
			commitAuthor = commitCmdName
		}
		if commitCmdEmail != "" {
			commitEmail = commitCmdEmail
		}

		// Author name and email are required
		if commitAuthor == "" || commitEmail == "" {
			return errors.New("Both author name and email are required!")
		}

		// Ensure commit message has been provided
		if commitCmdMsg == "" {
			return errors.New("Commit message is required!")
		}

		// TODO: Add support for committing when the database doesn't yet exist locally, nor remotely

		// Load the metadata
		meta, err := loadMetadata(db)
		if err != nil {
			return err
		}

		// If no branch name was passed, use the active branch
		if commitCmdBranch == "" {
			commitCmdBranch = meta.ActiveBranch
		}

		// Get the current head commit for the selected branch, as that will be the parent commit for this new one
		head, ok := meta.Branches[commitCmdBranch]
		if !ok {
			return errors.New(fmt.Sprintf("That branch ('%s') doesn't exist", commitCmdBranch))
		}

		// If no licence was given, use the licence from the head commit
		// TODO: Add support for the licence option
		if commitCmdLicence == "" {
			c, ok := meta.Commits[head.Commit]
			if !ok {
				return errors.New("Aborting: info for the head commit isn't found in the local commit cache")
			}
			commitCmdLicence = c.Tree.Entries[0].LicenceSHA
		}

		// * Collect info for the new commit *

		// Get file size and last modified time for the database
		fileSize := int(fi.Size())
		lastModified := fi.ModTime()

		// Verify we've read the file from disk ok
		b, err := ioutil.ReadFile(db)
		if err != nil {
			return err
		}
		if len(b) != fileSize {
			return errors.New(numFormat.Sprintf("Aborting: # of bytes read (%d) when generating commit don't "+
				"match database file size (%d)", len(b), fileSize))
		}

		// Generate sha256
		s := sha256.Sum256(b)
		shaSum := hex.EncodeToString(s[:])

		// * Generate the new commit *

		// Create a new dbTree entry for the database file
		var e dbTreeEntry
		e.EntryType = DATABASE
		e.LastModified = lastModified
		e.LicenceSHA = commitCmdLicence
		e.Name = db
		e.Sha256 = shaSum
		e.Size = fileSize

		// Create a new dbTree structure for the new database entry
		var t dbTree
		t.Entries = append(t.Entries, e)
		t.ID = createDBTreeID(t.Entries)

		// Create a new commit for the new tree
		newCom := commitEntry{
			CommitterName:  commitAuthor,
			CommitterEmail: commitEmail,
			Message:        commitCmdMsg,
			Parent:         head.Commit,
			Timestamp:      time.Now(),
			Tree:           t,
		}
		newCom.AuthorName = commitAuthor
		newCom.AuthorEmail = commitEmail

		// Calculate the new commit ID, which incorporates the updated tree ID (and thus the new licence sha256)
		newCom.ID = createCommitID(newCom)

		// Add the new commit info to the database commit list
		meta.Commits[newCom.ID] = newCom

		// Update the branch head info to point at the new commit
		meta.Branches[commitCmdBranch] = branchEntry{
			Commit:      newCom.ID,
			CommitCount: head.CommitCount + 1,
			Description: head.Description,
		}

		// If the database file isn't already in the local cache, then copy it there
		if _, err = os.Stat(filepath.Join(".dio", db, "db", shaSum)); os.IsNotExist(err) {
			err = ioutil.WriteFile(filepath.Join(".dio", db, "db", shaSum), b, 0644)
			if err != nil {
				return err
			}
		}

		// Save the updated metadata back to disk
		err = saveMetadata(db, meta)
		if err != nil {
			return err
		}

		fmt.Printf("Database commit '%s' created\n", newCom.ID)
		fmt.Printf("  * Name: %s\n", db)
		fmt.Printf("    Branch: %s\n", commitCmdBranch)
		if commitCmdLicence != "" {
			fmt.Printf("    Licence: %s\n", commitCmdLicence)
		}
		fmt.Printf("    Size: %d bytes\n", fi.Size())
		fmt.Printf("    Commit message: %s\n\n", commitCmdMsg)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(commitCmd)
	commitCmd.Flags().StringVar(&commitCmdBranch, "branch", "",
		"Remote branch the database will be uploaded to")
	commitCmd.Flags().StringVar(&commitCmdCommit, "commit", "",
		"ID of the previous commit, for appending this new database to")
	commitCmd.Flags().StringVar(&commitCmdEmail, "email", "", "Email address of the commit author")
	commitCmd.Flags().BoolVar(&commitCmdForce, "force", false, "Overwrite existing commit history?")
	commitCmd.Flags().StringVar(&commitCmdLicence, "licence", "",
		"The licence (ID) for the database, as per 'dio licence list'")
	commitCmd.Flags().StringVar(&commitCmdMsg, "message", "",
		"(Required) Commit message for this upload")
	commitCmd.Flags().StringVar(&commitCmdName, "name", "", "Name of the commit author")
	commitCmd.Flags().BoolVar(&commitCmdPublic, "public", false, "Should the database be public?")
}
