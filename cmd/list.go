package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// Displays the list of databases on DBHub.io for the user.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Returns the list of your databases on DBHub.io",
	RunE: func(cmd *cobra.Command, args []string) error {
		return list(args)
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}

func list(args []string) error {
	// TODO: Include things like # stars and fork count too
	// TODO: Add parameter for listing the (public) databases of other user(s) too

	// Retrieve the database list for the user
	dbList, err := getDatabases(cloud, certUser)
	if err != nil {
		return err
	}

	// Display the list of databases
	if len(dbList) == 0 {
		_, err = fmt.Fprintf(fOut, "Cloud '%s' has no databases\n", cloud)
		return err
	}
	fmt.Printf("Databases on %s\n\n", cloud)
	for _, j := range dbList {
		_, err = fmt.Fprintf(fOut, "  * Database: %s\n", j.Name)
		if err != nil {
			return err
		}
		if j.OneLineDesc != "" {
			_, err = fmt.Fprintf(fOut, "      Description: %s\n", j.OneLineDesc)
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(fOut, "      Default branch: %s\n", j.DefBranch)
		if err != nil {
			return err
		}
		_, err := numFormat.Fprintf(fOut, "      Size: %d bytes\n", j.Size)
		if err != nil {
			return err
		}
		if j.Licence != "" {
			_, err = fmt.Fprintf(fOut, "      Licence: %s\n", j.Licence)
			if err != nil {
				return err
			}
		} else {
			_, err = fmt.Fprintf(fOut, "      Licence: Not specified")
			if err != nil {
				return err
			}
		}
		// The server gives us the last modified and repo modified dates in pre-formatted UTC timezone.  For now, lets
		// convert these back to the users local time
		z, err := time.Parse(time.RFC3339, j.LastModified)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(fOut, "      File last modified: %s\n", z.Local().Format(time.RFC1123))
		if err != nil {
			return err
		}
		z, err = time.Parse(time.RFC3339, j.RepoModified)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(fOut, "      Repository last updated: %s\n\n", z.Local().Format(time.RFC1123))
		if err != nil {
			return err
		}
	}
	return nil
}
