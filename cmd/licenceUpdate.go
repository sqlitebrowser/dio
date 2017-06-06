package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

var licenceUpdateName, licenceUpdateURL string

// Updates the details of a known licence.
var licenceUpdateCmd = &cobra.Command{
	Use:   "update [licence name]",
	Short: "Update the details of a known licence",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a licence name was given
		if len(args) == 0 {
			return errors.New("No licence name specified")
		}
		if len(args) > 1 {
			return errors.New("Only one licence can be updated at a time")
		}

		// Sanity check
		if licenceUpdateName == "" && licenceUpdateURL == "" {
			return errors.New("Missing info for what needs updating (name?, URL?)")
		}

		// Update the licence
		lic := args[0]
		req := rq.New().Post(cloud+"/licence_update").
			Set("name", lic)
		if licenceUpdateName != "" {
			req.Set("newname", licenceUpdateName)
		}
		if licenceUpdateURL != "" {
			req.Set("source", licenceUpdateURL)
		}
		resp, _, errs := req.End()
		if errs != nil {
			log.Print("Errors when updating the licence:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when updating the licence")
		}
		if resp.StatusCode != http.StatusNoContent {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested licence")
			}
			return errors.New(fmt.Sprintf("Licence update failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		// Inform the user
		fmt.Println("Licence updated")
		return nil
	},
}

func init() {
	licenceCmd.AddCommand(licenceUpdateCmd)
	licenceUpdateCmd.Flags().StringVar(&licenceUpdateName, "new-name", "",
		"The new friendly name to use for the licence")
	licenceUpdateCmd.Flags().StringVar(&licenceUpdateURL, "source-url", "",
		"The new source URL to use for the licence")
}
