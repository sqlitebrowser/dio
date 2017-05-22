package main // import "github.com/justinclift/newdatamodel"

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"log"
	"os"
	"time"

//	"github.com/davecgh/go-spew/spew"
)

type branch struct {
	commit [32]byte
	name   string
}

type commit struct {
	authorEmail    string
	authorName     string
	committerEmail string
	committerName  string
	id             [32]byte
	message        string
	parent         [32]byte
	timestamp      time.Time
	tree           [32]byte
}

type DBTreeEntryType string

const (
	TREE     DBTreeEntryType = "tree"
	DATABASE                 = "db"
	LICENCE                  = "licence"
)

type dbTree struct {
	id      [32]byte
	entries []dbTreeEntry
}
type dbTreeEntry struct {
	aType   DBTreeEntryType
	licence [32]byte
	shaSum  [32]byte
	name    string
}

var NILSHA256 = [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

var branches []branch
var index []commit

func main() {
	// Create some initial database tree entries
	var entry1, entry2, entry3 dbTreeEntry
	entry1.aType = DATABASE
	entry1.name = "testdb1.sqlite"
	tempBuf, err := ioutil.ReadFile("/Users/jc/tmp/testdb1.sqlite")
	if err != nil {
		log.Printf("Something went wrong when reading in the database file: %v\n", err.Error())
		os.Exit(1)
	}
	entry1.shaSum = sha256.Sum256(tempBuf)

	entry2.aType = DATABASE
	entry2.name = "testdb2.sqlite"
	tempBuf, err = ioutil.ReadFile("/Users/jc/tmp/testdb2.sqlite")
	if err != nil {
		log.Printf("Something went wrong when reading in the database file: %v\n", err.Error())
		os.Exit(1)
	}
	entry2.shaSum = sha256.Sum256(tempBuf)

	entry3.aType = DATABASE
	entry3.name = "testdb3.sqlite"
	tempBuf, err = ioutil.ReadFile("/Users/jc/tmp/testdb3.sqlite")
	if err != nil {
		log.Printf("Something went wrong when reading in the database file: %v\n", err.Error())
		os.Exit(1)
	}
	entry3.shaSum = sha256.Sum256(tempBuf)

	// Populate a dbTree structure with the entries
	var someTree dbTree
	someTree.entries = append(someTree.entries, entry1)
	someTree.entries = append(someTree.entries, entry2)
	someTree.entries = append(someTree.entries, entry3)
	someTree.id = createDBTreeID(someTree.entries)

	// Construct an initial commit structure pointing to the entry
	var someCommit commit
	someCommit.authorEmail = "justin@postgresql.org"
	someCommit.authorName = "Justin Clift"
	someCommit.message = "Initial database upload"
	someCommit.timestamp = time.Now()
	someCommit.tree = someTree.id

	someCommit.id = createCommitID(someCommit)

}

func createCommitID(com commit) [32]byte {
	var b bytes.Buffer
	b.WriteString("tree " + hex.EncodeToString(com.tree[:]) + "\n")
	if com.parent != NILSHA256 {
		b.WriteString("parent " + hex.EncodeToString(com.parent[:]) + "\n")
	}
	b.WriteString("author " + com.authorName + " <" + com.authorEmail + "> " +
		com.timestamp.Format(time.UnixDate) + "\n")
	if com.committerEmail != "" {
		b.WriteString("committer " + com.committerName + " <" + com.committerEmail + "> " +
			com.timestamp.Format(time.UnixDate) + "\n")
	}
	b.WriteString("\n" + com.message)
//spew.Dump(b)
	return sha256.Sum256(b.Bytes())
}

func createDBTreeID(entries []dbTreeEntry) [32]byte {
	var buf bytes.Buffer
	for _, j := range entries {
		buf.WriteString(string(j.aType))
		buf.WriteByte(0) // null byte
		buf.WriteString(hex.EncodeToString(j.shaSum[:]))
		buf.WriteByte(0) // null byte
		buf.WriteString(j.name + "\n")
	}
//spew.Dump(buf)
	return sha256.Sum256(buf.Bytes())
}
