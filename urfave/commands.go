package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/urfave/cli"
)

// Creates a branch for a database.
func addBranch(c *cli.Context) error {
	// Ensure we've only been passed a single database to create the branch for
	l := c.Args()
	if len(l) < 1 {
		return errors.New("No database file specified")
	}
	if len(l) > 1 {
		return errors.New("'branch add' only takes a single database file argument")
	}

	// Retrieve the branches for the given database
	d := filepath.Base(l[0])
	branches, err := getBranches(d)
	if err != nil {
		return errors.New("That database isn't in the system")
	}
spew.Dump(branches)
	// TODO: Check if the named branch already exists.  Error out if so

	// TODO: Get the current commit ID from the database index
	currentCommit, err := getCurrentCommit(d)
	if err != nil {
		log.Printf("Something went wrong when retrieving the current commit ID: %v\n", err.Error())
		return err
	}
spew.Dump(currentCommit)
	// TODO: Add the named branch to the branches structure
	// Create the default branch
	//var b branch
	//b.Name = "master"
	//b.Commit = c.ID

	// TODO: Write the updates branches structure to disk

	return nil
}

// Remove a branch from a database.
func removeBranch(c *cli.Context) error {
	// TODO: Everything for this
	return nil
}

// Display the branches for a database.
func showBranches(c *cli.Context) error {
	// TODO: Everything for this
	return nil
}

// Display the commit log for a database.
func showLog(c *cli.Context) error {
	// Ensure we've only been passed a single database to generate the log for
	l := c.Args()
	if len(l) < 1 {
		return errors.New("No database file specified")
	}
	if len(l) > 1 {
		return errors.New("log only takes a single database file argument")
	}

	var activeBranch string
	dbPath := filepath.Base(l[0])
	activeBranch, err := getActiveBranch(dbPath)
	if err == nil {
		// As this is experimental code, we'll just assume an error here means there are no commits
		return errors.New("This database has no commits")
	}

	// Retrieve the commit history for the active branch
	commitList, err := getHistory(activeBranch)

	// Display the commits (in reverse order)
	for i := len(commitList) - 1; i >= 0; i-- {
		txt, err := generateCommitText(commitList[i])
		if err != nil {
			log.Printf("Something went wrong when generating commit text: %v\n", err.Error())
			return err
		}
		fmt.Printf("%s\n", txt)
	}

	return nil
}

func showTags(c *cli.Context) error {
	// TODO: Read in the existing tags

	// TODO: Display the tags
	return nil
}

// Store a database file.
func uploadDB(c *cli.Context) error {
	dbList := c.Args()

	if len(dbList) < 1 {
		return errors.New("No database files specified")
	}

	// TODO: Ensure the databases are unique (eg no double ups such as "dio up a.db b.db b.db")

	for _, j := range dbList {
		// Create dbTree entry for the database file
		var e dbTreeEntry
		e.AType = DATABASE
		e.Name = filepath.Base(j)
		buf, err := ioutil.ReadFile(j)
		if err != nil {
			log.Printf("Something went wrong when reading the database file '%s': %v\n", j,
				err.Error())
			continue
		}

		// Store the database file
		e.ShaSum, err = storeDatabase(buf)
		if err != nil {
			log.Printf("Something went wrong when storing the database file '%s': %v\n", j,
				err.Error())
			continue
		}

		// Add the entry to the dbTree structure
		var t dbTree
		t.Entries = append(t.Entries, e)
		t.ID = createDBTreeID(t.Entries)
		err = storeTree(t)
		if err != nil {
			log.Printf("Something went wrong when storing the tree file: %v\n", err.Error())
			return err
		}

		// Construct a commit structure for the entry
		var c commit
		c.AuthorEmail = "justin@postgresql.org" // TODO: Load the settings from a config file
		c.AuthorName = "Justin Clift"
		c.Timestamp = time.Now()
		c.Tree = t.ID

		// Construct the branch structures for the entry
		var b branch
		var branches []branch
		b.Commit = c.ID

		// Check for an "activebranch" file.  If it's not present, assume there are no prior commits
		d := filepath.Base(j)
		var activeBranch string
		activeBranch, err = getActiveBranch(d)
		if err == nil {
			// As this is experimental code, we'll just assume an error getting this means there are no
			// prior commits.  So, we create an initial branch called master.
			b.Name = "master"
			branches = append(branches, b)
		} else {
			// TODO: Load the previous branches, replacing the commit id of the existing maching branch
			// TODO  if it exists
			// branches = [ previous branches ]
		}

		// Add the new commit to the branches, then write the updated branch structure to disk
		b.Name = activeBranch // TODO: This will probably need to be in the above else {}

		// Write the updated branch structure to disk
		err = storeBranches(d, branches)
		if err != nil {
			log.Printf("Something went wrong when storing the branches file: %v\n", err.Error())
			return err
		}

		// Create the "HEAD" file to track which branch the client is on
		err = storeActiveBranch(e.Name, "master")
		if err != nil {
			log.Printf("Something went wrong when creating the HEAD file: %v\n", err.Error())
			return err
		}

		fmt.Printf("%s stored, %d bytes, commit ID %s\n", d, len(buf), c.ID)
	}

	return nil
}
