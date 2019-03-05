package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	commitCmdBranch, commitCmdCommit, commitCmdEmail string
	commitCmdLicence, commitCmdMsg, commitCmdName    string
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

		// TODO: Add support for committing when the database doesn't yet exist locally, nor remotely
		// TODO: Remember to create a reasonable commit message for a new database, if none is provided

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
		headCommit, ok := meta.Commits[head.Commit]
		if !ok {
			return errors.New("Aborting: info for the head commit isn't found in the local commit cache")
		}
		existingLicSHA := headCommit.Tree.Entries[0].LicenceSHA

		// Retrieve the list of known licences
		licList, err := getLicences()
		if err != nil {
			return err
		}

		// If no licence was given, use the licence from the head commit
		var licID, licSHA string
		if commitCmdLicence != "" {
			// Select the requested licence (SHA256) from the list
			matchFound := false
			lwrLic := strings.ToLower(commitCmdLicence)
			for i, j := range licList {
				if strings.ToLower(i) == lwrLic {
					licID = i
					licSHA = j.Sha256
					matchFound = true
					break
				}
			}
			if !matchFound {
				return errors.New("Aborting: could not determine the name of the existing database licence")
			}
		} else {
			licSHA = existingLicSHA
		}

		// Generate an appropriate commit message if none was provided
		if commitCmdMsg == "" {
			if existingLicSHA != licSHA {
				// * The licence has changed, so we create a reasonable commit message indicating this *

				// Work out the human friendly short licence name for the current database
				matchFound := false
				var existingLicID string
				for i, j := range licList {
					if existingLicSHA == j.Sha256 {
						existingLicID = i
						matchFound = true
						break
					}
				}
				if !matchFound {
					return errors.New("Aborting: could not locate the requested database licence")
				}
				commitCmdMsg = fmt.Sprintf("Database licence changed from '%s' to '%s'.", existingLicID, licID)
			}
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
		e.LicenceSHA = licSHA
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

		fmt.Printf("Commit created on '%s'\n", db)
		fmt.Printf("  * Commit ID: %s\n", newCom.ID)
		fmt.Printf("    Branch: %s\n", commitCmdBranch)
		fmt.Printf("    Licence: %s\n", licID)
		fmt.Printf("    Size: %d bytes\n", e.Size)
		if commitCmdMsg != "" {
			fmt.Printf("    Commit message: %s\n\n", commitCmdMsg)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(commitCmd)
	commitCmd.Flags().StringVar(&commitCmdBranch, "branch", "",
		"The branch this commit will be appended to")
	commitCmd.Flags().StringVar(&commitCmdCommit, "commit", "",
		"ID of the previous commit, for appending this new database to")
	commitCmd.Flags().StringVar(&commitCmdEmail, "email", "",
		"Email address of the commit author")
	commitCmd.Flags().StringVar(&commitCmdLicence, "licence", "",
		"The licence (ID) for the database, as per 'dio licence list'")
	commitCmd.Flags().StringVar(&commitCmdMsg, "message", "",
		"Description / commit message")
	commitCmd.Flags().StringVar(&commitCmdName, "name", "", "Name of the commit author")
}
