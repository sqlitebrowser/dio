package cmd

import (
	"errors"
	"net/http"
	"net/url"
	"os"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

var licenceAddFile, licenceAddFileFormat, licenceAddFullName, licenceAddURL string

// Adds a licence to the list of known licences on the server
var licenceAddCmd = &cobra.Command{
	Use:   "add [licence name]",
	Short: "Add a licence to the list of known licences on a DBHub.io cloud",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a licence friendly name is present
		if len(args) == 0 {
			return errors.New("Human friendly licence name is needed.  eg CC0-BY-1.0")
		}
		if len(args) > 1 {
			return errors.New("Only one licence can be added at a time (for now)")
		}

		// Ensure a licence file was specified, and that it exists
		if licenceAddFile == "" {
			return errors.New("A file containing the licence text is required")
		}
		_, err := os.Stat(licenceAddFile)
		if err != nil {
			return err
		}

		// Send the licence info to the API server
		name := args[0]
		req := rq.New().TLSClientConfig(&TLSConfig).Post(fmt.Sprintf("%s/licence/add", cloud)).
			Type("multipart").
			Query(fmt.Sprintf("licence_id=%s", url.QueryEscape(name))).
			SendFile(licenceAddFile, "", "file1")
		if licenceAddFileFormat != "" {
			req.Query(fmt.Sprintf("file_format=%s", url.QueryEscape(licenceAddFileFormat)))
		}
		if licenceAddFullName != "" {
			req.Query(fmt.Sprintf("licence_name=%s", url.QueryEscape(licenceAddFullName)))
		}
		if licenceAddURL != "" {
			req.Query(fmt.Sprintf("source_url=%s", url.QueryEscape(licenceAddURL)))
		}
		resp, _, errs := req.End()
		if errs != nil {
			fmt.Print("Errors when adding licence:")
			for _, err := range errs {
				fmt.Print(err.Error())
			}
			return errors.New("Error when adding licence")
		}
		if resp.StatusCode != http.StatusCreated {
			if resp.StatusCode == http.StatusConflict {
				return errors.New("A licence using that friendly name already exists")
			}

			return errors.New(fmt.Sprintf("Adding licence failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Printf("Licence '%s' added\n", name)
		return nil
	},
}

func init() {
	licenceCmd.AddCommand(licenceAddCmd)
	licenceAddCmd.Flags().StringVar(&licenceAddFileFormat, "file-format", "text",
		"The content format of the file.  Either text or html")
	licenceAddCmd.Flags().StringVar(&licenceAddFullName, "full-name", "",
		"The full name of the licence")
	licenceAddCmd.Flags().StringVar(&licenceAddFile, "licence-file", "",
		"Path to a file containing the licence as text")
	licenceAddCmd.Flags().StringVar(&licenceAddURL, "source-url", "",
		"Optional reference URL for the licence")
}
