package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	rq "github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var pullCmdBranch, pullCmdCommit string

// Downloads a database from DBHub.io.
var pullCmd = &cobra.Command{
	Use:   "pull [database name]",
	Short: "Download a database from DBHub.io",
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

		// Ensure we weren't given potentially conflicting info on what to pull down
		if pullCmdBranch != "" && pullCmdCommit != "" {
			return errors.New("Either a branch name or commit ID can be given.  Not both at the same time!")
		}

		//// If neither a branch nor commit ID were given, use the head commit of the default branch
		//if pullCmdBranch == "" && pullCmdCommit == "" {
		//	var errs []error
		//	var resp rq.Response
		//	resp, pullCmdBranch, errs = rq.New().Get(cloud+"/branch_default_get").
		//		Set("database", file).
		//		End()
		//	if errs != nil {
		//		return errors.New("Could not determine default branch for database")
		//	}
		//	if resp.StatusCode != http.StatusOK {
		//		if resp.StatusCode == http.StatusNotFound {
		//			return errors.New("Requested database not found")
		//		}
		//		return errors.New(fmt.Sprintf(
		//			"Retrieving default branch failed with an error: HTTP status %d - '%v'\n",
		//			resp.StatusCode, resp.Status))
		//	}
		//}

		// Download the database file
		db := args[0]
		dbURL := fmt.Sprintf("%s/%s/%s", cloud, certUser, db)
		req := rq.New().TLSClientConfig(&TLSConfig).Get(dbURL)
		//if pullCmdBranch != "" {
		//	req.Set("branch", pullCmdBranch)
		//} else {
		//	req.Set("commit", pullCmdCommit)
		//}
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

		// Write the database file to disk
		err := ioutil.WriteFile(db, []byte(body), 0644)
		if err != nil {
			return err
		}

		// TODO: It'd probably be useful for the DBHub.io server to include the licence info in the headers, so a
		//       follow up request can grab the licence too.  Maybe even add a --licence option or similar to the
		//       pull command, for automatically grabbing the licence as well?

		// If the headers included the modification-date parameter for the database, set the last accessed and last
		// modified times on the new database file
		if disp := resp.Header.Get("Content-Disposition"); disp != "" {
			s := strings.Split(disp, ";")
			if len(s) == 4 {
				a := strings.TrimLeft(s[2], " ")
				if strings.HasPrefix(a, "modification-date=") {
					b := strings.Split(a, "=")
					c := strings.Trim(b[1], "\"")
					lastMod, err := time.Parse(time.RFC3339, c)
					if err != nil {
						return err
					}
					err = os.Chtimes(db, time.Now(), lastMod)
					if err != nil {
						return err
					}
				}
			}
		}

		// Update the local metadata cache
		var meta metaData
		meta, err = updateMetadata(db)
		if err != nil {
			return err
		}

		// If the server provided a branch name, add it to the local metadata cache
		if branch := resp.Header.Get("Branch"); branch != "" {
			meta.ActiveBranch = branch
		}

		_, err = numFormat.Printf("Database '%s' downloaded.  Size: %d bytes\n", db, len(body))
		if err != nil {
			return err
		}

		//if pullCmdBranch != "" {
		//	fmt.Printf("Database '%s' downloaded from %s.  Branch: '%s'.  Size: %d bytes\n", file,
		//		cloud, pullCmdBranch, len(dbAndLicence.DBFile))
		//} else {
		//	fmt.Printf("Database '%s' downloaded from %s.  Size: %d bytes\nCommit: %s\n", file,
		//		cloud, len(dbAndLicence.DBFile), pullCmdCommit)
		//}

		//// If a licence was returned along with the database, write it to disk as well
		//if len(dbAndLicence.LicText) > 0 {
		//	licFile := file + "-LICENCE"
		//	err = ioutil.WriteFile(licFile, dbAndLicence.LicText, 0644)
		//	if err != nil {
		//		return err
		//	}
		//	err = os.Chtimes(licFile, time.Now(), dbAndLicence.LastModified)
		//	if err != nil {
		//		return err
		//	}
		//	fmt.Printf("This database is using the %s licence.  A copy has been created as %s.\n",
		//		dbAndLicence.LicName, licFile)
		//}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(pullCmd)
	pullCmd.Flags().StringVar(&pullCmdBranch, "branch", "",
		"Remote branch the database will be downloaded from")
	pullCmd.Flags().StringVar(&pullCmdCommit, "commit", "", "Commit ID of the database to download")
}
