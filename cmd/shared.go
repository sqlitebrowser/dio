package cmd

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	rq "github.com/parnurzeal/gorequest"
)

// Check if the database with the given SHA256 checksum is in local cache.  If it's not then download and cache it
func checkDBCache(db, shaSum string) (err error) {
	if _, err = os.Stat(filepath.Join(".dio", db, "db", shaSum)); os.IsNotExist(err) {
		var body string
		_, body, err = retrieveDatabase(db, pullCmdBranch, pullCmdCommit)
		if err != nil {
			return
		}

		// Verify the SHA256 checksum of the new download
		s := sha256.Sum256([]byte(body))
		thisSum := hex.EncodeToString(s[:])
		if thisSum != shaSum {
			// The newly downloaded database file doesn't have the expected checksum.  Abort.
			return errors.New(fmt.Sprintf("Aborting: newly downloaded database file should have "+
				"checksum '%s', but data with checksum '%s' received\n", shaSum, thisSum))
		}

		// Write the database file to disk in the cache directory
		err = ioutil.WriteFile(filepath.Join(".dio", db, "db", shaSum), []byte(body), 0644)
	}
	return
}

// Generate a stable SHA256 for a commit.
func createCommitID(c commitEntry) string {
	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("tree %s\n", c.Tree.ID))
	if c.Parent != "" {
		b.WriteString(fmt.Sprintf("parent %s\n", c.Parent))
	}
	for _, j := range c.OtherParents {
		b.WriteString(fmt.Sprintf("parent %s\n", j))
	}
	b.WriteString(fmt.Sprintf("author %s <%s> %v\n", c.AuthorName, c.AuthorEmail,
		c.Timestamp.UTC().Format(time.UnixDate)))
	if c.CommitterEmail != "" {
		b.WriteString(fmt.Sprintf("committer %s <%s> %v\n", c.CommitterName, c.CommitterEmail,
			c.Timestamp.UTC().Format(time.UnixDate)))
	}
	b.WriteString("\n" + c.Message)
	b.WriteByte(0)
	s := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(s[:])
}

// Generate the SHA256 for a tree.
// Tree entry structure is:
// * [ entry type ] [ licence sha256] [ file sha256 ] [ file name ] [ last modified (timestamp) ] [ file size (bytes) ]
func createDBTreeID(entries []dbTreeEntry) string {
	var b bytes.Buffer
	for _, j := range entries {
		b.WriteString(string(j.EntryType))
		b.WriteByte(0)
		b.WriteString(string(j.LicenceSHA))
		b.WriteByte(0)
		b.WriteString(j.Sha256)
		b.WriteByte(0)
		b.WriteString(j.Name)
		b.WriteByte(0)
		b.WriteString(j.LastModified.Format(time.RFC3339))
		b.WriteByte(0)
		b.WriteString(fmt.Sprintf("%d\n", j.Size))
	}
	s := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(s[:])
}

// Returns true if a database has been changed on disk since the last commit
func dbChanged(db string, meta metaData) (changed bool, err error) {
	// Retrieve the sha256, file size, and last modified date from the head commit of the active branch
	head, ok := meta.Branches[meta.ActiveBranch]
	if !ok {
		err = errors.New("Aborting: info for the active branch isn't found in the local branch cache")
		return
	}
	c, ok := meta.Commits[head.Commit]
	if !ok {
		err = errors.New("Aborting: info for the head commit isn't found in the local commit cache")
		return
	}
	metaSHASum := c.Tree.Entries[0].Sha256
	metaFileSize := c.Tree.Entries[0].Size
	metaLastModified := c.Tree.Entries[0].LastModified.UTC()

	// If the file size or last modified date in the metadata are different from the current file info, then the
	// local file has probably changed.  Well, "probably" for the last modified day, but "definitely" if the file
	// size is different
	fi, err := os.Stat(db)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return
	}
	fileSize := int(fi.Size())
	lastModified := fi.ModTime().UTC()
	if metaFileSize != fileSize || metaLastModified != lastModified {
		changed = true
		return
	}

	// * If the file size and last modified date are still the same, we SHA256 checksum and compare the file *

	// TODO: Should we only do this for smaller files (below some TBD threshold)?

	// Read the database from disk, and calculate it's sha256
	b, err := ioutil.ReadFile(db)
	if err != nil {
		return
	}
	if len(b) != fileSize {
		err = errors.New(numFormat.Sprintf("Aborting: # of bytes read (%d) when reading the database "+
			"doesn't match the database file size (%d)", len(b), fileSize))
		return
	}
	s := sha256.Sum256(b)
	shaSum := hex.EncodeToString(s[:])

	// Check if a change has been made
	if metaSHASum != shaSum {
		changed = true
	}
	return
}

// Retrieves the list of databases available to the user
var getDatabases = func(url string, user string) (dbList []dbListEntry, err error) {
	resp, body, errs := rq.New().TLSClientConfig(&TLSConfig).Get(fmt.Sprintf("%s/%s", url, user)).End()
	if errs != nil {
		e := fmt.Sprintln("Errors when retrieving the database list:")
		for _, err := range errs {
			e += fmt.Sprintf(err.Error())
		}
		err = errors.New(e)
		return
	}
	defer resp.Body.Close()
	err = json.Unmarshal([]byte(body), &dbList)
	if err != nil {
		_, errInner := fmt.Fprintf(fOut, "Error retrieving database list: '%v'\n", err.Error())
		if errInner != nil {
			err = fmt.Errorf("%s: %s", err, errInner)
			return
		}
	}
	return
}

// Returns a map with the list of licences available on the remote server
var getLicences = func() (list map[string]licenceEntry, err error) {
	// Retrieve the database list from the cloud
	resp, body, errs := rq.New().TLSClientConfig(&TLSConfig).Get(cloud + "/licence/list").End()
	if errs != nil {
		e := fmt.Sprintln("errors when retrieving the licence list:")
		for _, err := range errs {
			e += fmt.Sprintf(err.Error())
		}
		return list, errors.New(e)
	}
	defer resp.Body.Close()

	// Convert the JSON response to our licence entry structure
	err = json.Unmarshal([]byte(body), &list)
	if err != nil {
		return list, errors.New(fmt.Sprintf("error retrieving licence list: '%v'\n", err.Error()))
	}
	return list, err
}

// getUserAndServer() returns the user name and server from a DBHub.io client certificate
func getUserAndServer() (userAcc string, certServer string, err error) {
	if numCerts := len(TLSConfig.Certificates); numCerts == 0 {
		err = errors.New("No client certificates installed.  Can't proceed.")
		return
	}

	// Parse the client certificate
	// TODO: Add support for multiple certificates
	cert, err := x509.ParseCertificate(TLSConfig.Certificates[0].Certificate[0])
	if err != nil {
		err = errors.New("Couldn't parse cert")
		return
	}

	// Extract the account name and associated server from the certificate
	cn := cert.Subject.CommonName
	if cn == "" {
		// The common name field is empty in the client cert.  Can't proceed.
		err = errors.New("Common name is blank in client certificate")
		return
	}
	s := strings.Split(cn, "@")
	if len(s) < 2 {
		err = errors.New("Missing information in client certificate")
		return
	}
	userAcc = s[0]
	certServer = s[1]
	if userAcc == "" || certServer == "" {
		// Missing details in common name field
		err = errors.New("Missing information in client certificate")
		return
	}

	return
}

// Loads the local metadata from disk (if present).  If not, then grab it from the remote server, storing it locally.
//     Note - This is subtly different than calling updateMetadata() itself.  This function
//     (loadMetadata()) is for use by commands which can use a local metadata cache all by itself
//     (eg branch creation), but only if it already exists.  For those, it only calls the
//     remote server when a local metadata cache doesn't exist.
func loadMetadata(db string) (meta metaData, err error) {
	// Check if the local metadata exists.  If not, pull it from the remote server
	if _, err = os.Stat(filepath.Join(".dio", db, "metadata.json")); os.IsNotExist(err) {
		_, err = updateMetadata(db, true)
		if err != nil {
			return
		}
	}

	// Read and parse the metadata
	var md []byte
	md, err = ioutil.ReadFile(filepath.Join(".dio", db, "metadata.json"))
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(md), &meta)

	// If the tag or release maps are missing, create initial empty ones.
	// This is a safety check, not sure if it's really needed
	if meta.Tags == nil {
		meta.Tags = make(map[string]tagEntry)
	}
	if meta.Releases == nil {
		meta.Releases = make(map[string]releaseEntry)
	}
	return
}

// Loads the local metadata cache for the requested database, if present.  Otherwise, (optionally) retrieve it from
// the server.
//   Note - this is suitable for use by read-only functions (eg: branch/tag list, log)
//   as it doesn't store or change any metadata on disk
var localFetchMetadata = func(db string, getRemote bool) (meta metaData, err error) {
	md, err := ioutil.ReadFile(filepath.Join(".dio", db, "metadata.json"))
	if err == nil {
		err = json.Unmarshal([]byte(md), &meta)
		return
	}

	// Can't read local metadata, and we're requested to not grab remote metadata.  So, nothing to do but exit
	if !getRemote {
		err = errors.New("No local metadata for the database exists")
		return
	}

	// Can't read local metadata, but we're ok to grab the remote. So, use that instead
	meta, _, err = retrieveMetadata(db)
	return
}

// Merges old and new metadata
func mergeMetadata(origMeta metaData, newMeta metaData) (mergedMeta metaData, err error) {
	mergedMeta.Branches = make(map[string]branchEntry)
	mergedMeta.Commits = make(map[string]commitEntry)
	mergedMeta.Tags = make(map[string]tagEntry)
	mergedMeta.Releases = make(map[string]releaseEntry)
	if len(origMeta.Commits) > 0 {
		// Start by check branches which exist locally
		// TODO: Change sort order to be by alphabetical branch name, as the current unordered approach leads to
		//       inconsistent output across runs
		for brName, brData := range origMeta.Branches {
			matchFound := false
			for newBranch, newData := range newMeta.Branches {
				if brName == newBranch {
					// A branch with this name exists on both the local and remote server
					matchFound = true
					skipFurtherChecks := false

					// Rewind back to the local root commit, making a list of the local commits IDs we pass through
					var localList []string
					localCommit := origMeta.Commits[brData.Commit]
					localList = append(localList, localCommit.ID)
					for localCommit.Parent != "" {
						localCommit = origMeta.Commits[localCommit.Parent]
						localList = append(localList, localCommit.ID)
					}
					localLength := len(localList) - 1

					// Rewind back to the remote root commit, making a list of the remote commit IDs we pass through
					var remoteList []string
					remoteCommit := newMeta.Commits[newData.Commit]
					remoteList = append(remoteList, remoteCommit.ID)
					for remoteCommit.Parent != "" {
						remoteCommit = newMeta.Commits[remoteCommit.Parent]
						remoteList = append(remoteList, remoteCommit.ID)
					}
					remoteLength := len(remoteList) - 1

					// Make sure the local and remote commits start out with the same commit ID
					if localCommit.ID != remoteCommit.ID {
						// The local and remote branches don't have a common root, so abort
						err = errors.New(fmt.Sprintf("Local and remote branch %s don't have a common root.  "+
							"Aborting.", brName))
						return
					}

					// If there are more commits in the local branch than in the remote one, we keep the local branch
					// as it probably means the user is adding stuff locally (prior to pushing to the server)
					if localLength > remoteLength {
						c := origMeta.Commits[brData.Commit]
						mergedMeta.Commits[c.ID] = origMeta.Commits[c.ID]
						for c.Parent != "" {
							c = origMeta.Commits[c.Parent]
							mergedMeta.Commits[c.ID] = origMeta.Commits[c.ID]
						}

						// Copy the local branch data
						mergedMeta.Branches[brName] = brData
					}

					// We've wound back to the root commit for both the local and remote branch, and the root commit
					// IDs match.  Now we walk forwards through the commits, comparing them.
					branchesSame := true
					for i := 0; i <= localLength; i++ {
						lCommit := localList[localLength-i]
						if i > remoteLength {
							branchesSame = false
						} else {
							if lCommit != remoteList[remoteLength-i] {
								// There are conflicting commits in this branch between the local metadata and the
								// remote.  This will probably need to be resolved by user action.
								branchesSame = false
							}
						}
					}

					// If the local branch commits are in the remote branch already, then we only need to check for
					// newer commits in the remote branch
					if branchesSame {
						if remoteLength > localLength {
							_, err = fmt.Fprintf(fOut, "  * Remote branch '%s' has %d new commit(s)... merged\n",
								brName, remoteLength-localLength)
							if err != nil {
								return
							}
							for _, j := range remoteList {
								mergedMeta.Commits[j] = newMeta.Commits[j]
							}
							mergedMeta.Branches[brName] = newMeta.Branches[brName]
						} else {
							// The local and remote branches are the same, so copy the local branch commits across to
							// the merged data structure
							_, err = fmt.Fprintf(fOut, "  * Branch '%s' is unchanged\n", brName)
							if err != nil {
								return
							}
							for _, j := range localList {
								mergedMeta.Commits[j] = origMeta.Commits[j]
							}
							mergedMeta.Branches[brName] = brData
						}
						// No need to do further checks on this branch
						skipFurtherChecks = true
					}

					if skipFurtherChecks == false && brData.Commit != newData.Commit {
						_, err = fmt.Fprintf(fOut, "  * Branch '%s' has local changes, not on the server\n",
							brName)
						if err != nil {
							return
						}

						// Copy across the commits from the local branch
						localCommit := origMeta.Commits[brData.Commit]
						mergedMeta.Commits[localCommit.ID] = origMeta.Commits[localCommit.ID]
						for localCommit.Parent != "" {
							localCommit = origMeta.Commits[localCommit.Parent]
							mergedMeta.Commits[localCommit.ID] = origMeta.Commits[localCommit.ID]
						}

						// Copy across the branch data entry for the local branch
						mergedMeta.Branches[brName] = brData
					}
					if skipFurtherChecks == false && brData.Description != newData.Description {
						_, err = fmt.Fprintf(fOut, "  * Description for branch %s differs between the local "+
							"and remote\n"+
							"    * Local: '%s'\n"+
							"    * Remote: '%s'\n", brName, brData.Description, newData.Description)
						if err != nil {
							return
						}
					}
				}
			}
			if !matchFound {
				// This seems to be a branch that's not on the server, so we keep it as-is
				_, err = fmt.Fprintf(fOut, "  * Branch '%s' is local only, not on the server\n", brName)
				if err != nil {
					return
				}
				mergedMeta.Branches[brName] = brData

				// Copy across the commits from the local branch
				localCommit := origMeta.Commits[brData.Commit]
				mergedMeta.Commits[localCommit.ID] = origMeta.Commits[localCommit.ID]
				for localCommit.Parent != "" {
					localCommit = origMeta.Commits[localCommit.Parent]
					mergedMeta.Commits[localCommit.ID] = origMeta.Commits[localCommit.ID]
				}

				// Copy across the branch data entry for the local branch
				mergedMeta.Branches[brName] = brData
			}
		}

		// Add new branches
		for remoteName, remoteData := range newMeta.Branches {
			if _, ok := origMeta.Branches[remoteName]; ok == false {
				// Copy their commit data
				newCommit := newMeta.Commits[remoteData.Commit]
				mergedMeta.Commits[newCommit.ID] = newMeta.Commits[newCommit.ID]
				for newCommit.Parent != "" {
					newCommit = newMeta.Commits[newCommit.Parent]
					mergedMeta.Commits[newCommit.ID] = newMeta.Commits[newCommit.ID]
				}

				// Copy their branch data
				mergedMeta.Branches[remoteName] = remoteData

				_, err = fmt.Fprintf(fOut, "  * New remote branch '%s' merged\n", remoteName)
				if err != nil {
					return
				}
			}
		}

		// Preserve existing tags
		for tagName, tagData := range origMeta.Tags {
			mergedMeta.Tags[tagName] = tagData
		}

		// Add new tags
		for tagName, tagData := range newMeta.Tags {
			// Only add tags which aren't already in the merged metadata structure
			if _, tagFound := mergedMeta.Tags[tagName]; tagFound == false {
				// Also make sure its commit is in the commit list.  If it's not, then skip adding the tag
				if _, commitFound := mergedMeta.Commits[tagData.Commit]; commitFound == true {
					_, err = fmt.Fprintf(fOut, "  * New tag '%s' merged\n", tagName)
					if err != nil {
						return
					}
					mergedMeta.Tags[tagName] = tagData
				}
			}
		}

		// Preserve existing releases
		for relName, relData := range origMeta.Releases {
			mergedMeta.Releases[relName] = relData
		}

		// Add new releases
		for relName, relData := range newMeta.Releases {
			// Only add releases which aren't already in the merged metadata structure
			if _, relFound := mergedMeta.Releases[relName]; relFound == false {
				// Also make sure its commit is in the commit list.  If it's not, then skip adding the release
				if _, commitFound := mergedMeta.Commits[relData.Commit]; commitFound == true {
					_, err = fmt.Fprintf(fOut, "  * New release '%s' merged\n", relName)
					if err != nil {
						return
					}
					mergedMeta.Releases[relName] = relData
				}
			}
		}

		// Copy the default branch name from the remote server
		mergedMeta.DefBranch = newMeta.DefBranch

		// If an active (local) branch has been set, then copy it to the merged metadata.  Otherwise use the default
		// branch as given by the remote server
		if origMeta.ActiveBranch != "" {
			mergedMeta.ActiveBranch = origMeta.ActiveBranch
		} else {
			mergedMeta.ActiveBranch = newMeta.DefBranch
		}

		_, err = fmt.Fprintln(fOut)
		if err != nil {
			return
		}
	} else {
		// No existing metadata, so just copy across the remote metadata
		mergedMeta = newMeta

		// Use the remote default branch as the initial active (local) branch
		mergedMeta.ActiveBranch = newMeta.DefBranch
	}
	return
}

// Retrieves a database from DBHub.io
func retrieveDatabase(db string, branch string, commit string) (resp rq.Response, body string, err error) {
	dbURL := fmt.Sprintf("%s/%s/%s", cloud, certUser, db)
	req := rq.New().TLSClientConfig(&TLSConfig).Get(dbURL)
	if branch != "" {
		req.Query(fmt.Sprintf("branch=%s", url.QueryEscape(branch)))
	} else {
		req.Query(fmt.Sprintf("commit=%s", url.QueryEscape(commit)))
	}
	var errs []error
	resp, body, errs = req.End()
	if errs != nil {
		log.Print("Errors when downloading database:")
		for _, err := range errs {
			log.Print(err.Error())
		}
		err = errors.New("Error when downloading database")
		return
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			if branch != "" {
				err = errors.New(fmt.Sprintf("That database & branch '%s' aren't known on DBHub.io",
					branch))
				return
			}
			if commit != "" {
				err = errors.New(fmt.Sprintf("Requested database not found with commit %s.",
					commit))
				return
			}
			err = errors.New("Requested database not found")
			return
		}
		err = errors.New(fmt.Sprintf("Download failed with an error: HTTP status %d - '%v'\n",
			resp.StatusCode, resp.Status))
	}
	return
}

// Retrieves database metadata from DBHub.io
var retrieveMetadata = func(db string) (meta metaData, onCloud bool, err error) {
	// Download the database metadata
	resp, md, errs := rq.New().TLSClientConfig(&TLSConfig).Get(cloud + "/metadata/get").
		Query(fmt.Sprintf("username=%s", url.QueryEscape(certUser))).
		Query(fmt.Sprintf("folder=%s", "/")).
		Query(fmt.Sprintf("dbname=%s", url.QueryEscape(db))).
		End()

	if errs != nil {
		log.Print("Errors when downloading database metadata:")
		for _, err := range errs {
			log.Print(err.Error())
		}
		return metaData{}, false, errors.New("Error when downloading database metadata")
	}
	if resp.StatusCode == http.StatusNotFound {
		return metaData{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return metaData{}, false,
			errors.New(fmt.Sprintf("Metadata download failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
	}
	err = json.Unmarshal([]byte(md), &meta)
	if err != nil {
		return
	}
	return meta, true, nil
}

// Saves the metadata to a local cache
func saveMetadata(db string, meta metaData) (err error) {
	// Create the metadata directory if needed
	if _, err = os.Stat(filepath.Join(".dio", db)); os.IsNotExist(err) {
		// We create the "db" directory instead, as that'll be needed anyway and MkdirAll() ensures the .dio/<db>
		// directory will be created on the way through
		err = os.MkdirAll(filepath.Join(".dio", db, "db"), 0770)
		if err != nil {
			return
		}
	}

	// Serialise the metadata to JSON
	var jsonString []byte
	jsonString, err = json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}

	// Write the updated metadata to disk
	mdFile := filepath.Join(".dio", db, "metadata.json")
	err = ioutil.WriteFile(mdFile, jsonString, 0644)
	return err
}

// Saves metadata to the local cache, merging in with any existing metadata
func updateMetadata(db string, saveMeta bool) (mergedMeta metaData, err error) {
	// Check for existing metadata file, loading it if present
	var md []byte
	origMeta := metaData{}
	md, err = ioutil.ReadFile(filepath.Join(".dio", db, "metadata.json"))
	if err == nil {
		err = json.Unmarshal([]byte(md), &origMeta)
		if err != nil {
			return
		}
	}

	// Download the latest database metadata
	_, err = fmt.Fprintln(fOut, "Updating metadata")
	if err != nil {
		return
	}
	newMeta, _, err := retrieveMetadata(db)
	if err != nil {
		return
	}

	// If we have existing local metadata, then merge the metadata from DBHub.io with it
	if len(origMeta.Commits) > 0 {
		mergedMeta, err = mergeMetadata(origMeta, newMeta)
		if err != nil {
			return
		}
	} else {
		// No existing metadata, so just copy across the remote metadata
		mergedMeta = newMeta

		// Use the remote default branch as the initial active (local) branch
		mergedMeta.ActiveBranch = newMeta.DefBranch
	}

	// Serialise the updated metadata to JSON
	var jsonString []byte
	jsonString, err = json.MarshalIndent(mergedMeta, "", "  ")
	if err != nil {
		errMsg := fmt.Sprintf("Error when JSON marshalling the merged metadata: %v\n", err)
		log.Print(errMsg)
		return
	}

	// If requested, write the updated metadata to disk
	if saveMeta {
		if _, err = os.Stat(filepath.Join(".dio", db)); os.IsNotExist(err) {
			err = os.MkdirAll(filepath.Join(".dio", db), 0770)
			if err != nil {
				return
			}
		}
		mdFile := filepath.Join(".dio", db, "metadata.json")
		err = ioutil.WriteFile(mdFile, []byte(jsonString), 0644)
	}
	return
}
