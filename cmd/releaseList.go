package cmd

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// Displays the list of releases for a remote database
var releaseListCmd = &cobra.Command{
	Use:   "releases [database name]",
	Short: "Displays a list of releases for a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a database file was given
		if len(args) == 0 {
			return errors.New("No database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one database can be worked with at a time (for now)")
		}

		// If there is a local metadata cache for the requested database, use that.  Otherwise, retrieve it from the
		// server first (without storing it)
		db := args[0]
		meta, err := localFetchMetadata(db, true)
		if err != nil {
			return err
		}

		if len(meta.Releases) == 0 {
			fmt.Printf("Database %s has no releases\n", db)
			return nil
		}

		// Sort the list alphabetically
		var sortedKeys []string
		for k := range meta.Releases {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Strings(sortedKeys)

		// Display the list of releases
		fmt.Printf("Releases for %s:\n\n", db)
		for _, i := range sortedKeys {
			fmt.Printf("  * %s : commit %s\n\n", i, meta.Releases[i].Commit)
			fmt.Printf("      Author: %s <%s>\n", meta.Releases[i].ReleaserName, meta.Releases[i].ReleaserEmail)
			fmt.Printf("      Date: %s\n", meta.Releases[i].Date.Format(time.UnixDate))
			numFormat.Printf("      Size: %d\n", meta.Releases[i].Size)
			if meta.Releases[i].Description != "" {
				fmt.Printf("      Message: %s\n\n", meta.Releases[i].Description)
			} else {
				fmt.Println()
			}
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(releaseListCmd)
}
