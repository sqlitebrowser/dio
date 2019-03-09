package cmd

import (
	"encoding/json"
	"fmt"
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
	pushCmdName                              string
	pushCmdForce, pushCmdPublic              bool
)

// Uploads a database to DBHub.io.
var pushCmd = &cobra.Command{
	Use:   "push [database file]",
	Short: "Upload a database",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		var pushAuthor, pushEmail string
		u, ok := viper.Get("user.name").(string)
		if ok {
			pushAuthor = u
		}
		v, ok := viper.Get("user.email").(string)
		if ok {
			pushEmail = v
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

		// Ensure commit message has been provided
		if pushCmdMsg == "" {
			return errors.New("Commit message is required!")
		}

		// Determine name to store database as
		if pushCmdDB == "" {
			pushCmdDB = filepath.Base(db)
		}

		// TODO: New approach.  Check if there's local metadata.  If there is, we compare the local branch metadata
		//       with that on the server.  Then we go through a simple loop, uploading each outstanding commit to the
		//       remote server along with it's metadata (in appropriate headers)
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

			// Download the latest database metadata
			newMeta := metaData{}
			var tmp string
			var found bool
			tmp, found, err = retrieveMetadata(db)
			if err != nil {
				return err
			}
			if !found {
				// The database only exists locally, so we'll need to start with the first commit for the branch onwards
				// TODO: Write the code for this
				fmt.Printf("TBD: Database only exists locally, and isn't yet present on DBHub.io")

				// TODO: Make a list of the commits in the local branch

				// TODO: Copy the commit metadata from the first commit into appropriate headers for the upload

				// TODO: Do the upload

				// TODO: Make a loop, processing the remaining commits


				return nil
			}
			err = json.Unmarshal([]byte(tmp), &newMeta)
			if err != nil {
				return err
			}

			// TODO: To get here, the database has local metadata and a remote database
			// TODO: Write the code for this
			//fmt.Printf("TBD: Database exists both locally and on DBHub.io\n")


			// Check the branch exists remotely
			remoteHead, ok := newMeta.Branches[pushCmdBranch]
			if !ok {
				// TODO: Write the code to handle pushing new branches
				return errors.New(fmt.Sprintf("That branch ('%s') doesn't exist on DBHub.io", pushCmdBranch))
			}

			// Build a list of the commits in the local branch
			localCommitList := []string{localHead.Commit}
			c, ok := meta.Commits[localHead.Commit]
			if ok == false {
				return errors.New("Something has gone wrong.  Head commit for the local branch isn't in the "+
					"local commit list")
			}
			for c.Parent != "" {
				c = meta.Commits[c.Parent]
				localCommitList = append(localCommitList, c.ID)
			}
			localCommitLength := len(localCommitList) - 1

			// Build a list of the commits in the remote branch
			remoteCommitList := []string{remoteHead.Commit}
			c, ok = newMeta.Commits[remoteHead.Commit]
			if ok == false {
				return errors.New("Something has gone wrong.  Head commit for the remote branch isn't in the "+
					"remote commit list")
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

			// TODO: Compare the local branch to the head of the remote branch, to determine which commits need sending

			// If there are more commits in the remote branch than in the local one, then the branches have diverged
			// so abort (for now).
			// TODO: Write the code to allow --force overwriting for this
			if remoteCommitLength > localCommitLength {
				return fmt.Errorf("The remote branch has more commits than the local one.  Can't push the " +
					"branch.  If you want to overwrite changes on the remote server, consider the --force option\n")
			}

			// Check if the given branch is the same on the local and remote server.  If it is, nothing needs to be done
			if remoteCommitLength == localCommitLength && remoteCommitList[0] == localCommitList[0] {
				return fmt.Errorf("The local and remote branch '%s' are identical.  Push not needed\n",
					pushCmdBranch)
			}

			// * To get here, the local branch has more commits than the remote one *

			// Create the list of commits that need pushing
			var pushCommits []string
			for i := 0; i <= localCommitLength; i++ {
				lCommit := localCommitList[localCommitLength-i]
				if i > remoteCommitLength {
					fmt.Printf("Commit aded to push list '%s'\n", lCommit) // TODO: This can probably be removed
					pushCommits = append(pushCommits, lCommit)
				} else {
					rCommit := remoteCommitList[remoteCommitLength-i]
					if lCommit != rCommit {
						// There are conflicting commits in this branch between the local metadata and the
						// remote.  Abort (for now)
						// TODO: Consider how to allow --force pushing here
						e := fmt.Sprintf("The local and remote branch have conflicting commits.\n\n")
						e = fmt.Sprintf("%s  * local commit: %s\n", e, lCommit)
						e = fmt.Sprintf("%s  * remote commit: %s\n\n", e, rCommit)
						e = fmt.Sprintf("%sCan't push the branch.  If you want to overwrite changes on the " +
							"remote server, consider the --force option\n", e)
						return errors.New(e)
					}
				}
			}

			fmt.Printf("Number of commits to push: %v\n", len(pushCommits))

			// Send the commits
			for _, commitID := range pushCommits {
				commitData, ok := meta.Commits[commitID]
				if !ok {
					return fmt.Errorf("Something went wrong.  Could not retrieve data for commit '%s' from" +
						"local metadata commit list\n", commitID)
				}

				fmt.Printf("Pushing commit '%s'\n", commitID)

				shaSum := commitData.Tree.Entries[0].Sha256
				req := rq.New().TLSClientConfig(&TLSConfig).Post(dbURL).
					Type("multipart").
					Query(fmt.Sprintf("branch=%s", url.QueryEscape(pushCmdBranch))).
					Query(fmt.Sprintf("commitmsg=%s", url.QueryEscape(commitData.Message))).
					Query(fmt.Sprintf("lastmodified=%s",
						url.QueryEscape(commitData.Timestamp.Format(time.RFC3339)))).
					Query(fmt.Sprintf("commit=%s", commitData.Parent)).
					// TODO: Add the remaining required data fields.  Likely need to update the server side to add
					//       anything missing
					//Query(fmt.Sprintf("public=%v", pushCmdPublic)).
					//Query(fmt.Sprintf("force=%v", pushCmdForce)).
					SendFile(filepath.Join(".dio", db, "db", shaSum, "", "file1"))
				if pushCmdLicence != "" {
					req.Query(fmt.Sprintf("licence=%s", url.QueryEscape(pushCmdLicence)))
				}
				resp, body, errs := req.End()
				if errs != nil {
					log.Print("Errors when uploading database to the cloud:")
					for _, err := range errs {
						log.Print(err.Error())
					}
					return errors.New("Error when uploading database to the cloud")
				}
				if resp != nil && resp.StatusCode != http.StatusCreated {
					return errors.New(fmt.Sprintf("Upload failed with an error: HTTP status %d - '%v'\n",
						resp.StatusCode, resp.Status))
				}

				// Process the JSON format response data
				parsedResponse := map[string]string{}
				err = json.Unmarshal([]byte(body), &parsedResponse)
				if err != nil {
					fmt.Printf("Error parsing server response: '%v'\n", err.Error())
					return err
				}
			}

			// TODO: Copy the commit metadata from the first commit into appropriate headers for the upload

			// TODO: Do the upload

			// TODO: Make a loop, processing the remaining commits

			return nil


		}

		// To get here, the database doesn't already exist on DBHub.io, so we just use the original file upload
		// code
		req := rq.New().TLSClientConfig(&TLSConfig).Post(dbURL).
			Type("multipart").
			Query(fmt.Sprintf("branch=%s", url.QueryEscape(pushCmdBranch))).
			Query(fmt.Sprintf("commitmsg=%s", url.QueryEscape(pushCmdMsg))).
			Query(fmt.Sprintf("lastmodified=%s", url.QueryEscape(fi.ModTime().Format(time.RFC3339)))).
			Query(fmt.Sprintf("commit=%s", pushCmdCommit)).
			Query(fmt.Sprintf("public=%v", pushCmdPublic)).
			Query(fmt.Sprintf("force=%v", pushCmdForce)).
			SendFile(db, "", "file1")
		if pushCmdLicence != "" {
			req.Query(fmt.Sprintf("licence=%s", url.QueryEscape(pushCmdLicence)))
		}
		resp, body, errs := req.End()
		if errs != nil {
			log.Print("Errors when uploading database to the cloud:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when uploading database to the cloud")
		}
		if resp != nil && resp.StatusCode != http.StatusCreated {
			return errors.New(fmt.Sprintf("Upload failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		// Process the JSON format response data
		parsedResponse := map[string]string{}
		err = json.Unmarshal([]byte(body), &parsedResponse)
		if err != nil {
			fmt.Printf("Error parsing server response: '%v'\n", err.Error())
			return err
		}

		// Save the updated metadata back to disk
		err = saveMetadata(db, meta)
		if err != nil {
			return err
		}

		fmt.Printf("Database uploaded to %s\n\n", cloud)
		fmt.Printf("  * Name: %s\n", pushCmdDB)
		fmt.Printf("    Branch: %s\n", pushCmdBranch)
		if pushCmdLicence != "" {
			fmt.Printf("    Licence: %s\n", pushCmdLicence)
		}
		fmt.Printf("    Size: %d bytes\n", fi.Size())
		fmt.Printf("    Commit message: %s\n\n", pushCmdMsg)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(pushCmd)
	//pushCmd.Flags().StringVar(&pushCmdName, "author", "", "Author name")
	pushCmd.Flags().StringVar(&pushCmdBranch, "branch", "",
		"Remote branch the database will be uploaded to")
	pushCmd.Flags().StringVar(&pushCmdCommit, "commit", "",
		"ID of the previous commit, for appending this new database to")
	//pushCmd.Flags().StringVar(&pushCmdDB, "dbname", "", "Override for the database name")
	//pushCmd.Flags().StringVar(&pushCmdEmail, "email", "", "Email address of the author")
	pushCmd.Flags().BoolVar(&pushCmdForce, "force", false, "Overwrite existing commit history?")
	pushCmd.Flags().StringVar(&pushCmdLicence, "licence", "",
		"The licence (ID) for the database, as per 'dio licence list'")
	pushCmd.Flags().StringVar(&pushCmdMsg, "message", "",
		"(Required) Commit message for this upload")
	pushCmd.Flags().BoolVar(&pushCmdPublic, "public", false, "Should the database be public?")
}
