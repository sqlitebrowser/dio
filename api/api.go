package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
	ws.Route(ws.POST("/branch_create").To(branchCreate))
	ws.Route(ws.POST("/branch_default_change").To(branchDefaultChange))
	ws.Route(ws.GET("/branch_default_get").To(branchDefaultGet))
	ws.Route(ws.GET("/branch_history").To(branchHistory))
	ws.Route(ws.GET("/branch_list").To(branchList))
	ws.Route(ws.POST("/branch_remove").To(branchRemove))
	ws.Route(ws.POST("/branch_revert").To(branchRevert))
	ws.Route(ws.GET("/db_download").To(dbDownload))
	ws.Route(ws.GET("/db_list").To(dbList))
	ws.Route(ws.POST("/db_upload").To(dbUpload))
	ws.Route(ws.POST("/tag_create").To(tagCreate))
	ws.Route(ws.GET("/tag_list").To(tagList))
	ws.Route(ws.POST("/tag_remove").To(tagRemove))
	rest.Add(ws)
	http.ListenAndServe(":8080", nil)
}

// Creates a new branch for a database.
// Can be tested with: curl -d database=a.db -d branch=master -d commit=xxx http://localhost:8080/branch_create
func branchCreate(r *rest.Request, w *rest.Response) {
	// Retrieve the database and branch names
	dbName := r.Request.Header.Get("database")
	branchName := r.Request.Header.Get("branch")
	commit := r.Request.Header.Get("commit")

	// Sanity check the inputs
	if dbName == "" || branchName == "" || commit == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: Validate the database and branch names

	// Ensure the requested database is in our system
	if !dbExists(dbName) {
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
	// This is important, because if we didn't do it then people could supply any commit ID.  Even one's belonging
	// to completely unrelated databases, which they shouldn't have access to.
	// By ensuring the requested commit is already part of the existing database history, we solve that problem.
	commitExists := false
	for _, ID := range branches {
		// Because there seems to be no valid way of adding this condition in the loop declaration?
		// Do I just need more coffee? :D
		if commitExists == true {
			continue
		}

		// Check if the head commit of the branch matches the requested commit
		if ID == commit {
			commitExists = true
			continue
		}

		// It didn't, so walk the commit history looking for the commit there
		c, err := getCommit(ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for c.Parent != "" {
			c, err = getCommit(c.Parent)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if c.ID == commit {
				// This commit in the history matches the requested commit, so we're good
				commitExists = true
			}
		}
	}

	// The commit wasn't found, so don't create the new branch
	if commitExists != true {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Create the new branch
	branches[branchName] = commit
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
	if !dbExists(dbName) {
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
	if !dbExists(dbName) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Return the default branch name
	b := getDefaultBranchName(dbName)
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
	if !dbExists(dbName) {
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
	id, ok := branches[branchName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Walk the commit history, assembling it into something useful
	var history []commit
	c, err := getCommit(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	history = append(history, c)
	for c.Parent != "" {
		c, err = getCommit(c.Parent)
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
	if !dbExists(dbName) {
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
	if !dbExists(dbName) {
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
	if branchName == getDefaultBranchName(dbName) {
		w.WriteHeader(http.StatusConflict)
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
	commit := r.Request.Header.Get("commit")

	// Sanity check the inputs
	if dbName == "" || branchName == "" || commit == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: Validate the database and branch names

	// Ensure the requested database is in our system
	if !dbExists(dbName) {
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
	id, ok := branches[branchName]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// * Ensure the requested commit exists in the branch history *

	// If the head commit of the branch already matches the requested commit, there's nothing to change
	if id == commit {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// It didn't, so walk the branch history looking for the commit there
	commitExists := false
	c, err := getCommit(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	for c.Parent != "" {
		c, err = getCommit(c.Parent)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if c.ID == commit {
			// This commit in the branch history matches the requested commit, so we're good to proceed
			commitExists = true
		}
	}

	// The commit wasn't found, so don't update the branch
	if commitExists != true {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Update the branch
	branches[branchName] = commit
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
	if !dbExists(dbName) {
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
			if commitExists == true {
				continue
			}

			// Gather the details of the head commit
			c, err := getCommit(head)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// If the requested commit matches the branch head, retrieve the matching database ID
			var e dbTreeEntry
			var t dbTree
			if reqCommit == head {
				t, err = getTree(c.Tree)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				for _, e = range t.Entries {
					if e.Name == dbName {
						dbID = e.Sha256
						commitExists = true
					}
				}
			}

			// The requested commit wasn't the branch head, so we walk the branch history looking for it
			for c.Parent != "" {
				c, err = getCommit(c.Parent)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				if reqCommit == c.ID {
					// Found a match, so retrieve the database ID for the commit
					t, err = getTree(c.Tree)
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					for _, e = range t.Entries {
						if e.Name == dbName {
							dbID = e.Sha256
							commitExists = true
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
			branchName = getDefaultBranchName(dbName)
		}

		// Retrieve the commit ID for the branch
		commitID, ok := branches[branchName]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Retrieve the tree ID from the commit
		c, err := getCommit(commitID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		treeID := c.Tree

		// Retrieve the database ID from the tree
		t, err := getTree(treeID)
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
	dbName := r.Request.Header.Get("Name")
	branchName := r.Request.Header.Get("Branch")
	msg := r.Request.Header.Get("Message")
	modTime := r.Request.Header.Get("Modtime")

	// TODO: Validate the database and branch names

	// Sanity check the inputs
	if dbName == "" || msg == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// If no branch name was given, use the default for the database
	if branchName == "" {
		branchName = getDefaultBranchName(dbName)
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
	var c commit
	c.AuthorEmail = "justin@postgresql.org" // TODO: Author and Committer info should come from the client, so we
	c.AuthorName = "Justin Clift"           // TODO  hard code these for now.  Proper auth will need adding later
	c.Message = msg
	c.Timestamp = time.Now()
	c.Tree = t.ID

	// Check if the database already exists
	var branches map[string]string
	needDefBranch := false
	if dbExists(dbName) {
		// Load the existing branchHeads from disk
		branches, err = getBranches(dbName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Check if the desired branch already exists.  If it does, use its head commit as the parent for
		// our new uploads commit
		if id, ok := branches[branchName]; ok {
			c.Parent = id
		}
	} else {
		// No existing branches, so this will be the first
		branches = make(map[string]string)

		// We'll need to create the default branch value for the database later on too
		needDefBranch = true
	}

	// Update the branch with the commit for this new database upload
	c.ID = createCommitID(c)
	branches[branchName] = c.ID
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

	// Write the updated branch heads to disk
	err = storeBranches(dbName, branches)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write the default branch name to disk
	if needDefBranch {
		err = storeDefaultBranchName(dbName, branchName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
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
	commit := r.Request.Header.Get("commit")      // Required
	date := r.Request.Header.Get("date")          // Optional
	dbName := r.Request.Header.Get("database")    // Required
	tEmail := r.Request.Header.Get("taggeremail") // Only for annotated commits
	tName := r.Request.Header.Get("taggername")   // Only for annotated commits
	msg := r.Request.Header.Get("msg")            // Only for annotated commits
	tag := r.Request.Header.Get("tag")            // Required

	// Ensure at least the minimum inputs were provided
	if dbName == "" || tag == "" || commit == "" {
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
	if !dbExists(dbName) {
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
	for _, ID := range branches {
		// Because there seems to be no valid way of adding this condition in the loop declaration?
		if commitExists == true {
			continue
		}

		// Check if the head commit of the branch matches the requested commit
		if ID == commit {
			commitExists = true
			continue
		}

		// It didn't, so walk the commit history looking for the commit there
		c, err := getCommit(ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for c.Parent != "" {
			c, err = getCommit(c.Parent)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if c.ID == commit {
				// This commit in the history matches the requested commit, so we're good
				commitExists = true
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
			Commit:      commit,
			Date:        tDate,
			Message:     msg,
			TagType:     ANNOTATED,
			TaggerEmail: tEmail,
			TaggerName:  tName,
		}
	} else {
		// It's a simple commit
		t = tagEntry{
			Commit:      commit,
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
	if !dbExists(dbName) {
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
	if !dbExists(dbName) {
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
