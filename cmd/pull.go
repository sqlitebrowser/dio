package cmd

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
		file := args[0]
		dbURL := fmt.Sprintf("%s/%s/%s", cloud, certUser, file)
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
		err := ioutil.WriteFile(file, []byte(body), 0644)
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
					err = os.Chtimes(file, time.Now(), lastMod)
					if err != nil {
						return err
					}
				}
			}
		}

		// TODO: * Download the metadata for the database, and save it in a subdirectory of the local .dio directory *

		// We store metadata for all databases in a ".dio" directory in the current directory.  Each downloaded database
		// has it's metadata stored in a folder (named the same as the database) in this directory.

		// Check if a folder in the .dio directory exists with the same name as the database just downloaded
		if _, err = os.Stat(filepath.Join(".dio", file)); os.IsNotExist(err) {
			// If not, then create it
			err = os.MkdirAll(filepath.Join(".dio", file), 0770)
			if err != nil {
				return err
			}
		}

		// TODO: Check if the database metadata (metadata.json) file already exists in the subdirectory
			// If it does, we'll probably need to figure out some way to merge things, so it doesn't muck up any
			// locally created branches (should we support those? probably yes)

		// TODO: Update the push code to use this newly saved metadata

		// TODO: We'll probably need to write (on the server) decently in depth validation code for the commit data,
		//       for when a client sends (--force enabled) new metadata to the server.  Otherwise, if the commit
		//       history is badly formed it could lead to bad crap happening.

		// Download the database metadata
		var mdBody string
		resp, mdBody, errs = rq.New().TLSClientConfig(&TLSConfig).Get(cloud + "/metadata/get").
			Query(fmt.Sprintf("username=%s", url.QueryEscape(certUser))).
			Query(fmt.Sprintf("folder=%s", "/")).
			Query(fmt.Sprintf("dbname=%s", url.QueryEscape(file))).
			End()

		if errs != nil {
			log.Print("Errors when downloading database metadata:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when downloading database metadata")
		}
		if resp.StatusCode != http.StatusOK {
			return errors.New(fmt.Sprintf("Metadata download failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		// Write the metadata file to disk
		mdFile := filepath.Join(".dio", file, "metadata.json")
		err = ioutil.WriteFile(mdFile, []byte(mdBody), 0644)
		if err != nil {
			return err
		}

		fmt.Printf("Database '%s' downloaded.  Size: %d bytes\n", file, len(body))

		//if pullCmdBranch != "" {
		//	fmt.Printf("Database '%s' downloaded from %s.  Branch: '%s'.  Size: %d bytes\n", file,
		//		cloud, pullCmdBranch, len(dbAndLicence.DBFile))
		//} else {
		//	fmt.Printf("Database '%s' downloaded from %s.  Size: %d bytes\nCommit: %s\n", file,
		//		cloud, len(dbAndLicence.DBFile), pullCmdCommit)
		//}
		//
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
