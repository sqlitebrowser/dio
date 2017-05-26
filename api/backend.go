package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Generate a stable SHA256 for a commit.
func createCommitID(c commit) string {
	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("tree %s\n", c.Tree))
	if c.Parent != "" {
		b.WriteString(fmt.Sprintf("parent %s\n", c.Parent))
	}
	b.WriteString(fmt.Sprintf("author %s <%s> %v\n", c.AuthorName, c.AuthorEmail,
		c.Timestamp.Format(time.UnixDate)))
	if c.CommitterEmail != "" {
		b.WriteString(fmt.Sprintf("committer %s <%s> %v\n", c.CommitterName, c.CommitterEmail,
			c.Timestamp.Format(time.UnixDate)))
	}
	b.WriteString("\n" + c.Message)
	b.WriteByte(0)
	s := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(s[:])
}

// Creates the user visible commit text for a commit.
func createCommitText(c commit) (string, error) {
	if c.ID == "" {
		c.ID = createCommitID(c)
	}
	s := fmt.Sprintf("commit %s\n", c.ID)
	s += fmt.Sprintf("Author: %s <%s>\n", c.AuthorName, c.AuthorEmail)
	s += fmt.Sprintf("Date: %v\n\n", c.Timestamp.Format(time.UnixDate))
	if c.Message != "" {
		s += fmt.Sprintf("    %s\n", c.Message)
	}
	return s, nil
}

// Generate the SHA256 for a tree.
// Tree entry structure is:
// * [ type ] [ sha256 ] [ db name ] [ last modified (timestamp) ] [ db size (bytes) ]
func createDBTreeID(entries []dbTreeEntry) string {
	var b bytes.Buffer
	for _, j := range entries {
		b.WriteString(string(j.AType))
		b.WriteByte(0)
		b.WriteString(j.Sha256)
		b.WriteByte(0)
		b.WriteString(j.Name)
		b.WriteByte(0)
		b.WriteString(j.Last_Modified.Format(time.RFC3339))
		b.WriteByte(0)
		b.WriteString(fmt.Sprintf("%d\n", j.Size))
	}
	s := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(s[:])
}

// Returns the list of available databases.
func databaseList() ([]byte, error) {
	// For now, just use the entries in the "meta" directory as the list
	p := filepath.Join(STORAGEDIR, "meta")
	dirEntries, err := ioutil.ReadDir(p)
	if err != nil {
		// As this is just experimental code, we'll assume a failure above means the db doesn't exist
		log.Printf("Error when reading database list: %v\n", err)
		return []byte{}, err
	}
	var dbs []string
	for _, j := range dirEntries {
		dbs = append(dbs, j.Name())
	}

	// Convert into json
	j, err := json.MarshalIndent(dbs, "", " ")
	if err != nil {
		log.Printf("Something went wrong serialising the branch data: %v\n", err.Error())
		return []byte{}, err
	}

	return j, nil
}

// Check if a database already exists.
func dbExists(dbName string) bool {
	path := filepath.Join(STORAGEDIR, "meta", dbName)
	_, err := os.Stat(path)
	if err != nil {
		// As this is just experimental code, we'll assume a failure above means the db doesn't exist
		// TODO: Proper handling for errors here.  It may not mean the file doesn't exist.
		return false
	}
	return true
}

// Load the branch heads for a database.
func getBranches(dbName string) (map[string]string, error) {
	b, err := ioutil.ReadFile(filepath.Join(STORAGEDIR, "meta", dbName, "branchHeads"))
	if err != nil {
		return nil, err
	}
	var i map[string]string
	err = json.Unmarshal(b, &i)
	if err != nil {
		log.Printf("Something went wrong unserialising the branchHeads data: %v\n", err.Error())
		return nil, err
	}
	return i, nil
}

// Retrieve the default branch name for a database.
func getDefaultBranchName(dbName string) string {
	if !dbExists(dbName) {
		return "master" // Database doesn't exist, so use "master" as the initial default
	}

	// Return the default branch name
	b, err := ioutil.ReadFile(filepath.Join(STORAGEDIR, "meta", dbName, "defaultBranch"))
	if err != nil {
		log.Printf("Error when reading default branch for '%s': %v\n", dbName, err.Error())
		return "master" // An error occurred reading the default branch name, so default to master
	}
	return string(b[:])
}

// Reads a commit from disk.
func getCommit(id string) (commit, error) {
	var c commit
	b, err := ioutil.ReadFile(filepath.Join(STORAGEDIR, "files", id))
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		log.Printf("Something went wrong unserialising a commit's data: %v\n", err.Error())
		return c, err
	}
	return c, nil
}

// Reads a database from disk.
func getDatabase(id string) ([]byte, error) {
	d, err := ioutil.ReadFile(filepath.Join(STORAGEDIR, "files", id))
	if err != nil {
		log.Printf("Error reading file: '%s': %v\n", id, err.Error())
		return []byte{}, err
	}
	return d, nil
}

// Reads a tree from disk.
func getTree(id string) (dbTree, error) {
	var t dbTree
	b, err := ioutil.ReadFile(filepath.Join(STORAGEDIR, "files", id))
	if err != nil {
		log.Printf("Error reading file: '%s': %v\n", id, err.Error())
		return t, err
	}
	err = json.Unmarshal(b, &t)
	if err != nil {
		log.Printf("Something went wrong unserialising a commit's data: %v\n", err.Error())
		return t, err
	}
	return t, nil
}

// Store the branch heads for a database.
func storeBranches(dbName string, branches map[string]string) error {
	path := filepath.Join(STORAGEDIR, "meta", dbName)
	_, err := os.Stat(path)
	if err != nil {
		// As this is just experimental code, we'll assume a failure above means the dir needs creating
		// TODO: Proper handling for errors here.  It may not mean the dir doesn't exist.
		err := os.MkdirAll(filepath.Join(STORAGEDIR, "meta", dbName), os.ModeDir|0755)
		if err != nil {
			log.Printf("Something went wrong creating the database meta dir: %v\n", err.Error())
			return err
		}
	}

	j, err := json.MarshalIndent(branches, "", " ")
	if err != nil {
		log.Printf("Something went wrong serialising the branch data: %v\n", err.Error())
		return err
	}
	err = ioutil.WriteFile(filepath.Join(STORAGEDIR, "meta", dbName, "branchHeads"), j, os.ModePerm)
	if err != nil {
		log.Printf("Something went wrong writing the branchHeads file: %v\n", err.Error())
		return err
	}
	return nil
}

// Store a commit.
func storeCommit(c commit) error {
	j, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		log.Printf("Something went wrong when serialising the commit data: %v\n", err.Error())
		return err
	}
	err = ioutil.WriteFile(filepath.Join(STORAGEDIR, "files", c.ID), j, os.ModePerm)
	if err != nil {
		log.Printf("Something went wrong writing the commit file: %v\n", err.Error())
		return err
	}
	return nil
}

// Store a database file.
func storeDatabase(db []byte) error {
	// Create the database file if it doesn't already exist
	a := sha256.Sum256(db)
	sha := hex.EncodeToString(a[:])
	path := filepath.Join(STORAGEDIR, "files", sha)
	f, err := os.Stat(path)
	if err != nil {
		// As this is just experimental code, we'll assume a failure above means the file needs creating
		// TODO: Proper handling for errors here.  It may not mean the file doesn't exist.
		err = ioutil.WriteFile(path, db, os.ModePerm)
		if err != nil {
			log.Printf("Something went wrong writing the database file: %v\n", err.Error())
			return err
		}
		return nil
	}

	// The file already exists.
	// Check if the file size matches the buffer size we're intending on writing, and skip it if so
	// (Obviously this is just a super lightweight check, not a real world approach)
	// TODO: Add real world checks to ensure the file contents are identical.  Maybe read the file contents into
	// TODO  memory, then binary compare them?  Prob not great for memory efficiency, but it would likely do as a
	// TODO  first go for something accurate.
	if len(db) != int(f.Size()) {
		err = ioutil.WriteFile(path, db, os.ModePerm)
		if err != nil {
			log.Printf("Something went wrong writing the database file: %v\n", err.Error())
			return err
		}
	}
	return nil
}

// Stores the default branch name for a database.
func storeDefaultBranchName(dbName string, branchName string) error {
	var buf bytes.Buffer
	buf.WriteString(branchName)
	err := ioutil.WriteFile(filepath.Join(STORAGEDIR, "meta", dbName, "defaultBranch"), buf.Bytes(), os.ModePerm)
	if err != nil {
		log.Printf("Something went wrong writing the default branch name for '%s': %v\n", dbName,
			err.Error())
		return err
	}
	return nil
}

// Store a tree.
func storeTree(t dbTree) error {
	j, err := json.MarshalIndent(t, "", " ")
	if err != nil {
		log.Printf("Something went wrong serialising the tree data: %v\n", err.Error())
		return err
	}
	err = ioutil.WriteFile(filepath.Join(STORAGEDIR, "files", t.ID), j, os.ModePerm)
	if err != nil {
		log.Printf("Something went wrong writing the tree file: %v\n", err.Error())
		return err
	}
	return nil
}
