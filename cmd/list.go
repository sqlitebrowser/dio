package cmd

import (
	"encoding/json"
	"errors"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Displays the list of databases on DBHub.io for the user.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Returns the list of your databases on DBHub.io",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: Include things like # stars and fork count too
		// TODO: Add parameter for listing the (public) databases of other user(s) too

		// Retrieve the database list for the user
		resp, body, errs := rq.New().TLSClientConfig(&TLSConfig).Get(fmt.Sprintf("%s/%s", cloud, certUser)).End()
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
			fmt.Printf("      File last modified: %s\n", j.LastModified)
			fmt.Printf("      Repository last updated: %s\n\n", j.RepoModified)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
