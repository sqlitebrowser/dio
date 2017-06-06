package cmd

import (
	"encoding/json"
	"fmt"

	rq "github.com/parnurzeal/gorequest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Displays a list of the available licences.
var licenceListCmd = &cobra.Command{
	Use:   "list",
	Short: "Displays a list of the known licences",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Retrieve the licence list
		resp, body, errs := rq.New().Get(cloud + "/licence_list").End()
		if errs != nil {
			e := fmt.Sprintf("Errors when retrieving the licence list:")
			for _, err := range errs {
				e += err.Error()
			}
			return errors.New(e)
		}
		defer resp.Body.Close()
		var list []licenceEntry
		err := json.Unmarshal([]byte(body), &list)
		if err != nil {
			return errors.New(fmt.Sprintf("Error retrieving licence list: '%v'\n", err.Error()))
		}

		// Display the list of licences
		if len(list) == 0 {
			fmt.Printf("Cloud '%s' knows no licences\n", cloud)
			return nil
		}
		fmt.Printf("Licences on %s\n\n", cloud)
		for _, j := range list {
			fmt.Printf("  * Name: %s\n", j.Name)
			fmt.Printf("    Source URL: %s\n", j.URL)
			fmt.Printf("    SHA256: %s\n\n", j.Sha256)
		}
		return nil
	},
}

func init() {
	licenceCmd.AddCommand(licenceListCmd)
}
