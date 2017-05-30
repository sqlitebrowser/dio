package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// TODO: Add support for annotated tags

// Creates a tag for a database
var tagCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a tag for a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a database file was given
		if len(args) == 0 {
			return errors.New("No database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one database can be changed at a time (for now)")
		}

		// Ensure a new tag name and commit ID were given
		if tag == "" {
			return errors.New("No tag name given")
		}
		if commit == "" {
			return errors.New("No commit ID given")
		}

		// Create the tag
		file := args[0]
		resp, _, errs := rq.New().Post(cloud+"/tag_create").
			Set("tag", tag).
			Set("commit", commit).
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when creating tag:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when creating tag")
		}
		if resp.StatusCode != http.StatusNoContent {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database or commit not found")
			}
			if resp.StatusCode == http.StatusConflict {
				return errors.New("Requested tag already exists")
			}
			return errors.New(fmt.Sprintf("Tag creation failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Println("Tag creation succeeded")
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagCreateCmd)
	tagCreateCmd.Flags().StringVar(&tag, "tag", "", "Name of remote tag to create")
	tagCreateCmd.Flags().StringVar(&commit, "commit", "", "Commit ID for the new tag")
}
