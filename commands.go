package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli"
)

func showLog(c *cli.Context) error {
	// Ensure we've only been passed a single database to generate the log for
	dbList := c.Args()
	if len(dbList) < 1 {
		return errors.New("No database file specified")
	}
	if len(dbList) > 1 {
		return errors.New("log only takes a single database file argument")
	}

	// Parse the index for the given database
	dbPath := filepath.Base(dbList[0])
	buf, err := ioutil.ReadFile(STORAGEDIR + string(os.PathSeparator) + "meta" + string(os.PathSeparator) +
		dbPath + string(os.PathSeparator) + "index")
	if err != nil {
		log.Printf("Something went wrong when reading the index file: %v\n", err.Error())
		return err
	}
	var index []commit
	err = json.Unmarshal(buf, &index)
	if err != nil {
		log.Printf("Something went wrong when unserialising the index data: %v\n", err.Error())
		return err
	}

	// Display the commits
	for _, j := range index {
		txt, err := generateCommitText(j, true)
		if err != nil {
			log.Printf("Something went wrong when generating commit text: %v\n", err.Error())
			return err
		}
		fmt.Printf("%s\n", txt)
	}

	return nil
}

// Store a database file.
func uploadDB(c *cli.Context) error {
	dbList := c.Args()

	if len(dbList) < 1 {
		return errors.New("No database files specified")
	}

	for _, j := range dbList {
		// Create dbTree entry for the database file
		var e dbTreeEntry
		e.AType = DATABASE
		e.Name = filepath.Base(j)
		buf, err := ioutil.ReadFile(j)
		if err != nil {
			log.Printf("Something went wrong when reading in the database file: %v\n", err.Error())
			return err
		}

		// Store the database file
		e.ShaSum, err = storeDatabase(buf)
		if err != nil {
			log.Printf("Something went wrong when storing the database file: %v\n", err.Error())
			return err
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

		// Construct an initial commit structure pointing to the entry
		var c commit
		c.AuthorEmail = "justin@postgresql.org"
		c.AuthorName = "Justin Clift"
		c.Message = "Initial database upload"
		c.Timestamp = time.Now()
		c.Tree = t.ID
		c.ID = createCommitID(c)
		err = storeCommit(c)
		if err != nil {
			log.Printf("Something went wrong when storing the commit file: %v\n", err.Error())
			return err
		}

		// Assemble the commits into an index
		var i []commit
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
