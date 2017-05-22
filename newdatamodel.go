package main // import "github.com/justinclift/newdatamodel"

import (
	"io/ioutil"
	"log"
	"os"
	"time"
)

var branches []branch
var index []commit

func main() {
	// Create some initial database tree entries
	var entry1, entry2, entry3 dbTreeEntry
	entry1.AType = DATABASE
	entry1.Name = "testdb1.sqlite"
	tempBuf, err := ioutil.ReadFile("/Users/jc/tmp/testdb1.sqlite")
	if err != nil {
		log.Printf("Something went wrong when reading in the database file: %v\n", err.Error())
		os.Exit(1)
	}

	// Store the database file
	entry1.ShaSum, err = storeDatabase(tempBuf)
	if err != nil {
		log.Printf("Something went wrong when storing the database file: %v\n", err.Error())
		os.Exit(2)
	}

	entry2.AType = DATABASE
	entry2.Name = "testdb2.sqlite"
	tempBuf, err = ioutil.ReadFile("/Users/jc/tmp/testdb2.sqlite")
	if err != nil {
		log.Printf("Something went wrong when reading in the database file: %v\n", err.Error())
		os.Exit(1)
	}

	// Store the database file
	entry2.ShaSum, err = storeDatabase(tempBuf)
	if err != nil {
		log.Printf("Something went wrong when storing the database file: %v\n", err.Error())
		os.Exit(2)
	}

	// Populate a dbTree structure with the entries
	var someTree dbTree
	someTree.Entries = append(someTree.Entries, entry1)
	someTree.Entries = append(someTree.Entries, entry2)
	someTree.ID = createDBTreeID(someTree.Entries)
	err = storeTree(someTree)
	if err != nil {
		log.Printf("Something went wrong when storing the tree file: %v\n", err.Error())
		os.Exit(5)
	}

	// Construct an initial commit structure pointing to the entry
	var someCommit commit
	someCommit.AuthorEmail = "justin@postgresql.org"
	someCommit.AuthorName = "Justin Clift"
	someCommit.Message = "Initial database upload"
	someCommit.Timestamp = time.Now()
	someCommit.Tree = someTree.ID
	someCommit.ID = createCommitID(someCommit)

	// Create another tree and commit
	entry3.AType = DATABASE
	entry3.Name = "testdb3.sqlite"
	tempBuf, err = ioutil.ReadFile("/Users/jc/tmp/testdb3.sqlite")
	if err != nil {
		log.Printf("Something went wrong when reading in the database file: %v\n", err.Error())
		os.Exit(1)
	}

	// Store the database file
	entry3.ShaSum, err = storeDatabase(tempBuf)
	if err != nil {
		log.Printf("Something went wrong when storing the database file: %v\n", err.Error())
		os.Exit(2)
	}

	var someTree2 dbTree
	someTree2.Entries = append(someTree2.Entries, entry3)
	someTree2.ID = createDBTreeID(someTree2.Entries)
	err = storeTree(someTree2)
	if err != nil {
		log.Printf("Something went wrong when storing the tree file: %v\n", err.Error())
		os.Exit(5)
	}

	var someCommit2 commit
	someCommit2.Parent = someCommit.ID
	someCommit2.AuthorEmail = "justin@postgresql.org"
	someCommit2.AuthorName = "Justin Clift"
	someCommit2.Message = "Added another database"
	someCommit2.Timestamp = time.Now()
	someCommit2.Tree = someTree2.ID
	someCommit2.ID = createCommitID(someCommit2)

	// Assemble the commits into an index
	index = append(index, someCommit)
	index = append(index, someCommit2)

	// Serialise and write out the index
	err = storeIndex(index)
	if err != nil {
		log.Printf("Something went wrong when storing the index file: %v\n", err.Error())
		os.Exit(3)
	}

	// Create a branch
	var someBranch branch
	someBranch.Name = "master"
	someBranch.Commit = someCommit2.ID

	// Create a branch pointing at the initial commit
	var someBranch2 branch
	someBranch2.Name = "first_commit"
	someBranch2.Commit = someCommit.ID

	// Populate the branches variable
	branches = append(branches, someBranch)
	branches = append(branches, someBranch2)

	// Serialise and write out the branches
	err = storeBranches(branches)
	if err != nil {
		log.Printf("Something went wrong when storing the branches file: %v\n", err.Error())
		os.Exit(4)
	}
}
