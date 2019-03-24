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
		return releaseList(args)
	},
}

func init() {
	RootCmd.AddCommand(releaseListCmd)
}

func releaseList(args []string) error {
	// Ensure a database file was given
	var db string
	var err error
	var meta metaData
	if len(args) == 0 {
		db, err = getDefaultDatabase()
		if err != nil {
			return err
		}
		if db == "" {
			// No database name was given on the command line, and we don't have a default database selected
			return errors.New("No database file specified")
		}
	} else {
		db = args[0]
	}
	if len(args) > 1 {
		return errors.New("Only one database can be worked with at a time (for now)")
	}

	// If there is a local metadata cache for the requested database, use that.  Otherwise, retrieve it from the
	// server first (without storing it)
	meta, err = localFetchMetadata(db, true)
	if err != nil {
		return err
	}

	if len(meta.Releases) == 0 {
		_, err = fmt.Fprintf(fOut, "Database %s has no releases\n", db)
		return err
	}

	// Sort the list alphabetically
	var sortedKeys []string
	for k := range meta.Releases {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Display the list of releases
	_, err = fmt.Fprintf(fOut, "Releases for %s:\n\n", db)
	if err != nil {
		return err
	}
	for _, i := range sortedKeys {
		_, err = fmt.Fprintf(fOut, "  * '%s' : commit %s\n\n", i, meta.Releases[i].Commit)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(fOut, "      Author: %s <%s>\n", meta.Releases[i].ReleaserName, meta.Releases[i].ReleaserEmail)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(fOut, "      Date: %s\n", meta.Releases[i].Date.Format(time.UnixDate))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(fOut, numFormat.Sprintf("      Size: %d", meta.Releases[i].Size))
		if err != nil {
			return err
		}
		if meta.Releases[i].Description != "" {
			_, err = fmt.Fprintf(fOut, "      Message: %s\n\n", meta.Releases[i].Description)
			if err != nil {
				return err
			}
		} else {
			_, err = fmt.Fprintln(fOut)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
