package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/urfave/cli"
)

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

	// Parse the index for the given database
	p := filepath.Base(l[0])
	idx, err := getIndex(p)
	if err != nil {
		return errors.New("That database isn't in the system")
	}

	// Display the commits (in reverse order)
	for i := len(idx) - 1; i >= 0; i-- {
		txt, err := generateCommitText(idx[i])
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

		// If the database is already in our system, load its existing index
		i, err := getIndex(e.Name)
		if err != nil {
			// As this is just experimental code, we'll assume an error returned here means the database
			// isn't already in the system
			// TODO: Proper error handling
			c.Message = "Initial database upload" // TODO: Add a flag to passing the commit message
		} else {
			// No error, which should mean the database already exists
			c.Message = "foo" // TODO: Add a flag to passing the commit message
		}

		// Store the commit and add its id to the index
		c.ID = createCommitID(c)
		err = storeCommit(c)
		if err != nil {
			log.Printf("Something went wrong when storing the commit file: %v\n", err.Error())
			return err
		}
		i = append(i, c)

		// Serialise and write out the index
		n := filepath.Base(j)
		err = storeIndex(n, i)
		if err != nil {
			log.Printf("Something went wrong when storing the index file: %v\n", err.Error())
			return err
		}

		// Create the default branch
		var b branch
		b.Name = "master"
		b.Commit = c.ID

		// Populate the branches variable
		var branches []branch
		branches = append(branches, b)

		// Serialise and write out the branches
		err = storeBranches(n, branches)
		if err != nil {
			log.Printf("Something went wrong when storing the branches file: %v\n", err.Error())
			return err
		}

		fmt.Printf("%s stored, %d bytes, commit ID %s\n", n, len(buf), c.ID)
	}

	return nil
}
