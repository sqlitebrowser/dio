package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Removes a tag from a database
var tagRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a tag from a database",
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

		// Ensure a tag name was given
		if tag == "" {
			return errors.New("No tag name given")
		}

		// Remove the tag
		file := args[0]
		resp, _, errs := rq.New().Post(cloud+"/tag_remove").
			Set("tag", tag).
			Set("database", file).
			End()
		if errs != nil {
			log.Print("Errors when removing tag:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when removing tag")
		}
		if resp.StatusCode != http.StatusNoContent {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database or tag not found")
			}
			return errors.New(fmt.Sprintf("Tag removal failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Println("Tag remove succeeded")
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagRemoveCmd)
	tagRemoveCmd.Flags().StringVar(&tag, "tag", "", "Name of remote tag to remove")
}
