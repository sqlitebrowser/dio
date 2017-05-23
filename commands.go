package main

import (
	"errors"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/urfave/cli"
)

func uploadDB(c *cli.Context) error {
	dbList := c.Args()
log.Printf("%v arguments: %v\n", len(dbList), dbList)

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
		err = storeIndex(filepath.Base(j), i)
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
		err = storeBranches(filepath.Base(j), branches)
		if err != nil {
			log.Printf("Something went wrong when storing the branches file: %v\n", err.Error())
			return err
		}

spew.Dump(c)
log.Printf("Length of database file '%s': %v\n", j, len(buf))
	}

	return nil
}
