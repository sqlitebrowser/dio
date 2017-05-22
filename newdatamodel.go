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
	authorEmail string
	authorName  string
	id          [32]byte
	message     string
	parent      [32]byte
	timestamp   time.Time
	tree        [32]byte
}

type DBTreeEntryType string

const (
	TREE     DBTreeEntryType = "tree"
	DATABASE                 = "db"
)

type dbTree struct {
	id      [32]byte
	entries []dbTreeEntry
}
type dbTreeEntry struct {
	aType  DBTreeEntryType
	shaSum [32]byte
	name   string
}

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

//log.Printf("SHA256: %s\n", hex.EncodeToString(someTree.id[:]))

	// Construct an initial commit structure pointing to the entry
	//var someCommit commit
	//someCommit.tree = someTree.id

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
