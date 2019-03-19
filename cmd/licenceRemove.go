package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Removes a licence from the system.
var licenceRemoveCmd = &cobra.Command{
	Use:   "remove [licence name]",
	Short: "Removes a licence from the list of known licences on the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return licenceRemove(args)
	},
}

func init() {
	licenceCmd.AddCommand(licenceRemoveCmd)
}

func licenceRemove(args []string) error {
	// Ensure a licence friendly name is present
	if len(args) == 0 {
		return errors.New("A short licence name or identified is needed.  eg CC0-BY-1.0")
	}
	if len(args) > 1 {
		return errors.New("Only one licence can be removed at a time (for now)")
	}

	// Remove the licence
	name := args[0]
	resp, body, errs := rq.New().TLSClientConfig(&TLSConfig).Post(fmt.Sprintf("%s/licence/remove", cloud)).
		Query(fmt.Sprintf("licence_id=%s", url.QueryEscape(name))).End()
	if errs != nil {
		_, err := fmt.Fprint(fOut, "Errors when removing licence:")
		if err != nil {
			return err
		}
		for _, err := range errs {
			_, errInner := fmt.Fprint(fOut, err.Error())
			if errInner != nil {
				return errInner
			}
		}
		return errors.New("Error when removing licence")
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New(body)
	}

	_, err := fmt.Fprintf(fOut, "Licence '%s' removed\n", name)
	return err
}
