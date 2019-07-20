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
	commitCmdAuthEmail, commitCmdAuthName, commitCmdBranch, commitCmdCommit string
	commitCmdLicence, commitCmdMsg, commitCmdTimestamp                      string
)

// Create a commit for the database on the currently active branch
var (
	commitCmd = &cobra.Command{
		Use:   "commit [database file]",
		Short: "Creates a new commit for the database",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commit(args)
		},
	}
)

func init() {
	RootCmd.AddCommand(commitCmd)
	commitCmd.Flags().StringVar(&commitCmdBranch, "branch", "",
		"The branch this commit will be appended to")
	commitCmd.Flags().StringVar(&commitCmdCommit, "commit", "",
		"ID of the previous commit, for appending this new database to")
	commitCmd.Flags().StringVar(&commitCmdAuthEmail, "email", "",
		"Email address of the commit author")
	commitCmd.Flags().StringVar(&commitCmdLicence, "licence", "",
		"The licence (ID) for the database, as per 'dio licence list'")
	commitCmd.Flags().StringVar(&commitCmdMsg, "message", "",
		"Description / commit message")
	commitCmd.Flags().StringVar(&commitCmdAuthName, "name", "", "Name of the commit author")
	commitCmd.Flags().StringVar(&commitCmdTimestamp, "timestamp", "", "Timestamp for the commit")
}

func commit(args []string) error {
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
	// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
	// TODO  into a for loop
	if len(args) > 1 {
		return errors.New("Only one database can be uploaded at a time (for now)")
	}

	// Ensure the database file exists
	fi, err := os.Stat(db)
	if err != nil {
		return err
	}

	// Grab author name & email from the dio config file, but allow command line flags to override them
	var authorName, authorEmail, committerName, committerEmail string
	if z, ok := viper.Get("user.name").(string); ok {
		authorName = z
		committerName = z
	}
	if z, ok := viper.Get("user.email").(string); ok {
		authorEmail = z
		committerEmail = z
	}
	if commitCmdAuthName != "" {
		authorName = commitCmdAuthName
	}
	if commitCmdAuthEmail != "" {
		authorEmail = commitCmdAuthEmail
	}

	// Author name and email are required
	if authorName == "" || authorEmail == "" || committerName == "" || committerEmail == "" {
		return errors.New("Author and committer name and email addresses are required!")
	}

	// If a timestamp was provided, make sure it parses ok
	commitTime := time.Now()
	if commitCmdTimestamp != "" {
		commitTime, err = time.Parse(time.RFC3339, commitCmdTimestamp)
		if err != nil {
			return err
		}
	}

	// If the database metadata doesn't exist locally, check if it does exist on the server.
	var newDB, localPresent bool
	if _, err = os.Stat(filepath.Join(".dio", db, "db")); os.IsNotExist(err) {
		// At the moment, since there's no better way to check for the existence of a remote database, we just
		// grab the list of the users databases and check against that
		dbList, errInner := getDatabases(cloud, certUser)
		if errInner != nil {
			return errInner
		}
		for _, j := range dbList {
			if db == j.Name {
				// This database already exists on DBHub.io.  We need local metadata in order to proceed, but don't
				// yet have it.  Safest option, at least for now, is to tell the user and abort
				return errors.New("Aborting: the database exists on the remote server, but has no " +
					"local metadata cache.  Please retrieve the remote metadata, then run the commit command again")
			}
		}

		// This is a new database, so we generate new metadata
		newDB = true
		meta = newMetaStruct(commitCmdBranch)
	} else {
		// We have local metaData
		localPresent = true
	}

	// Load the metadata
	if !newDB {
		meta, err = loadMetadata(db)
		if err != nil {
			return err
		}
	}

	// If no branch name was passed, use the active branch
	if commitCmdBranch == "" {
		commitCmdBranch = meta.ActiveBranch
	}

	// Check if the database is unchanged from the previous commit, and if so we abort the commit
	if localPresent {
		changed, err := dbChanged(db, meta)
		if err != nil {
			return err
		}
		if !changed && commitCmdLicence == ""{
			return fmt.Errorf("Database is unchanged from last commit.  No need to commit anything.")
		}
	}

	// Get the current head commit for the selected branch, as that will be the parent commit for this new one
	head, ok := meta.Branches[commitCmdBranch]
	if !ok {
		return errors.New(fmt.Sprintf("That branch ('%s') doesn't exist", commitCmdBranch))
	}
	var existingLicSHA string
	if newDB {
		if commitCmdLicence == "" {
			// If this is a new database, and no licence was given on the command line, then default to
			// 'Not specified'
			commitCmdLicence = "Not specified"
		}
	} else {
		if localPresent {
			// We can only use commit data if local metadata is present
			headCommit, ok := meta.Commits[head.Commit]
			if !ok {
				return errors.New("Aborting: info for the head commit isn't found in the local commit cache")
			}
			existingLicSHA = headCommit.Tree.Entries[0].LicenceSHA
		}
	}

	// Retrieve the list of known licences
	licList, err := getLicences()
	if err != nil {
		return err
	}

	// Determine the SHA256 of the requested licence
	var licID, licSHA string
	if commitCmdLicence != "" {
		// Scan the licence list for a matching licence name
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
		// If no licence was given, use the licence from the previous commit
		licSHA = existingLicSHA
	}

	// Generate an appropriate commit message if none was provided
	if commitCmdMsg == "" {
		if !newDB && existingLicSHA != licSHA {
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

		// If it's a new database and there's still no commit message, generate a reasonable one
		if newDB && commitCmdMsg == "" {
			commitCmdMsg = "New database created"
		}
	}

	// * Collect info for the new commit *

	// Get file size and last modified time for the database
	fileSize := fi.Size()
	lastModified := fi.ModTime()

	// Verify we've read the file from disk ok
	b, err := ioutil.ReadFile(db)
	if err != nil {
		return err
	}
	if int64(len(b)) != fileSize {
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
	e.LastModified = lastModified.UTC()
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
		AuthorName:     authorName,
		AuthorEmail:    authorEmail,
		CommitterName:  committerName,
		CommitterEmail: committerEmail,
		Message:        commitCmdMsg,
		Parent:         head.Commit,
		Timestamp:      commitTime.UTC(),
		Tree:           t,
	}

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
		if _, err = os.Stat(filepath.Join(".dio", db)); os.IsNotExist(err) {
			err = os.MkdirAll(filepath.Join(".dio", db, "db"), 0770)
			if err != nil {
				return err
			}
		}
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

	// Display results to the user
	_, err = fmt.Fprintf(fOut, "Commit created on '%s'\n", db)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(fOut, "  * Commit ID: %s\n", newCom.ID)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(fOut, "    Branch: %s\n", commitCmdBranch)
	if err != nil {
		return err
	}
	if licID != "" {
		_, err = fmt.Fprintf(fOut, "    Licence: %s\n", licID)
		if err != nil {
			return err
		}
	}
	_, err = numFormat.Fprintf(fOut, "    Size: %d bytes\n", e.Size)
	if err != nil {
		return err
	}
	if commitCmdMsg != "" {
		_, err = fmt.Fprintf(fOut, "    Commit message: %s\n\n", commitCmdMsg)
		if err != nil {
			return err
		}
	}
	return nil
}

// Creates a new metadata structure in memory
func newMetaStruct(branch string) (meta metaData) {
	b := branchEntry{
		Commit:      "",
		CommitCount: 0,
		Description: "",
	}
	var initialBranch string
	if branch == "" {
		initialBranch = "master"
	} else {
		initialBranch = branch
	}
	meta = metaData{
		ActiveBranch: initialBranch,
		Branches:     map[string]branchEntry{initialBranch: b},
		Commits:      map[string]commitEntry{},
		DefBranch:    initialBranch,
		Releases:     map[string]releaseEntry{},
		Tags:         map[string]tagEntry{},
	}
	return
}
