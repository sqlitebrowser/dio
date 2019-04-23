package cmd

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

var licenceAddFile, licenceAddFileFormat, licenceAddFullName, licenceAddURL string
var licenceAddDisplayOrder int

// Adds a licence to the list of known licences on the server
var licenceAddCmd = &cobra.Command{
	Use:   "add [licence name]",
	Short: "Add a licence to the list of known licences on a DBHub.io cloud",
	RunE: func(cmd *cobra.Command, args []string) error {
		return licenceAdd(args)
	},
}

func init() {
	licenceCmd.AddCommand(licenceAddCmd)
	licenceAddCmd.Flags().IntVar(&licenceAddDisplayOrder, "display-order", 0,
		"Used when displaying a list of available licences.  This adjusts the position in the list.")
	licenceAddCmd.Flags().StringVar(&licenceAddFileFormat, "file-format", "text",
		"The content format of the file.  Either text or html")
	licenceAddCmd.Flags().StringVar(&licenceAddFullName, "full-name", "",
		"The full name of the licence")
	licenceAddCmd.Flags().StringVar(&licenceAddFile, "licence-file", "",
		"Path to a file containing the licence as text")
	licenceAddCmd.Flags().StringVar(&licenceAddURL, "source-url", "",
		"Optional reference URL for the licence")
}

func licenceAdd(args []string) error {
	// Ensure a short licence name is present
	if len(args) == 0 {
		return errors.New("A short licence name or identifier is needed.  eg CC0-BY-1.0")
	}
	if len(args) > 1 {
		return errors.New("Only one licence can be added at a time (for now)")
	}

	// Ensure a display order was specified
	if licenceAddDisplayOrder == 0 {
		return errors.New("A (unique) display order # must be given")
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
		Query(fmt.Sprintf("display_order=%d", licenceAddDisplayOrder)).
		Set("User-Agent", fmt.Sprintf("Dio %s", DIO_VERSION)).
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
	resp, body, errs := req.End()
	if errs != nil {
		_, err = fmt.Fprint(fOut, "Errors when adding licence:")
		if err != nil {
			return err
		}
		for _, errInner := range errs {
			errTxt := errInner.Error()
			_, errInnerInner := fmt.Fprint(fOut, errTxt)
			if errInnerInner != nil {
				return errInnerInner
			}
		}
		return errors.New("Error when adding licence")
	}
	if resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusConflict {
			return errors.New(body)
		}

		return errors.New(fmt.Sprintf("Adding licence failed with an error: HTTP status %d - '%v'\n",
			resp.StatusCode, resp.Status))
	}

	_, err = fmt.Fprintf(fOut, "Licence '%s' added\n", name)
	return err
}
