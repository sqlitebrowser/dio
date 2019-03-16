package cmd

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// Displays the list of tags for a remote database
var tagListCmd = &cobra.Command{
	Use:   "tags [database name]",
	Short: "Displays a list of tags for a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tagList(args)
	},
}

func init() {
	RootCmd.AddCommand(tagListCmd)
}

func tagList(args []string) error {
	// Ensure a database file was given
	if len(args) == 0 {
		return errors.New("No database file specified")
	}
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

	if len(meta.Tags) == 0 {
		_, err = fmt.Fprintf(fOut, "Database %s has no tags\n", db)
		return err
	}

	// Sort the list alphabetically
	var sortedKeys []string
	for k := range meta.Tags {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Display the list of tags
	_, err = fmt.Fprintf(fOut, "Tags for %s:\n\n", db)
	if err != nil {
		return err
	}
	for _, i := range sortedKeys {
		_, err = fmt.Fprintf(fOut, "  * '%s' : commit %s\n\n", i, meta.Tags[i].Commit)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(fOut, "      Author: %s <%s>\n", meta.Tags[i].TaggerName, meta.Tags[i].TaggerEmail)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(fOut, "      Date: %s\n", meta.Tags[i].Date.Format(time.UnixDate))
		if err != nil {
			return err
		}
		if meta.Tags[i].Description != "" {
			_, err = fmt.Fprintf(fOut, "      Message: %s\n\n", meta.Tags[i].Description)
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
