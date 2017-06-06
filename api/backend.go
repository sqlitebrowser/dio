package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/jackc/pgx"
	"github.com/pkg/errors"
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
func getBranches(dbName string) (branches map[string]branchEntry, err error) {
	dbQuery := `
		SELECT "branchHeads"
		FROM sqlite_databases
		WHERE "dbName" = $1`
	err = pdb.QueryRow(dbQuery, dbName).Scan(&branches)
	if err != nil {
		log.Printf("Error when retrieving branch heads for database '%v': %v\n", dbName, err)
		return nil, err
	}
	return branches, nil
}

// Reads a commit from disk.
func getCommit(dbName string, id string) (commitEntry, error) {
	// Retrieve all of the commits from the database
	// TODO: We can probably directly retrieve the desired commit from PG using a jsonb operator.  eg @> 'something'
	dbQuery := `
		SELECT "commitList"
		FROM sqlite_databases
		WHERE "dbName" = $1`
	var list []commitEntry
	err := pdb.QueryRow(dbQuery, dbName).Scan(&list)
	if err != nil {
		log.Printf("Error when retrieving commit list for database '%v': %v\n", dbName, err)
		return commitEntry{}, err
	}

	// Return the individual commit we want
	var c commitEntry
	for _, j := range list {
		if j.ID == id {
			c = j
			break
		}
	}
	if c.ID == "" {
		return c, errors.New("Requested commit not found")
	}
	return c, nil
}

// Retrieves a database from Minio.
func getDatabase(sha string) (io.ReadCloser, error) {
	bkt := sha[0:6]
	id := sha[6:]
	db, err := minioClient.GetObject(bkt, id)
	if err != nil {
		log.Printf("Error retrieving DB from Minio: %v\n", err)
		return nil, errors.New("Error retrieving database from internal storage")
	}
	return db, nil
}

// Retrieve the default branch name for a database.
func getDefaultBranchName(dbName string) (string, error) {
	// Return the default branch name
	dbQuery := `
		SELECT "dbDefaultBranch"
		FROM sqlite_databases
		WHERE "dbName" = $1`
	var branchName string
	err := pdb.QueryRow(dbQuery, dbName).Scan(&branchName)
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

// Returns the text for a given licence.
func getLicence(lName string) (lText string, err error) {
	dbQuery := `
		SELECT "licenceText"
		FROM database_licences
		WHERE "friendlyName" = $1`
	err = pdb.QueryRow(dbQuery, lName).Scan(&lText)
	if err != nil {
		log.Printf("Error when retrieving licence '%s' from database: %v\n", lName, err)
		return "", err
	}
	if lName == "" {
		// The requested licence text wasn't found
		return "", errors.New("Licence text not found")
	}
	return lText, nil
}

// Load the tags for a database.
func getTags(dbName string) (tags map[string]tagEntry, err error) {
	dbQuery := `
		SELECT tags
		FROM sqlite_databases
		WHERE "dbName" = $1`
	err = pdb.QueryRow(dbQuery, dbName).Scan(&tags)
	if err != nil {
		log.Printf("Error when retrieving branch heads for database '%v': %v\n", dbName, err)
		return nil, err
	}
	if tags == nil {
		// If there aren't any tags yet, return an empty set instead of nil
		tags = make(map[string]tagEntry)
	}
	return tags, nil
}

// Check if a licence already exists.
// TODO: We'll probably need some way to check if a licence already exists via sha256 as well.
func licExists(lName string) (bool, error) {
	dbQuery := `
		SELECT count("friendlyName")
		FROM database_licences
		WHERE "friendlyName" = $1`
	var q int
	err := pdb.QueryRow(dbQuery, lName).Scan(&q)
	if err != nil {
		log.Printf("Error when checking if licence '%v' exists: %v\n", lName, err)
		return false, err
	}
	if q == 0 {
		// Licence doesn't exist
		return false, nil
	}
	// Licence does exist
	return true, nil
}

// Returns the list of available databases.
func listDatabases() ([]byte, error) {
	dbQuery := `
		SELECT "dbName"
		FROM sqlite_databases`
	rows, err := pdb.Query(dbQuery)
	if err != nil {
		log.Printf("Database query failed: %v\n", err)
		return nil, err
	}
	defer rows.Close()
	var dbs []dbListEntry
	for rows.Next() {
		var oneRow dbListEntry
		err = rows.Scan(&oneRow.Database)
		if err != nil {
			log.Printf("Error retrieving database list for user: %v\n", err)
			return nil, err
		}
		dbs = append(dbs, oneRow)
	}

	// TODO: For the real code, we'll want to extract the last modified date and file size of (say) the latest revision
	// TODO  of each database

	// Convert into json
	j, err := json.MarshalIndent(dbs, "", " ")
	if err != nil {
		log.Printf("Something went wrong serialising the branch data: %v\n", err.Error())
		return []byte{}, err
	}

	return j, nil
}

// Returns the list of available licences.
func listLicences() ([]byte, error) {
	dbQuery := `
		SELECT "friendlyName", sha256, "sourceURL"
		FROM database_licences
		ORDER BY "friendlyName"`
	rows, err := pdb.Query(dbQuery)
	if err != nil {
		log.Printf("Database query failed: %v\n", err)
		return nil, err
	}
	defer rows.Close()
	var lics []licenceEntry
	for rows.Next() {
		var oneRow licenceEntry
		err = rows.Scan(&oneRow.Name, &oneRow.Sha256, &oneRow.URL)
		if err != nil {
			log.Printf("Error retrieving licence list: %v\n", err)
			return nil, err
		}
		lics = append(lics, oneRow)
	}

	// Convert into json
	j, err := json.MarshalIndent(lics, "", " ")
	if err != nil {
		log.Printf("Something went wrong serialising the licence data: %v\n", err.Error())
		return []byte{}, err
	}

	return j, nil
}

// Remove an available licence from our system.
func removeLicence(lic string) error {
	dbQuery := `
		DELETE FROM database_licences
		WHERE "friendlyName" = $1`
	commandTag, err := pdb.Exec(dbQuery, lic)
	if err != nil {
		log.Printf("Removing licence '%v' failed: %v\n", lic, err)
		return err
	}
	if numRows := commandTag.RowsAffected(); numRows != 1 {
		log.Printf("Wrong number of rows (%v) affected when removing licence from database: '%v'\n",
			numRows, lic)
	}
	return nil
}

// Store the branch heads for a database.
func storeBranches(dbName string, branches map[string]branchEntry) error {
	dbQuery := `
		UPDATE sqlite_databases
		SET "branchHeads" = $2
		WHERE "dbName" = $1`
	commandTag, err := pdb.Exec(dbQuery, dbName, branches)
	if err != nil {
		log.Printf("Storing branch heads for database '%v' failed: %v\n", dbName, err)
		return err
	}
	if numRows := commandTag.RowsAffected(); numRows != 1 {
		log.Printf("Wrong number of rows (%v) affected when storing branch heads for database: '%v'\n", numRows,
			dbName)
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
	bkt := sha[0:6]
	id := sha[6:]

	// If a Minio bucket with the desired name doesn't already exist, create it
	found, err := minioClient.BucketExists(bkt)
	if err != nil {
		log.Printf("Error when checking if Minio bucket '%s' already exists: %v\n", bkt, err)
		return err
	}
	if !found {
		err := minioClient.MakeBucket(bkt, "us-east-1")
		if err != nil {
			log.Printf("Error creating Minio bucket '%v': %v\n", bkt, err)
			return err
		}
	}

	// Store the SQLite database file in Minio
	dbSize, err := minioClient.PutObject(bkt, id, bytes.NewReader(db), "application/x-sqlite3")
	if err != nil {
		log.Printf("Storing file in Minio failed: %v\n", err)
		return err
	}
	// Sanity check.  Make sure the # of bytes written is equal to the size of the buffer we were given
	if len(db) != int(dbSize) {
		log.Printf("Something went wrong storing the database file: %v\n", err.Error())
		return err
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

// Store a licence.
func storeLicence(lName string, lTxt []byte, srcURL string) error {
	sha := sha256.Sum256(lTxt)
	dbQuery := `
		INSERT INTO database_licences ("friendlyName", sha256, "licenceText", "sourceURL")
		VALUES ($1, $2, $3, $4)`
	commandTag, err := pdb.Exec(dbQuery, lName, hex.EncodeToString(sha[:]), lTxt, srcURL)
	if err != nil {
		log.Printf("Inserting licence '%v' in database failed: %v\n", lName, err)
		return err
	}
	if numRows := commandTag.RowsAffected(); numRows != 1 {
		log.Printf("Wrong number of rows (%v) affected during insert: licence name: '%v'\n", numRows,
			lName)
	}
	return nil
}

// Store the tags (standard, non-annotated type) for a database.
func storeTags(dbName string, tags map[string]tagEntry) error {
	dbQuery := `
		UPDATE sqlite_databases
		SET tags = $2
		WHERE "dbName" = $1`
	commandTag, err := pdb.Exec(dbQuery, dbName, tags)
	if err != nil {
		log.Printf("Storing tags for database '%v' failed: %v\n", dbName, err)
		return err
	}
	if numRows := commandTag.RowsAffected(); numRows != 1 {
		log.Printf("Wrong number of rows (%v) affected when storing tags for database: '%v'\n", numRows, dbName)
	}
	return nil
}
