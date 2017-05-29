package cmd

import (
	"fmt"
	"io/ioutil"
	"log"

	rq "github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Downloads a database from a DBHub.io cloud.
var pullCmd = &cobra.Command{
	Use:   "pull [database file]",
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

		// Download the database file
		file := args[0]
		resp, body, errs := rq.New().Get(cloud+"/db_download").
			Set("branch", branch).
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when downloading database:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when downloading database")
		}
		if resp.StatusCode != 200 {
			if resp.StatusCode == 404 {
				return errors.New("Requested database not found")
			}
			return errors.New(fmt.Sprintf("Download failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		// Write the file to disk
		err := ioutil.WriteFile(file, []byte(body), 0644)
		if err != nil {
			return err
		}
		fmt.Printf("%s - Database download successful.  Name: %s, size: %d, branch: %s\n", cloud,
			file, len(body), branch)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(pullCmd)
	pullCmd.Flags().StringVar(&branch, "branch", "master",
		"Remote branch the database will be downloaded from")
}
