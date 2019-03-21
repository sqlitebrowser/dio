package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	rq "github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	pushCmdBranch, pushCmdCommit, pushCmdDB  string
	pushCmdEmail, pushCmdLicence, pushCmdMsg string
	pushCmdName, pushCmdTimestamp            string
	pushCmdForce, pushCmdPublic              bool
)

// Uploads a database to DBHub.io.
var pushCmd = &cobra.Command{
	Use:   "push [database file]",
	Short: "Upload a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		return push(args)
	},
}

func init() {
	RootCmd.AddCommand(pushCmd)
	pushCmd.Flags().StringVar(&pushCmdName, "author", "", "Author name")
	pushCmd.Flags().StringVar(&pushCmdBranch, "branch", "",
		"Remote branch the database will be uploaded to")
	pushCmd.Flags().StringVar(&pushCmdCommit, "commit", "",
		"ID of the previous commit, for appending this new database to")
	pushCmd.Flags().StringVar(&pushCmdDB, "dbname", "", "Override for the database name")
	pushCmd.Flags().StringVar(&pushCmdEmail, "email", "", "Email address of the author")
	pushCmd.Flags().BoolVar(&pushCmdForce, "force", false, "Overwrite existing commit history?")
	pushCmd.Flags().StringVar(&pushCmdLicence, "licence", "",
		"The licence (ID) for the database, as per 'dio licence list'")
	pushCmd.Flags().StringVar(&pushCmdMsg, "message", "",
		"(Required) Commit message for this upload")
	pushCmd.Flags().BoolVar(&pushCmdPublic, "public", false, "Should the database be public?")
	pushCmd.Flags().StringVar(&pushCmdTimestamp, "timestamp", "", "Timestamp to use as the commit date")
}

func push(args []string) error {
	// Ensure a database file was given
	if len(args) == 0 {
		return errors.New("No database file specified")
	}
	// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
	// TODO  into a for loop
	if len(args) > 1 {
		return errors.New("Only one database can be uploaded at a time (for now)")
	}

	// Ensure the database file exists
	db := args[0]
	fi, err := os.Stat(db)
	if err != nil {
		return err
	}

	// Grab author name & email from the dio config file, but allow command line flags to override them
	var committerName, committerEmail, pushAuthor, pushEmail string
	u, ok := viper.Get("user.name").(string)
	if ok {
		pushAuthor = u
		committerName = u
	}
	v, ok := viper.Get("user.email").(string)
	if ok {
		pushEmail = v
		committerEmail = u
	}
	if pushCmdName != "" {
		pushAuthor = pushCmdName
	}
	if pushCmdEmail != "" {
		pushEmail = pushCmdEmail
	}

	// Author name and email are required
	if pushAuthor == "" || pushEmail == "" {
		return errors.New("Both author name and email are required!")
	}

	// Determine name to store database as
	if pushCmdDB == "" {
		pushCmdDB = filepath.Base(db)
	}

	// Check if there's local metadata.  If there is, we compare the local branch metadata with that on the server.
	// Then we go through a simple loop, uploading each outstanding commit to the remote server along with it's
	// metadata (via appropriate http headers)
	var meta metaData
	dbURL := fmt.Sprintf("%s/%s/%s", cloud, certUser, db)
	if _, err = os.Stat(filepath.Join(".dio", db, "metadata.json")); err == nil {
		// Load the local metadata cache, without retrieving updated metadata from the cloud
		meta, err = localFetchMetadata(db, false)
		if err != nil {
			return err
		}

		// If no branch name was given on the command line, we use the active branch
		if pushCmdBranch == "" {
			pushCmdBranch = meta.ActiveBranch
		}

		// Check the branch exists locally
		localHead, ok := meta.Branches[pushCmdBranch]
		if !ok {
			return errors.New(fmt.Sprintf("That branch ('%s') doesn't exist", pushCmdBranch))
		}

		// Build a list of the commits in the local branch
		localCommitList := []string{localHead.Commit}
		c, ok := meta.Commits[localHead.Commit]
		if ok == false {
			return errors.New("Something has gone wrong.  Head commit for the local branch isn't in the " +
				"local commit list")
		}
		for c.Parent != "" {
			c = meta.Commits[c.Parent]
			localCommitList = append(localCommitList, c.ID)
		}
		localCommitLength := len(localCommitList) - 1

		// Download the latest database metadata
		extraCtr := 0
		newMeta, found, err := retrieveMetadata(db)
		if err != nil {
			return err
		}
		if !found {
			// The database only exists locally, so we use the first commit to create the remote database,
			// then loop around pushing the remaining commits
			newCommit := meta.Commits[localCommitList[len(localCommitList)-1]].ID
			err = sendCommit(meta, db, dbURL, newCommit, pushCmdPublic)
			if err != nil {
				return err
			}

			// If there was only a single commit to push, there's nothing more to do
			if len(localCommitList) == 1 {
				_, err = fmt.Fprintf(fOut, "Database uploaded to %s\n\n", cloud)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(fOut, "  * Name: %s\n", pushCmdDB)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(fOut, "    Branch: %s\n", pushCmdBranch)
				if err != nil {
					return err
				}
				if pushCmdLicence != "" {
					_, err = fmt.Fprintf(fOut, "    Licence: %s\n", pushCmdLicence)
					if err != nil {
						return err
					}
				}
				_, err = numFormat.Fprintf(fOut, "    Size: %d bytes\n", fi.Size())
				if err != nil {
					return err
				}
				if pushCmdMsg != "" {
					_, err = fmt.Fprintf(fOut, "    Commit message: %s\n", pushCmdMsg)
					if err != nil {
						return err
					}
				}
				_, err = fmt.Fprintln(fOut)
				return err
			}

			// Let the user know the remote database has been created
			_, err = fmt.Fprintf(fOut, "Created new database '%s' on %s\n", db, cloud)
			if err != nil {
				return err
			}

			// Fetch the remote metadata, now that the database exists remotely.  This lets us use the existing
			// code below to add the remaining commits
			newMeta, found, err = retrieveMetadata(db)
			if err != nil {
				return err
			}
			extraCtr++
		}

		// * To get here, the database exists on the remote cloud and has local metadata *

		// Check the branch exists remotely
		remoteHead, ok := newMeta.Branches[pushCmdBranch]
		if !ok {
			// * The branch doesn't exist remotely, so create a fork on the remote cloud *

			// Determine which of the commits in the local branch is the first one not also on the server
			extraCtr++
			var baseBranchCounter int
			remoteBranchCommitCounter := make(map[string]int)
			for brName, brEntry := range newMeta.Branches {
				// Build a list of the commits in the remote branch
				remoteBranchCommitCounter[brName] = 0
				remoteCommitList := make(map[string]struct{})
				remoteCommitList[brEntry.Commit] = struct{}{}
				c, ok = newMeta.Commits[brEntry.Commit]
				if ok == false {
					return errors.New("Something has gone wrong.  Head commit for the remote branch " +
						"isn't in the remote commit list")
				}
				for c.Parent != "" {
					c = newMeta.Commits[c.Parent]
					remoteCommitList[c.ID] = struct{}{}
				}

				// At this point we have both a local and remote commit list, so we can now compare them and count
				// the # of matches for this branch
				for _, j := range localCommitList {
					if _, ok := remoteCommitList[j]; ok {
						remoteBranchCommitCounter[brName]++
					}
				}
			}

			// We take the highest number of known commits here, as that means the next commit in line is the first
			// unknown one on the remote cloud
			for _, j := range remoteBranchCommitCounter {
				if j > baseBranchCounter {
					baseBranchCounter = j
				}
			}

			// Create the new (forked) branch on DBHub.io
			newCommit := localCommitList[localCommitLength-baseBranchCounter]
			err = sendCommit(meta, db, dbURL, newCommit, pushCmdPublic)
			if err != nil {
				return err
			}

			// Count the number of commits in the new fork
			d := meta.Commits[newCommit]
			forkCommitCtr := 1
			for d.Parent != "" {
				d = meta.Commits[d.Parent]
				forkCommitCtr++
			}

			// Add the new (forked) branch to the local list of remote metadata
			newMeta.Branches[pushCmdBranch] = branchEntry{
				Commit:      newCommit,
				CommitCount: forkCommitCtr,
				Description: meta.Branches[pushCmdBranch].Description,
			}
			remoteHead = newMeta.Branches[pushCmdBranch]

			// Add the newly generated commit to the local list of remote metadata
			newMeta.Commits[newCommit] = meta.Commits[newCommit]

			// If this fork only had the one commit (eg no further commits to push), then finish here
			if len(localCommitList) == forkCommitCtr {
				_, err = fmt.Fprintf(fOut, "New branch '%s' created and all commits for it pushed to %s\n",
					pushCmdBranch, cloud)
				return err
			}

			// * Now that the initial commit for the new branch is on the remote server, we can continue on
			// "as per normal", using the existing code to loop around adding the remaining commits *
		}

		// Build a list of the commits in the remote branch
		remoteCommitList := []string{remoteHead.Commit}
		c, ok = newMeta.Commits[remoteHead.Commit]
		if ok == false {
			return errors.New("Something has gone wrong.  Head commit for the remote branch isn't in " +
				"the remote commit list")
		}
		for c.Parent != "" {
			c = newMeta.Commits[c.Parent]
			remoteCommitList = append(remoteCommitList, c.ID)
		}
		remoteCommitLength := len(remoteCommitList) - 1

		// Make sure the local and remote commits start out with the same commit ID
		if localCommitList[localCommitLength] != remoteCommitList[remoteCommitLength] {
			// The local and remote branches don't have a common root, so abort
			err = errors.New(fmt.Sprintf("Local and remote branch %s don't have a common root.  "+
				"Aborting.", pushCmdBranch))
			return err
		}

		// * Compare the local branch to the head of the remote branch, to determine which commits need sending *

		// If there are more commits in the remote branch than in the local one, then the branches have diverged
		// so abort (for now).
		// TODO: Write the code to allow --force overwriting for this
		if remoteCommitLength > localCommitLength {
			return fmt.Errorf("The remote branch has more commits than the local one.  Can't push the " +
				"branch.  If you want to overwrite changes on the remote server, consider the --force option.")
		}

		// Check if the given branch is the same on the local and remote server.  If it is, nothing needs to be done
		if remoteCommitLength == localCommitLength && remoteCommitList[0] == localCommitList[0] {
			return fmt.Errorf("The local and remote branch '%s' are identical.  Nothing to push.",
				pushCmdBranch)
		}

		// * To get here, the local branch has more commits than the remote one *

		// Create the list of commits that need pushing
		var pushCommits []string
		for i := 0; i <= localCommitLength; i++ {
			lCommit := localCommitList[localCommitLength-i]
			if i > remoteCommitLength {
				pushCommits = append(pushCommits, lCommit)
			} else {
				rCommit := remoteCommitList[remoteCommitLength-i]
				if lCommit != rCommit {
					// There are conflicting commits in this branch between the local metadata and the
					// remote.  Abort (for now)
					// TODO: Consider how to allow --force pushing here.  Also remember that when doing this, there
					//       needs a check added for potentially isolated tags and releases, same as branch revert
					e := fmt.Sprintf("The local and remote branch have conflicting commits.\n\n")
					e = fmt.Sprintf("%s  * local commit: %s\n", e, lCommit)
					e = fmt.Sprintf("%s  * remote commit: %s\n\n", e, rCommit)
					e = fmt.Sprintf("%sCan't push the branch.  If you want to overwrite changes on the "+
						"remote server, consider the --force option.", e)
					return errors.New(e)
				}
			}
		}

		// Display useful info message to the user
		numCommits := len(pushCommits) + extraCtr
		if numCommits == 1 {
			_, err = fmt.Fprintf(fOut, "Pushing 1 commit for branch '%s'", pushCmdBranch)
			if err != nil {
				return err
			}
		} else {
			_, err = fmt.Fprintf(fOut, "Pushing %d commit(s) for branch '%s'", numCommits, pushCmdBranch)
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(fOut, " to %s...\n", cloud)
		if err != nil {
			return err
		}

		// Send the commits to the cloud
		for _, commitID := range pushCommits {
			err = sendCommit(meta, db, dbURL, commitID, pushCmdPublic)
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintln(fOut, "All commits pushed.")
		return err
	}

	// To get here, we don't have existing metadata.  We just use the original file upload code, which creates the
	// database remotely (if it's not there already) and creates the local metadata.
	// If the database already exists remotely, this code will fail.
	// TODO: Maybe add a nicer failure message here for when local metadata is missing but the db exists remotely?
	z, ok := viper.Get("user.name").(string)
	if !ok {
		return fmt.Errorf("Committer name could not be determined")
	}
	committerName = z
	z, ok = viper.Get("user.email").(string)
	if !ok {
		return fmt.Errorf("Committer email could not be determined")
	}
	committerEmail = z

	b, err := ioutil.ReadFile(db)
	if err != nil {
		return err
	}
	s := sha256.Sum256(b)
	shaSum := hex.EncodeToString(s[:])
	req := rq.New().TLSClientConfig(&TLSConfig).Post(dbURL).
		Type("multipart").
		Query(fmt.Sprintf("authoremail=%s", url.QueryEscape(pushEmail))).
		Query(fmt.Sprintf("authorname=%s", url.QueryEscape(pushAuthor))).
		Query(fmt.Sprintf("branch=%s", url.QueryEscape(pushCmdBranch))).
		Query(fmt.Sprintf("commit=%s", pushCmdCommit)).
		Query(fmt.Sprintf("commitmsg=%s", url.QueryEscape(pushCmdMsg))).
		Query(fmt.Sprintf("committeremail=%s", url.QueryEscape(committerEmail))).
		Query(fmt.Sprintf("committername=%s", url.QueryEscape(committerName))).
		Query(fmt.Sprintf("committimestamp=%v", pushCmdTimestamp)).
		Query(fmt.Sprintf("dbshasum=%s", url.QueryEscape(shaSum))).
		Query(fmt.Sprintf("force=%v", pushCmdForce)).
		Query(fmt.Sprintf("lastmodified=%s", url.QueryEscape(fi.ModTime().UTC().Format(time.RFC3339)))).
		Query(fmt.Sprintf("public=%v", pushCmdPublic)).
		SendFile(db, "", "file1")
	if pushCmdLicence != "" {
		req.Query(fmt.Sprintf("licence=%s", url.QueryEscape(pushCmdLicence)))
	}
	resp, _, errs := req.End()
	if errs != nil {
		log.Print("Errors when uploading database to the cloud:")
		for _, err := range errs {
			_, _ = fmt.Fprint(fOut, err)
		}
		return errors.New("Error when uploading database to the cloud")
	}
	if resp != nil && resp.StatusCode != http.StatusCreated {
		return errors.New(fmt.Sprintf("Upload failed with an error: HTTP status %d - '%v'\n",
			resp.StatusCode, resp.Status))
	}

	// Retrieve updated metadata
	meta, _, err = retrieveMetadata(db)
	if err != nil {
		return err
	}
	meta.ActiveBranch = meta.DefBranch
	if pushCmdBranch == "" {
		pushCmdBranch = meta.ActiveBranch
	}

	// Save the updated metadata back to disk
	err = saveMetadata(db, meta)
	if err != nil {
		return err
	}

	// If the database isn't in the local metadata cache, then copy it there
	err = ioutil.WriteFile(filepath.Join(".dio", db, "db", shaSum), b, 0644)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(fOut, "Database uploaded to %s\n\n", cloud)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(fOut, "  * Name: %s\n", pushCmdDB)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(fOut, "    Branch: %s\n", pushCmdBranch)
	if err != nil {
		return err
	}
	if pushCmdLicence != "" {
		_, err = fmt.Fprintf(fOut, "    Licence: %s\n", pushCmdLicence)
		if err != nil {
			return err
		}
	}
	_, err = numFormat.Fprintf(fOut, "    Size: %d bytes\n", fi.Size())
	if err != nil {
		_, errInner := fmt.Fprintln(fOut)
		if errInner != nil {
			return fmt.Errorf("%s: %s", err, errInner)
		}
		return err
	}
	if pushCmdMsg != "" {
		_, err = fmt.Fprintf(fOut, "    Commit message: %s\n", pushCmdMsg)
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(fOut)
	return err
}

// Sends a commit to the cloud
func sendCommit(meta metaData, db string, dbURL string, newCommit string, public bool) (err error) {
	commitData, ok := meta.Commits[newCommit]
	if !ok {
		return fmt.Errorf("Something went wrong.  Could not retrieve data for commit '%s' from"+
			"local metadata commit list.", newCommit)
	}
	shaSum := commitData.Tree.Entries[0].Sha256
	var otherParents string
	for i, j := range commitData.OtherParents {
		if i != 1 {
			otherParents += ","
		}
		otherParents += j
	}

	// Push the first commit to the remote cloud, to create the database there
	req := rq.New().TLSClientConfig(&TLSConfig).Post(dbURL).
		Type("multipart").
		Query(fmt.Sprintf("branch=%s", url.QueryEscape(pushCmdBranch))).
		Query(fmt.Sprintf("commitmsg=%s", url.QueryEscape(commitData.Message))).
		Query(fmt.Sprintf("lastmodified=%s",
			url.QueryEscape(commitData.Tree.Entries[0].LastModified.UTC().Format(time.RFC3339)))).
		Query(fmt.Sprintf("commit=%s", commitData.Parent)).
		Query(fmt.Sprintf("authoremail=%s", url.QueryEscape(commitData.AuthorEmail))).
		Query(fmt.Sprintf("authorname=%s", url.QueryEscape(commitData.AuthorName))).
		Query(fmt.Sprintf("committeremail=%s", url.QueryEscape(commitData.CommitterEmail))).
		Query(fmt.Sprintf("committername=%s", url.QueryEscape(commitData.CommitterName))).
		Query(fmt.Sprintf("committimestamp=%s",
			url.QueryEscape(commitData.Timestamp.UTC().Format(time.RFC3339)))).
		Query(fmt.Sprintf("otherparents=%s", url.QueryEscape(otherParents))).
		Query(fmt.Sprintf("dbshasum=%s", url.QueryEscape(shaSum))).
		Query(fmt.Sprintf("public=%v", pushCmdPublic)).
		SendFile(filepath.Join(".dio", db, "db", shaSum), db, "file1")
	if pushCmdLicence != "" {
		req.Query(fmt.Sprintf("licence=%s", url.QueryEscape(pushCmdLicence)))
	}
	resp, body, errs := req.End()
	if errs != nil {
		e := fmt.Sprintln("Errors when uploading database to the cloud:")
		for _, err := range errs {
			e = err.Error()
		}
		return errors.New(e)
	}
	if resp != nil && resp.StatusCode != http.StatusCreated {
		return errors.New(fmt.Sprintf("Upload failed with an error: '%v'", body))
	}

	// Process the JSON format response data
	parsedResponse := map[string]string{}
	err = json.Unmarshal([]byte(body), &parsedResponse)
	if err != nil {
		_, errInner := fmt.Fprintf(fOut, "Error parsing server response: '%v'", err.Error())
		if errInner != nil {
			return fmt.Errorf("%s: %s", err, errInner)
		}
		return err
	}

	// Check that the ID for the new commit as generated by the server matches the ID generated locally
	remoteCommitID, ok := parsedResponse["commit_id"]
	if !ok {
		return errors.New("Unexpected response from server, doesn't contain new commit ID.")
	}
	if remoteCommitID != newCommit {
		return fmt.Errorf("Error.  The Commit ID generated on the server (%s) doesn't match the "+
			"local Commit ID (%s)", remoteCommitID, newCommit)
	}
	return
}
