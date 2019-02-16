package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Displays a list of the databases on the DBHub.io server.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Returns a list of available databases",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Include things like # stars and fork count too

		// Retrieve the database list from the cloud
		resp, body, errs := rq.New().
			TLSClientConfig(&TLSConfig).
			Get(cloud + "/default").
			End()
		if errs != nil {
			e := fmt.Sprintln("Errors when retrieving the database list:")
			for _, err := range errs {
				e += fmt.Sprintf(err.Error())
			}
			return errors.New(e)
		}
		defer resp.Body.Close()
		var list []dbListEntry
		err := json.Unmarshal([]byte(body), &list)
		if err != nil {
			fmt.Printf("Error retrieving database list: '%v'\n", err.Error())
			return err
		}

		// Display the list of databases
		if len(list) == 0 {
			fmt.Printf("Cloud '%s' has no databases\n", cloud)
			return nil
		}
		fmt.Printf("Databases on %s\n\n", cloud)
		for _, j := range list {
			fmt.Printf("  * Database: %s\n", j.Name)
			fmt.Printf("      Default branch: %s\n", j.DefBranch)
			fmt.Printf("      Size: %d bytes\n", j.Size)
			if j.Licence != "" {
				fmt.Printf("      Licence: %s\n", j.Licence)
			} else {
				fmt.Println("      Licence: Not specified")
			}
			fmt.Printf("      Last Modified: %s\n\n", j.LastModified)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
