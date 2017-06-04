package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	rest "github.com/emicklei/go-restful"
	"github.com/jackc/pgx"
	"github.com/minio/go-homedir"
	"github.com/minio/minio-go"
)

// Number of connections to PostgreSQL to use
const PGConnections = 5

// Configuration file
type TomlConfig struct {
	Minio MinioInfo
	Pg    PGInfo
}

// Minio connection parameters
type MinioInfo struct {
	AccessKey string `toml:"access_key"`
	HTTPS     bool
	Secret    string
	Server    string
}

// PostgreSQL connection parameters
type PGInfo struct {
	Database string
	Port     int
	Password string
	Server   string
	Username string
}

// Our configuration info
var (
	conf        TomlConfig
	minioClient *minio.Client
	pdb         *pgx.ConnPool
)

func main() {
	// Read our config settings
	userHome, err := homedir.Dir()
	if err != nil {
		log.Printf("User home directory couldn't be determined: %s", "\n")
		os.Exit(1)
	}
	configFile := filepath.Join(userHome, ".dbhub", "dio.toml")
	if _, err := toml.DecodeFile(configFile, &conf); err != nil {
		log.Printf("Config file couldn't be parsed: %v\n", err)
		os.Exit(1)
	}

	// Connect to PostgreSQL
	pgConfig := new(pgx.ConnConfig)
	pgConfig.Host = conf.Pg.Server
	pgConfig.Port = uint16(conf.Pg.Port)
	pgConfig.User = conf.Pg.Username
	pgConfig.Password = conf.Pg.Password
	pgConfig.Database = conf.Pg.Database
	pgConfig.TLSConfig = nil
	pgPoolConfig := pgx.ConnPoolConfig{*pgConfig, PGConnections, nil,
		2 * time.Second}
	pdb, err = pgx.NewConnPool(pgPoolConfig)
	if err != nil {
		log.Printf("Couldn't connect to PostgreSQL server: %v\n", err)
		os.Exit(1)
	}
	log.Printf("Connected to PostgreSQL server: %v:%v\n", pgConfig.Host, pgConfig.Port)

	// Connect to Minio
	minioClient, err = minio.New(conf.Minio.Server, conf.Minio.AccessKey, conf.Minio.Secret, conf.Minio.HTTPS)
	if err != nil {
		log.Printf("Problem with Minio server configuration: %v\n", err.Error())
		return
	}
	log.Printf("Minio server config ok. Address: %v\n", conf.Minio.Server)

	// Create and start the API server
	ws := new(rest.WebService)
	ws.Filter(rest.NoBrowserCacheFilter)
	ws.Route(ws.POST("/branch_create").To(branchCreate))
	ws.Route(ws.POST("/branch_default_change").To(branchDefaultChange))
	ws.Route(ws.GET("/branch_default_get").To(branchDefaultGet))
	ws.Route(ws.GET("/branch_history").To(branchHistory))
	ws.Route(ws.GET("/branch_list").To(branchList))
	ws.Route(ws.POST("/branch_remove").To(branchRemove))
	ws.Route(ws.POST("/branch_revert").To(branchRevert))
	ws.Route(ws.POST("/branch_update").To(branchUpdate))
	ws.Route(ws.GET("/db_download").To(dbDownload))
	ws.Route(ws.GET("/db_list").To(dbList))
	ws.Route(ws.POST("/db_upload").To(dbUpload))
	ws.Route(ws.POST("/tag_create").To(tagCreate))
	ws.Route(ws.GET("/tag_list").To(tagList))
	ws.Route(ws.POST("/tag_remove").To(tagRemove))
	rest.Add(ws)
	http.ListenAndServe(":8080", nil)

	// Close connection to PostgreSQL
	pdb.Close()
}

// Creates a new branch for a database.
// Can be tested with: curl -d database=a.db -d branch=master -d commit=xxx http://localhost:8080/branch_create
func branchCreate(r *rest.Request, w *rest.Response) {
	// Retrieve the database and branch names
	dbName := r.Request.Header.Get("database")
	branchDesc := r.Request.Header.Get("desc")
	branchName := r.Request.Header.Get("branch")
	commitID := r.Request.Header.Get("commit")

	// Sanity check the inputs
	if dbName == "" || branchName == "" || commitID == "" { // branchDesc can be empty as its optional
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: Validate the inputs

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Load the existing branch heads from disk
	branches, err := getBranches(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Ensure the new branch name doesn't already exist in the database
	if _, ok := branches[branchName]; ok {
		w.WriteHeader(http.StatusConflict)
		return
	}

	// Ensure the requested commit exists in the database history
	// This is important, because if we didn't do it then people could supply any commit ID.  Even one's (in our
	// multi-user backend) belonging to completely unrelated databases, which they shouldn't have access to.
	// By ensuring the requested commit is already part of the existing database history, we solve that problem.
	commitExists := false
	for _, b := range branches {
		// Walk the commit history looking for the commit
		c := commitEntry{Parent: b.Commit}
		for c.Parent != "" {
			c, err = getCommit(dbName, c.Parent)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if c.ID == commitID {
				// This commit in the history matches the requested commit, so we're good
				commitExists = true
				break
			}
		}
	}

	// The commit wasn't found, so don't create the new branch
	if commitExists != true {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Create the new branch
	branches[branchName] = branchEntry{Commit: commitID, Description: branchDesc}
	err = storeBranches(dbName, branches)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Changes the default branch for a database.
// Can be tested with: curl -d database=a.db -d branch=master -d commit=xxx http://localhost:8080/branch_default_change
func branchDefaultChange(r *rest.Request, w *rest.Response) {
	dbName := r.Request.Header.Get("database")
	branchName := r.Request.Header.Get("branch")

	// Sanity check the inputs
	if dbName == "" || branchName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: Validate the database and branch names

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Load the existing branch heads from disk
	branches, err := getBranches(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Ensure the branch exists in the database
	if _, ok := branches[branchName]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Write the new default branch to disk
	err = storeDefaultBranchName(dbName, branchName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Changes the default branch for a database.
// Can be tested with: $ dio branch default get a.db
func branchDefaultGet(r *rest.Request, w *rest.Response) {
	dbName := r.Request.Header.Get("database")

	// Sanity check the input
	if dbName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: Validate the database name

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Return the default branch name
	b, err := getDefaultBranchName(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write([]byte(b))
}

// Returns the history for a branch.
// Can be tested with: curl 'http://localhost:8080/branch_history?database=a.db&branch=master'
func branchHistory(r *rest.Request, w *rest.Response) {
	// Retrieve the database and branch names
	dbName := r.Request.Header.Get("database")
	branchName := r.Request.Header.Get("branch")

	// TODO: Validate the database and branch names

	// Sanity check the inputs
	if dbName == "" || branchName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Load the existing branch heads from disk
	branches, err := getBranches(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Ensure the requested branch exists in the database
	b, ok := branches[branchName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Walk the commit history, assembling it into something useful
	var history []commitEntry
	c := commitEntry{Parent: b.Commit}
	for c.Parent != "" {
		c, err = getCommit(dbName, c.Parent)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		history = append(history, c)
	}
	w.WriteAsJson(history)
}

// Returns the list of branch heads for a database.
// Can be tested with: curl http://localhost:8080/branch_list?database=a.db
func branchList(r *rest.Request, w *rest.Response) {
	// Retrieve the database name
	dbName := r.Request.Header.Get("database")

	// TODO: Validate the database name

	// Sanity check the input
	if dbName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Load the existing branch heads from disk
	branches, err := getBranches(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Return the list of branch heads
	w.WriteAsJson(branches)
}

// Removes a branch from a database.
// Can be tested with: curl -d database=a.db -d branch=master http://localhost:8080/branch_remove
func branchRemove(r *rest.Request, w *rest.Response) {
	// Retrieve the database and branch name
	dbName := r.Request.Header.Get("database")
	branchName := r.Request.Header.Get("branch")

	// Sanity check the inputs
	if dbName == "" || branchName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: Validate the database and branch names

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Load the existing branch heads from disk
	branches, err := getBranches(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Ensure the branch exists in the database
	if _, ok := branches[branchName]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Ensure the branch isn't the default for the database
	defBranch, err := getDefaultBranchName(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if branchName == defBranch {
		w.WriteHeader(http.StatusConflict)
		return
	}

	// * Check if any tags exist which are only on this branch.  If so, tell the client about it + don't nuke the
	// branch *

	// Grab the list of tags for the database
	tags, err := getTags(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Walk the commit tree, assembling a list of which branches each tag is on
	tagBranchList := make(map[string][]string)
	for bName, bEntry := range branches {
		c := commitEntry{Parent: bEntry.Commit}
		for c.Parent != "" {
			c, err = getCommit(dbName, c.Parent)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			for tagName, tagData := range tags {
				if tagData.Commit == c.ID {
					// This tag is on this commit, so add this branch to the tag's branch list
					a, ok := tagBranchList[tagName]
					if !ok {
						a = []string{}
					}
					a = append(a, bName)
					tagBranchList[tagName] = a
				}
			}
		}
	}

	// Check if any are only on this branch
	var isolatedTags []string
	for tagName, tagBranches := range tagBranchList {
		if len(tagBranches) == 1 && tagBranches[0] == branchName {
			isolatedTags = append(isolatedTags, tagName)
		}
	}
	if len(isolatedTags) > 0 {
		// Potentially isolated tags were found.  Return the details to the API caller.
		e := errorInfo{Condition: "isolated_tags", Data: isolatedTags}
		j, err := json.MarshalIndent(e, "", " ")
		if err != nil {
			log.Printf("Something went wrong serialising the error information: %v\n", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(j))
		return
	}

	// Remove the branch
	delete(branches, branchName)
	err = storeBranches(dbName, branches)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reverts a branch to a previous commit.
// Can be tested with: curl -d database=a.db -d branch=master -d commit=xxx http://localhost:8080/branch_revert
func branchRevert(r *rest.Request, w *rest.Response) {
	// Retrieve the database and branch names
	dbName := r.Request.Header.Get("database")
	branchName := r.Request.Header.Get("branch")
	commitID := r.Request.Header.Get("commit")
	tag := r.Request.Header.Get("tag")

	// Sanity check the inputs
	if dbName == "" || branchName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if commitID == "" && tag == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if commitID != "" && tag != "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: Validate the database and branch names

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// If we were given a tag, load it's corresponding commit
	if tag != "" {
		tags, err := getTags(dbName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		c, ok := tags[tag]
		if !ok {
			// Requested tag wasn't found
			w.WriteHeader(http.StatusNotFound)
			return
		}
		commitID = c.Commit
	}

	// Load the existing branch heads from disk
	branches, err := getBranches(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Ensure the branch exists in the database
	b, ok := branches[branchName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// * Ensure the requested commit exists in the branch history *

	// If the head commit of the branch already matches the requested commit, there's nothing to change
	if b.Commit == commitID {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// It didn't, so walk the branch history looking for the commit there
	commitExists := false
	c := commitEntry{Parent: b.Commit}
	for c.Parent != "" {
		c, err = getCommit(dbName, c.Parent)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if c.ID == commitID {
			// This commit in the branch history matches the requested commit, so we're good to proceed
			commitExists = true
			break
		}
	}

	// The commit wasn't found, so don't update the branch
	if commitExists != true {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// * Check if any tags exist which are only on this branch, and which would no longer point to a commit on the
	// branch.  If so, tell the client about it + don't nuke the branch *

	// Grab the list of tags for the database
	tags, err := getTags(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Walk the commit tree, assembling a list of which branches each tag is on
	tagBranchList := make(map[string][]string)
	for bName, bEntry := range branches {
		c := commitEntry{Parent: bEntry.Commit}
		for c.Parent != "" {
			c, err = getCommit(dbName, c.Parent)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			for tagName, tagData := range tags {
				if tagData.Commit == c.ID {
					// This tag is on this commit, so add this branch to the tag's branch list
					a, ok := tagBranchList[tagName]
					if !ok {
						a = []string{}
					}
					a = append(a, bName)
					tagBranchList[tagName] = a
				}
			}
		}
	}

	// Check if any are only on this branch
	thisBranchTags := make(map[string]tagEntry)
	for tagName, tagBranches := range tagBranchList {
		if len(tagBranches) == 1 && tagBranches[0] == branchName {
			t := tagEntry{Commit: tags[tagName].Commit}
			thisBranchTags[tagName] = t // Unlike the branchRemove() function above, we grab the tag data too
		}
	}
	if len(thisBranchTags) > 0 {
		// Tags exist which are only on this branch.  So we walk the commits backwards from the selected branch head
		// to the commit to be reverted to, checking of any of them match these tags.  If they do we need to let the
		// caller know + abort the revert
		var isolatedTags []string
		c := commitEntry{Parent: b.Commit}
		for c.Parent != "" {
			c, err = getCommit(dbName, c.Parent)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			for tName, tEntry := range thisBranchTags {
				if tEntry.Commit == c.ID {
					// The commits match, so store the tag name in order to warn the caller
					isolatedTags = append(isolatedTags, tName)
				}
			}
			// If we're on the last commit we need to check, trigger the loop finishing condition
			if c.Parent == commitID {
				c.Parent = ""
			}
		}
		if len(isolatedTags) > 0 {
			// Potentially isolated tags were found.  Return the details to the API caller.
			e := errorInfo{Condition: "isolated_tags", Data: isolatedTags}
			j, err := json.MarshalIndent(e, "", " ")
			if err != nil {
				log.Printf("Something went wrong serialising the error information: %v\n", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(j))
			return
		}

	}

	// Update the branch
	a := branches[branchName]
	a.Commit = commitID
	branches[branchName] = a
	err = storeBranches(dbName, branches)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Updates the description for a branch
// Can be tested with: $ dio branch update a.db --branch foo --description "An AMAZING! description"
func branchUpdate(r *rest.Request, w *rest.Response) {
	// Retrieve the database and branch names
	dbName := r.Request.Header.Get("database")
	branchDesc := r.Request.Header.Get("desc")
	branchName := r.Request.Header.Get("branch")

	// Sanity check the inputs
	if dbName == "" || branchName == "" || branchDesc == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: Validate the inputs

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Load the existing branch heads from disk
	branches, err := getBranches(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Ensure the branch exists in the database
	b, ok := branches[branchName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Update the branch
	b.Description = branchDesc
	branches[branchName] = b
	err = storeBranches(dbName, branches)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Download a database
// Can be tested with: curl -OJ 'http://localhost:8080/db_download?database=a.db&branch=master'
// or curl -OJ 'http://localhost:8080/db_download?database=a.db&commit=xxx'
func dbDownload(r *rest.Request, w *rest.Response) {
	// Retrieve the database and branch names
	dbName := r.Request.Header.Get("database")
	branchName := r.Request.Header.Get("branch")
	reqCommit := r.Request.Header.Get("commit")

	// TODO: Validate the database and branch names

	// Sanity check the inputs
	if dbName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Grab the branch list for the database
	branches, err := getBranches(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Determine the database commit ID
	var dbID string
	if reqCommit != "" {
		// * It was given by the user *

		// Ensure the commit exists in the database history
		commitExists := false
		for _, head := range branches {
			// Walk the branch history looking for the commit
			var e dbTreeEntry
			var t dbTree
			c := commitEntry{Parent: head.Commit}
			for c.Parent != "" {
				c, err = getCommit(dbName, c.Parent)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				if reqCommit == c.ID {
					// Found a match, so retrieve the database ID for the commit
					t, err = getTree(c.Tree.ID)
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					for _, e = range t.Entries {
						if e.Name == dbName {
							dbID = e.Sha256
							commitExists = true
							break
						}
					}
				}
			}
		}

		// The requested commit isn't in the database history
		if !commitExists {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	} else {
		// * We'll need to figure the database file ID from branch history *

		// If no branch name was given, use the default for the database
		if branchName == "" {
			branchName, err = getDefaultBranchName(dbName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		// Retrieve the commit ID for the branch
		b, ok := branches[branchName]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Retrieve the tree ID from the commit
		c, err := getCommit(dbName, b.Commit)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		treeID := c.Tree

		// Retrieve the database ID from the tree
		t, err := getTree(treeID.ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for _, e := range t.Entries {
			if e.Name == dbName {
				dbID = e.Sha256
			}
		}
		if dbID == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// Send the database
	db, err := getDatabase(dbID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", dbName))
	w.Header().Set("Content-Type", "application/x-sqlite3")
	w.Write(db)
}

// Get the list of databases.
// Can be tested with: curl http://localhost:8080/db_list
// or dio list
func dbList(r *rest.Request, w *rest.Response) {
	dbList, err := listDatabases()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(dbList)
}

// Upload a database.
// Can be tested with: dio push
func dbUpload(r *rest.Request, w *rest.Response) {
	// Retrieve metadata from the post headers
	authorName := r.Request.Header.Get("Author")
	branchName := r.Request.Header.Get("Branch") // Optional
	dbName := r.Request.Header.Get("Database")
	email := r.Request.Header.Get("Email")
	msg := r.Request.Header.Get("Message")
	modTime := r.Request.Header.Get("Modtime") // Optional

	// TODO: Validate the inputs

	// Sanity check the inputs
	if authorName == "" || email == "" || dbName == "" || msg == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Grab the uploaded database
	tempFile, _, err := r.Request.FormFile("file1")
	if err != nil {
		log.Printf("Uploading file failed: %v\n", err)
		return
	}
	defer tempFile.Close()
	var buf bytes.Buffer
	_, err = io.Copy(&buf, tempFile)
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	sha := sha256.Sum256(buf.Bytes())

	// Create a dbTree entry for the individual database file
	var e dbTreeEntry
	e.AType = DATABASE
	e.Sha256 = hex.EncodeToString(sha[:])
	e.Name = dbName
	e.Last_Modified, err = time.Parse(time.RFC3339, modTime)
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	e.Size = buf.Len()

	// Create a dbTree structure for the database entry
	var t dbTree
	t.Entries = append(t.Entries, e)
	t.ID = createDBTreeID(t.Entries)

	// Construct a commit structure pointing to the tree
	var c commitEntry
	c.AuthorName = authorName
	c.AuthorEmail = email
	c.Message = msg
	c.Timestamp = time.Now()
	c.Tree = t

	// Check if the database already exists
	var branches map[string]branchEntry
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if exists {
		// Load the existing branchHeads for the database
		branches, err = getBranches(dbName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// If no branch name was given, use the default for the database
		if branchName == "" {
			branchName, err = getDefaultBranchName(dbName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		// Ensure the desired branch already exists.  Use its head commit as the parent for our new uploads' commit
		b, ok := branches[branchName]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		c.Parent = b.Commit
	} else {
		// No existing branches, so this will be the first
		branches = make(map[string]branchEntry)

		// Set the default branch name for the database
		if branchName == "" {
			branchName = "master"
		}
		err := storeDefaultBranchName(dbName, branchName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	// Update the branch with the commit for this new database upload
	c.ID = createCommitID(c)
	b := branches[branchName]
	b.Commit = c.ID
	branches[branchName] = b
	err = storeDatabase(buf.Bytes())
	if err != nil {
		log.Printf("Error when writing database '%s' to disk: %v\n", dbName, err.Error())

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write the commit to disk
	err = storeCommit(dbName, c)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write the updated branch heads to disk
	err = storeBranches(dbName, branches)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Log the upload
	log.Printf("Database uploaded.  Name: '%s', size: %d bytes, branch: '%s'\n", dbName, buf.Len(),
		branchName)

	// Send a 201 "Created" response, along with the location of the URL for working with the (new) database
	w.AddHeader("Location", "/"+dbName)
	w.WriteHeader(http.StatusCreated)
}

// Creates a new tag for a database.
// Can be tested with: curl -d database=a.db -d tag=foo -d commit=xxx http://localhost:8080/tag_create
// or curl -d database=a.db -d tag=foo -d commit=xxx -d taggeremail=foo@bar.com -d taggername="Some person" \
//   -d msg="My tag message" -d date="2017-05-28T07:15:46%2b01:00" http://localhost:8080/tag_create
// Note the URL encoded + sign (%2b) in the date argument above, as the + sign doesn't get through otherwise
func tagCreate(r *rest.Request, w *rest.Response) {
	// Retrieve the database and tag names, and the commit ID
	commitID := r.Request.Header.Get("commit")    // Required
	date := r.Request.Header.Get("date")          // Optional
	dbName := r.Request.Header.Get("database")    // Required
	tEmail := r.Request.Header.Get("taggeremail") // Only for annotated commits
	tName := r.Request.Header.Get("taggername")   // Only for annotated commits
	msg := r.Request.Header.Get("msg")            // Only for annotated commits
	tag := r.Request.Header.Get("tag")            // Required

	// Ensure at least the minimum inputs were provided
	if dbName == "" || tag == "" || commitID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	isAnno := false
	if tEmail != "" || tName != "" || msg != "" {
		// If any of these fields are filled out, it's an annotated commit, so make sure they
		// all have a value
		if tEmail == "" || tName == "" || msg == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		isAnno = true
	}

	// TODO: Validate the inputs

	var tDate time.Time
	var err error
	if date == "" {
		tDate = time.Now()
	} else {
		tDate, err = time.Parse(time.RFC3339, date)
		if err != nil {
			log.Printf("Error when converting date: %v\n", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Load the existing tags from disk
	tags, err := getTags(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Ensure the new tag doesn't already exist for the database
	if _, ok := tags[tag]; ok {
		w.WriteHeader(http.StatusConflict)
		return
	}

	// Load the existing branch heads from disk
	branches, err := getBranches(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Ensure the requested commit exists in the database history
	// This is important, because if we didn't do it then people could supply any commit ID.  Even one's belonging
	// to completely unrelated databases, which they shouldn't have access to.
	// By ensuring the requested commit is already part of the existing database history, we solve that problem.
	commitExists := false
	for _, b := range branches {
		// Walk the commit history looking for the commit
		c := commitEntry{Parent: b.Commit}
		for c.Parent != "" {
			c, err = getCommit(dbName, c.Parent)
			if err != nil {
				if err.Error() == "Requested commit not found" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if c.ID == commitID {
				// This commit in the history matches the requested commit, so we're good
				commitExists = true
				break
			}
		}
	}

	// The commit wasn't found, so don't create the new tag
	if commitExists != true {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Create the new tag
	var t tagEntry
	if isAnno == true {
		// It's an annotated commit
		t = tagEntry{
			Commit:      commitID,
			Date:        tDate,
			Message:     msg,
			TagType:     ANNOTATED,
			TaggerEmail: tEmail,
			TaggerName:  tName,
		}
	} else {
		// It's a simple commit
		t = tagEntry{
			Commit:      commitID,
			Date:        tDate,
			Message:     "",
			TagType:     SIMPLE,
			TaggerEmail: "",
			TaggerName:  "",
		}
	}
	tags[tag] = t
	err = storeTags(dbName, tags)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Returns the list of tags for a database.
// Can be tested with: curl http://localhost:8080/tag_list?database=a.db
func tagList(r *rest.Request, w *rest.Response) {
	// Retrieve the database name
	dbName := r.Request.Header.Get("database")

	// TODO: Validate the database name

	// Sanity check the input
	if dbName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Load the existing tags from disk
	tags, err := getTags(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Return the list of tags
	w.WriteAsJson(tags)
}

// Removes a tag from a database.
// Can be tested with: curl -d database=a.db -d tag=foo http://localhost:8080/tag_remove
func tagRemove(r *rest.Request, w *rest.Response) {
	// Retrieve the database and tag name
	dbName := r.Request.Header.Get("database")
	tag := r.Request.Header.Get("tag")

	// Sanity check the inputs
	if dbName == "" || tag == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: Validate the inputs

	// Ensure the requested database is in our system
	exists, err := dbExists(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Load the existing tags from disk
	tags, err := getTags(dbName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Ensure the tag exists in the database
	if _, ok := tags[tag]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Remove the tag
	delete(tags, tag)
	err = storeTags(dbName, tags)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
