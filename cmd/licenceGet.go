package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Downloads a licence from a DBHub.io cloud.
var licenceGetCmd = &cobra.Command{
	Use:   "get [licence name]",
	Short: "Downloads the text for a licence from a DBHub.io cloud, saving it to [licence name].txt",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a licence name was given
		if len(args) == 0 {
			return errors.New("No licence name specified")
		}
		// TODO: The key word "all" should be a short cut for downloading all of the licences.  When implemented, make
		//       sure to add info about the all keyword to the help text

		// Download the licence text
		dlStatus := make(map[string]string)
		for _, lic := range args {
			resp, body, errs := rq.New().TLSClientConfig(&TLSConfig).Get(cloud + "/licence/get").
				Query(fmt.Sprintf("licence=%s", lic)).End()
			if errs != nil {
				for _, err := range errs {
					log.Print(err.Error())
				}
				dlStatus[lic] = "Error when downloading licence text"
				continue
			}
			if resp.StatusCode != http.StatusOK {
				if resp.StatusCode == http.StatusNotFound {
					dlStatus[lic] = "Requested licence not found"
					continue
				}
				dlStatus[lic] = fmt.Sprintf("Download failed with an error: HTTP status %d - '%v'",
					resp.StatusCode, resp.Status)
				continue
			}

			// Write the licence to disk
			var ext string
			if resp.Header.Get("Content-Type") == "text/html" {
				ext = "html"
			} else {
				ext = "txt"
			}
			err := ioutil.WriteFile(fmt.Sprintf("%s.%s", lic, ext), []byte(body), 0644)
			if err != nil {
				dlStatus[lic] = err.Error()
			}
			dlStatus[lic] = fmt.Sprintf("Licence '%s.%s' downloaded", lic, ext)
		}

		// Display the status of the individual licence downloads
		fmt.Printf("Licence downloading completed from: %s\n\n", cloud)
		for i, j := range dlStatus {
			fmt.Printf("  * %s: %s\n", i, j)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	licenceCmd.AddCommand(licenceGetCmd)
}
