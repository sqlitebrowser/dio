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

	"github.com/jackc/pgx"
)

// Generate a stable SHA256 for a commit.
func createCommitID(c commitEntry) string {
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

// Check if a database already exists.
func dbExists(dbName string) (bool, error) {
	dbQuery := `
		SELECT count("dbName")
		FROM sqlite_databases
		WHERE "dbName" = $1`
	var q int
	err := pdb.QueryRow(dbQuery, dbName).Scan(&q)
	if err != nil {
		log.Printf("Error when checking if database '%v' exists: %v\n", dbName, err)
		return false, err
	}
	if q == 0 {
		// Database doesn't exist
		return false, nil
	}
	// Database does exist
	return true, nil
}

// Load the branch heads for a database.
// TODO: It might be better to have the default branch name be returned as part of this list, by indicating in the list
// TODO  which of the branches is the default.
func getBranches(dbName string) (map[string]branchEntry, error) {
	b, err := ioutil.ReadFile(filepath.Join(STORAGEDIR, "meta", dbName, "branchHeads"))
	if err != nil {
		return nil, err
	}
	var i map[string]branchEntry
	err = json.Unmarshal(b, &i)
	if err != nil {
		log.Printf("Something went wrong unserialising the branchHeads data: %v\n", err.Error())
		return nil, err
	}
	return i, nil
}

// Retrieve the default branch name for a database.
func getDefaultBranchName(dbName string) (string, error) {
	db, err := dbExists(dbName)
	if err != nil {
		return "", err
	}
	if !db {
		// Database doesn't exist, so use "master" as the initial default
		return "master", nil
	}

	// Return the default branch name
	dbQuery := `
		SELECT "dbDefaultBranch"
		FROM sqlite_databases
		WHERE "dbName" = $1`
	var branchName string
	err = pdb.QueryRow(dbQuery).Scan(&branchName)
	if err != nil {
		if err != pgx.ErrNoRows {
			log.Printf("Error when retrieving default branch name for database '%v': %v\n", dbName, err)
			return "", err
		} else {
			log.Printf("No default branch name exists for database '%s'. This shouldn't happen\n", dbName)
			return "", err
		}
	}
	return branchName, nil
}

// Reads a commit from disk.
func getCommit(id string) (commitEntry, error) {
	var c commitEntry
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

// Load the tags for a database.
func getTags(dbName string) (map[string]tagEntry, error) {
	b, err := ioutil.ReadFile(filepath.Join(STORAGEDIR, "meta", dbName, "tags"))
	if err != nil {
		_, ok := err.(*os.PathError)
		if ok {
			// There are no tags for the database yet
			return make(map[string]tagEntry), nil
		}

		log.Printf("Something went wrong reading the tags data: %v\n", err.Error())
		return nil, err
	}
	var i map[string]tagEntry
	err = json.Unmarshal(b, &i)
	if err != nil {
		log.Printf("Something went wrong unserialising the tags data: %v\n", err.Error())
		return nil, err
	}
	return i, nil
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

// Returns the list of available databases.
func listDatabases() ([]byte, error) {
	// For now, just use the entries in the "meta" directory as the list
	p := filepath.Join(STORAGEDIR, "meta")
	dirEntries, err := ioutil.ReadDir(p)
	if err != nil {
		// As this is just experimental code, we'll assume a failure above means the db doesn't exist
		log.Printf("Error when reading database list: %v\n", err)
		return []byte{}, err
	}
	var dbs []dbListEntry
	for _, i := range dirEntries {
		// Get the size and last modified date of each of the databases from it's commit tree entry
		def, err := getDefaultBranchName(i.Name())
		if err != nil {
			return []byte{}, err
		}
		b, err := getBranches(i.Name())
		if err != nil {
			return []byte{}, err
		}
		c, err := getCommit(b[def].Commit)
		if err != nil {
			return []byte{}, err
		}
		t, err := getTree(c.Tree)
		if err != nil {
			return []byte{}, err
		}
		var lastMod time.Time
		var dbSize int
		for _, j := range t.Entries {
			if j.Name == i.Name() {
				lastMod = j.Last_Modified
				dbSize = j.Size
			}
		}
		d := dbListEntry{Database: i.Name(), LastModified: lastMod, Size: dbSize}
		dbs = append(dbs, d)
	}

	// Convert into json
	j, err := json.MarshalIndent(dbs, "", " ")
	if err != nil {
		log.Printf("Something went wrong serialising the branch data: %v\n", err.Error())
		return []byte{}, err
	}

	return j, nil
}

// Store the branch heads for a database.
func storeBranches(dbName string, branches map[string]branchEntry) error {
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
func storeCommit(dbName string, c commitEntry) error {
	dbQuery := `
		INSERT INTO sqlite_databases ("dbName", "commitList")
		VALUES ($1, $2)
		ON CONFLICT ("dbName")
			DO UPDATE
			SET "commitList" = sqlite_databases."commitList" || $2`
	commandTag, err := pdb.Exec(dbQuery, dbName, []commitEntry{c})
	if err != nil {
		log.Printf("Inserting commit '%v' failed: %v\n", c.ID, err)
		return err
	}
	if numRows := commandTag.RowsAffected(); numRows != 1 {
		log.Printf("Wrong number of rows (%v) affected during insert: new commit hash: '%v'\n", numRows, c.ID)
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
	dbQuery := `
		UPDATE sqlite_databases
		SET "dbDefaultBranch" = $1
		WHERE "dbName" = $2`
	commandTag, err := pdb.Exec(dbQuery, branchName, dbName)
	if err != nil {
		log.Printf("Changing default branch for database '%v' to '%v' failed: %v\n", dbName, branchName, err)
		return err
	}
	if numRows := commandTag.RowsAffected(); numRows != 1 {
		log.Printf("Wrong number of rows (%v) affected during update: database: %v, new branch name: '%v'\n",
			numRows, dbName, branchName)
	}
	return nil
}

// Store the tags (standard, non-annotated type) for a database.
func storeTags(dbName string, tags map[string]tagEntry) error {
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

	j, err := json.MarshalIndent(tags, "", " ")
	if err != nil {
		log.Printf("Something went wrong serialising the branch data: %v\n", err.Error())
		return err
	}
	err = ioutil.WriteFile(filepath.Join(STORAGEDIR, "meta", dbName, "tags"), j, os.ModePerm)
	if err != nil {
		log.Printf("Something went wrong writing the tags file: %v\n", err.Error())
		return err
	}
	return nil
}

//// Store a tree.
//func storeTree(t dbTree) error {
//	// Convert the tree entries to json
//	j, err := json.MarshalIndent(t.Entries, "", " ")
//	if err != nil {
//		log.Printf("Something went wrong serialising the tree data: %v\n", err.Error())
//		return err
//	}
//
//	// Store the json in the database
//	dbQuery := `
//		INSERT INTO database_trees (treeHash, treeEntries)
//		VALUES ($1, $2)`
//	commandTag, err := pdb.Exec(dbQuery, t.ID, j)
//	if err != nil {
//		log.Printf("Inserting tree '%v' failed: %v\n", t.ID, err)
//		return err
//	}
//	if numRows := commandTag.RowsAffected(); numRows != 1 {
//		log.Printf("Wrong number of rows (%v) affected during insert: new tree hash: '%v'\n", numRows, t.ID)
//	}
//	return nil
//}
