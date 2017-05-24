package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	rest "github.com/emicklei/go-restful"
)

func main() {
	// Create the storage directories on disk
	err := os.MkdirAll(filepath.Join(STORAGEDIR, "files"), os.ModeDir|0755)
	if err != nil {
		log.Printf("Something went wrong when creating the files dir: %v\n", err.Error())
		return
	}
	err = os.MkdirAll(filepath.Join(STORAGEDIR, "meta"), os.ModeDir|0755)
	if err != nil {
		log.Printf("Something went wrong when creating the meta dir: %v\n", err.Error())
		return
	}

	// Create and start the API server
	ws := new(rest.WebService)
	ws.Filter(rest.NoBrowserCacheFilter)
	ws.Route(ws.PUT("/db_upload").To(dbUpload))
	ws.Route(ws.GET("/db_download").To(dbDownload))
	ws.Route(ws.GET("/db_list").To(dbList))
	rest.Add(ws)
	http.ListenAndServe(":8080", nil)
}

// Upload a database.
// Can be tested with: curl -T a.db -H "Name: a.db" -w \%{response_code} -D headers.out http://localhost:8080/db_upload
func dbUpload(r *rest.Request, w *rest.Response) {
	// Retrieve the database and branch names
	dbName := r.Request.Header.Get("Name")
	branchName := r.Request.Header.Get("Branch")

	// TODO: Validate the database and branch names

	// Default to "master" if no branch name was given
	if branchName == "" {
		branchName = "master"
	}

	// Read the database into a buffer
	var buf bytes.Buffer
	buf.ReadFrom(r.Request.Body)
	sha := sha256.Sum256(buf.Bytes())

	// Create a dbTree entry for the individual database file
	var e dbTreeEntry
	e.AType = DATABASE
	e.Sha256 = hex.EncodeToString(sha[:])
	e.Name = dbName
	e.Last_Modified = time.Now()
	e.Size = buf.Len()

	// Create a dbTree structure for the database entry
	var t dbTree
	t.Entries = append(t.Entries, e)
	t.ID = createDBTreeID(t.Entries)

	// Construct a commit structure pointing to the tree
	var c commit
	c.AuthorEmail = "justin@postgresql.org" // TODO: Author and Committer info should come from the client, so we
	c.AuthorName = "Justin Clift"           // TODO  hard code these for now.  Proper auth will need adding later
	c.Timestamp = time.Now()                // TODO: Would it be better to accept a timestamp from the client?
	c.Tree = t.ID

	// Check if the database already exists
	var err error
	var branches []branch
	if dbExists(dbName) {
		// Load the existing branchHeads from disk
		branches, err = getBranches(dbName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// TODO: If the requested branch already exists, we need to retrieve the commit ID from it,
		// TODO  use that as the "parent" value for our new commit, generate a commit ID, and update
		// TODO  update the commit ID in it
log.Println("Code needs writing for the branchHeads already existing")
	} else {
		// No existing branches, so this will be the first
		c.ID = createCommitID(c)
		branches = append(branches, branch{Commit: c.ID, Name: branchName})
	}

	// Write the database to disk
	err = storeDatabase(buf.Bytes())
	if err != nil {
		log.Printf("Error when writing database '%s' to disk: %v\n", dbName, err.Error())

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write the tree to disk
	err = storeTree(t)
	if err != nil {
		log.Printf("Something went wrong when storing the tree file: %v\n", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write the commit to disk
	err = storeCommit(c)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write the updated branchHeads to disk
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

// Download a database
func dbDownload(r *rest.Request, w *rest.Response) {
	log.Println("dbDownload() called")
}

// Get a list of databases
func dbList(r *rest.Request, w *rest.Response) {
	log.Println("dbList() called")
}
