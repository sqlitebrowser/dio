package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

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

		// Ensure we were given either a branch name or commit ID (and not both)
		if pullCmdBranch == "" && pullCmdCommit == "" {
			return errors.New("Either a branch name or commit ID must be given.")
		}
		if pullCmdBranch != "" && pullCmdCommit != "" {
			return errors.New("Either a branch name or commit ID must be given.  Not both!")
		}

		// Download the database file
		file := args[0]
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

		// Write the database file to disk
		err := ioutil.WriteFile(file, []byte(body), 0644)
		if err != nil {
			return err
		}
		if pullCmdBranch != "" {
			fmt.Printf("%s - Database '%s' downloaded.  Size: %d, branch: %s\n", cloud, file, len(body),
				pullCmdBranch)
		} else {
			fmt.Printf("%s - Database '%s' downloaded.  Size: %d, commit: %s\n", cloud, file, len(body),
				pullCmdCommit)
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
