package cmd

import (
	"fmt"

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
		dbList, err := getDatabases(cloud, certUser)
		if err != nil {
			return err
		}

		// Display the list of databases
		if len(dbList) == 0 {
			fmt.Printf("Cloud '%s' has no databases\n", cloud)
			return nil
		}
		fmt.Printf("Databases on %s\n\n", cloud)
		for _, j := range dbList {
			fmt.Printf("  * Database: %s\n", j.Name)
			if j.OneLineDesc != "" {
				fmt.Printf("      Description: %s\n", j.OneLineDesc)
			}
			fmt.Printf("      Default branch: %s\n", j.DefBranch)
			_, err := numFormat.Printf("      Size: %d bytes\n", j.Size)
			if err != nil {
				fmt.Println(err) // Not sure if this is the right approach
			}
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
