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
		// TODO: Allow giving multiple licence names on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one licence can be downloaded at a time (for now)")
		}

		// Download the licence text
		lic := args[0]
		resp, body, errs := rq.New().Get(cloud+"/licence_get").
			Set("name", lic).End()
		if errs != nil {
			log.Print("Errors when downloading licence text:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when downloading licence text")
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested licence not found")
			}
			return errors.New(fmt.Sprintf("Download failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		// Write the licence to disk
		err := ioutil.WriteFile(lic+".txt", []byte(body), 0644)
		if err != nil {
			return err
		}
		fmt.Printf("Licence '%s.txt' downloaded from %s.\n", lic, cloud)
		return nil
	},
}

func init() {
	licenceCmd.AddCommand(licenceGetCmd)
}
