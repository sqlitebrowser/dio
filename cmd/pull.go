package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	rq "github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var pullCmdBranch, pullCmdCommit string

// Downloads a database from a DBHub.io cloud.
var pullCmd = &cobra.Command{
	Use:   "pull [database name]",
	Short: "Download a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a database file was given
		if len(args) == 0 {
			return errors.New("No database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one database can be downloaded at a time (for now)")
		}

		// Ensure we weren't given conflicting info on what to pull down
		if pullCmdBranch != "" && pullCmdCommit != "" {
			return errors.New("Either a branch name or commit ID can be given.  Not both at the same time!")
		}

		// If neither a branch nor commit ID were given, use the head commit of the default branch
		file := args[0]
		if pullCmdBranch == "" && pullCmdCommit == "" {
			var errs []error
			var resp rq.Response
			resp, pullCmdBranch, errs = rq.New().Get(cloud+"/branch_default_get").
				Set("database", file).
				End()
			if errs != nil {
				return errors.New("Could not determine default branch for database")
			}
			if resp.StatusCode != http.StatusOK {
				if resp.StatusCode == http.StatusNotFound {
					return errors.New("Requested database not found")
				}
				return errors.New(fmt.Sprintf(
					"Retrieving default branch failed with an error: HTTP status %d - '%v'\n",
					resp.StatusCode, resp.Status))
			}
		}

		// Download the database file
		req := rq.New().Get(cloud+"/db_download").Set("database", file)
		if pullCmdBranch != "" {
			req.Set("branch", pullCmdBranch)
		} else {
			req.Set("commit", pullCmdCommit)
		}
		resp, body, errs := req.End()
		if errs != nil {
			log.Print("Errors when downloading database:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when downloading database")
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				if pullCmdCommit != "" {
					return errors.New(fmt.Sprintf("Requested database not found with commit %s.",
						pullCmdCommit))
				}
				return errors.New("Requested database not found")
			}
			return errors.New(fmt.Sprintf("Download failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		// Extract the database and licence from the API servers' response
		var dbAndLicence struct {
			DBFile       []byte
			LastModified time.Time
			LicName      string
			LicText      []byte
		}
		err := json.Unmarshal([]byte(body), &dbAndLicence)
		if err != nil {
			return err
		}

		// Write the database file to disk
		err = ioutil.WriteFile(file, dbAndLicence.DBFile, 0644)
		if err != nil {
			return err
		}
		err = os.Chtimes(file, time.Now(), dbAndLicence.LastModified)
		if err != nil {
			return err
		}
		if pullCmdBranch != "" {
			fmt.Printf("Database '%s' downloaded from %s.  Branch: '%s'.  Size: %d bytes\n", file,
				cloud, pullCmdBranch, len(dbAndLicence.DBFile))
		} else {
			fmt.Printf("Database '%s' downloaded from %s.  Size: %d bytes\nCommit: %s\n", file,
				cloud, len(dbAndLicence.DBFile), pullCmdCommit)
		}

		// If a licence was returned along with the database, write it to disk as well
		if len(dbAndLicence.LicText) > 0 {
			licFile := file + "-LICENCE"
			err = ioutil.WriteFile(licFile, dbAndLicence.LicText, 0644)
			if err != nil {
				return err
			}
			err = os.Chtimes(licFile, time.Now(), dbAndLicence.LastModified)
			if err != nil {
				return err
			}
			fmt.Printf("This database is using the %s licence.  A copy has been created as %s.\n",
				dbAndLicence.LicName, licFile)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(pullCmd)
	pullCmd.Flags().StringVar(&pullCmdBranch, "branch", "",
		"Remote branch the database will be downloaded from")
	pullCmd.Flags().StringVar(&pullCmdCommit, "commit", "", "Commit ID of the database to download")
}
