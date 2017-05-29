package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	rq "github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var branch, name string

// Uploads a database to a DBHub.io cloud.
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
		file := args[0]
		fi, err := os.Stat(file)
		if err != nil {
			return err
		}

		// Determine name to store database as
		if name == "" {
			name = filepath.Base(file)
		}

		// Send the file
		resp, _, errs := rq.New().Post(cloud+"/db_upload").
			Type("multipart").
			Set("Branch", branch).
			Set("ModTime", fi.ModTime().Format(time.RFC3339)).
			Set("Name", name).
			SendFile(file).
			End()
		if errs != nil {
			log.Print("Errors when uploading database to the cloud:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when uploading database to the cloud")
		}

		if resp != nil && resp.StatusCode != 201 {
			return errors.New(fmt.Sprintf("Upload failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}
		fmt.Printf("%s - Database upload successful.  Name: %s, size: %d, branch: %s\n", cloud, name,
			fi.Size(), branch)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(pushCmd)
	pushCmd.Flags().StringVar(&branch, "branch", "master", "Remote branch the database will be uploaded to")
	pushCmd.Flags().StringVar(&name, "name", "", "Override for the database name")
}
