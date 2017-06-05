package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Removes a licence from the system.
var licenceRemoveCmd = &cobra.Command{
	Use:   "remove [licence name]",
	Short: "Removes a license from the list of available licences on the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a licence friendly name is present
		if len(args) == 0 {
			return errors.New("Human friendly licence name is needed.  eg CC0-BY-1.0")
		}
		// TODO: Allow giving multiple licence names on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one licence can be removed at a time (for now)")
		}

		// Remove the licence
		lic := args[0]
		resp, _, errs := rq.New().Post(cloud+"/licence_remove").
			Set("licence", lic).
			End()
		if errs != nil {
			log.Print("Errors when removing licence:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when removing licence")
		}
		if resp.StatusCode != http.StatusNoContent {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested licence not found")
			}
			return errors.New(fmt.Sprintf("Licence removal failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Printf("Licence '%s' removed\n", lic)
		return nil
	},
}

func init() {
	licenceCmd.AddCommand(licenceRemoveCmd)
}
