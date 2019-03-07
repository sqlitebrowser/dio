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
		u, ok := viper.Get("author").(string)
		if ok {
			pushAuthor = u
		}
		v, ok := viper.Get("email").(string)
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

		// TODO: If a local metadata cache doesn't exist, and neither does the database on the remote server (yet),
		//       then this is a brand new database needing creation on the DBHub.io cloud
		var meta metaData
		if _, err = os.Stat(filepath.Join(".dio", db, "metadata.json")); os.IsNotExist(err) {
			// At the moment, since there's no better way to check for the existence of a remote database, we just
			// grab the list of the users databases and check against that
			dbList, err := getDatabases(cloud, certUser)
			if err != nil {
				return err
			}
			for _, j := range dbList {
				if db == j.Name {
					// This database already exists on DBHub.io.  We need local metadata in order to proceed, but don't
					// yet have it.  Safest option, at least for now, is to tell the user and abort
					return errors.New("Aborting: the database exists on the remote server, but has no " +
						"local metadata cache.  Please retrieve the remote metadata, then run the push command again")
				}
			}

			// To get here, the database doesn't already exist on DBHub.io, so we just use the original file upload
			// code
			dbURL := fmt.Sprintf("%s/%s/%s", cloud, certUser, db)
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

			// TODO: Create an initial set of metadata, incorporating the saved response.  Might need to specifically
			//       call the DBHub.io cloud for updated metadata first (unsure yet)

		} else {
			// * The database has a local metadata cache.  We'll need to use the new upload mechanism for this *

			// Load the local metadata cache, without retrieving updated metadata from the cloud
			meta, err = localFetchMetadata(db, false)
			if err != nil {
				return err
			}

			// TODO: Write the new upload mechanism ;)

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
	pushCmd.Flags().StringVar(&pushCmdBranch, "branch", "master",
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
