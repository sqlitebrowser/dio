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

// Generate a stable SHA256 for a commit.
func createCommitID(c commitEntry) string {
	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("tree %s\n", c.Tree.ID))
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
		b.WriteString(j.LastModified.Format(time.RFC3339))
		b.WriteByte(0)
		b.WriteString(fmt.Sprintf("%d\n", j.Size))
	}
	s := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(s[:])
}

// Returns a map with the list of licences available on the remote server
func getLicences() (list map[string]licenceEntry, err error) {
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
	return
}

// Loads the local metadata cache for the requested database, if present.  Otherwise, retrieve it from the server first.
//   Note that this suitable for use by read-only functions (eg: branch/tag list, log)
//   as it doesn't store or change any metadata on disk
func localFetchMetadata(db string) (meta metaData, err error) {
	var md []byte
	md, err = ioutil.ReadFile(filepath.Join(".dio", db, "metadata.json"))
	if err != nil {
		// No local cache, so retrieve the info from the server
		var temp string
		temp, err = retrieveMetadata(db)
		if err != nil {
			return
		}
		md = []byte(temp)
	}
	err = json.Unmarshal([]byte(md), &meta)
	return
}

// Merges old and new metadata
func mergeMetadata(origMeta metaData, newMeta metaData) (mergedMeta metaData, err error) {
	mergedMeta.Branches = make(map[string]branchEntry)
	mergedMeta.Commits = make(map[string]commitEntry)
	mergedMeta.Tags = make(map[string]tagEntry)
	if len(origMeta.Commits) > 0 {
		// Start by check branches which exist locally
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
							fmt.Printf("Remote list doesn't have element %d - commit '%s'\n", i, lCommit)
							branchesSame = false
						} else {
							if lCommit != remoteList[remoteLength-i] {
								// There are conflicting commits in this branch between the local metadata and the
								// remote.  This will probably need to be resolved by user action.
								// TODO: Figure out what user actions should be possible from here, to resolve the
								//       problem
								fmt.Printf("Commit %d differs - local: '%s', remote: '%s'\n", i, lCommit, remoteList[i])
								branchesSame = false
							}
						}
					}

					// If the local branch commits are in the remote branch already, then we only need to check for
					// newer commits in the remote branch
					if branchesSame {
						if remoteLength > localLength {
							fmt.Printf("  * Remote branch '%s' has %d new commit(s)... merged\n", brName,
								remoteLength-localLength)
							for _, j := range remoteList {
								mergedMeta.Commits[j] = newMeta.Commits[j]
							}
							mergedMeta.Branches[brName] = newMeta.Branches[brName]
						} else {
							// The local and remote branches are the same, so copy the local branch commits across to
							// the merged data structure
							fmt.Printf("  * Branch '%s' is unchanged\n", brName) // TODO: Probably don't need this line
							for _, j := range localList {
								mergedMeta.Commits[j] = origMeta.Commits[j]
							}
							mergedMeta.Branches[brName] = brData
						}
						// No need to do further checks on this branch
						skipFurtherChecks = true
					}

					if skipFurtherChecks == false && brData.Commit != newData.Commit {
						fmt.Printf("  * Head commit for branch %s differs between the local and remote\n"+
							"    * Local: %s\n"+
							"    * Remote: %s\n",
							brName, brData.Commit, newData.Commit)
					}
					if skipFurtherChecks == false && brData.Description != newData.Description {
						fmt.Printf("  * Description for branch %s differs between the local and remote\n"+
							"    * Local: '%s'\n"+
							"    * Remote: '%s'\n",
							brName, brData.Description, newData.Description)
					}
				}
			}
			if !matchFound {
				// This seems to be a branch that's not on the server, so we keep it as-is
				fmt.Printf("  * Branch %s is local only, not on the server\n", brName)
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

				fmt.Printf("  * New remote branch '%s' merged\n", remoteName)
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
				// Also make sure it's commit is in the commit list.  If it's not, then skip adding the tag
				if _, commitFound := mergedMeta.Commits[tagData.Commit]; commitFound == true {
					fmt.Printf("  * New tag '%s' merged\n", tagName)
					mergedMeta.Tags[tagName] = tagData
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

		fmt.Println()
	} else {
		// No existing metadata, so just copy across the remote metadata
		mergedMeta = newMeta

		// Use the remote default branch as the initial active (local) branch
		mergedMeta.ActiveBranch = newMeta.DefBranch
	}
	return
}

// Retrieves database metadata from DBHub.io
func retrieveMetadata(db string) (md string, err error) {
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
		return "", errors.New("Error when downloading database metadata")
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New(fmt.Sprintf("Metadata download failed with an error: HTTP status %d - '%v'\n",
			resp.StatusCode, resp.Status))
	}
	return md, nil
}

// Saves the metadata to a local cache
func saveMetadata(db string, meta metaData) (err error) {
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
	fmt.Println("Updating metadata")
	newMeta := metaData{}
	var tmp string
	tmp, err = retrieveMetadata(db)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(tmp), &newMeta)
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
