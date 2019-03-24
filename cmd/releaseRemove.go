package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var releaseRemoveRelease string

// Removes a release from a database
var releaseRemoveCmd = &cobra.Command{
	Use:   "remove [database name] --release xxx",
	Short: "Remove a release from a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		return releaseRemove(args)
	},
}

func init() {
	releaseCmd.AddCommand(releaseRemoveCmd)
	releaseRemoveCmd.Flags().StringVar(&releaseRemoveRelease, "release", "", "Name of release to remove")
}

func releaseRemove(args []string) error {
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
		return errors.New("Only one database can be changed at a time (for now)")
	}

	// Ensure a release name was given
	if releaseRemoveRelease == "" {
		return errors.New("No release name given")
	}

	// Load the metadata
	meta, err = loadMetadata(db)
	if err != nil {
		return err
	}

	// Check if the release exists
	if _, ok := meta.Releases[releaseRemoveRelease]; ok != true {
		return errors.New("A release with that name doesn't exist")
	}

	// Remove the release
	delete(meta.Releases, releaseRemoveRelease)

	// Save the updated metadata back to disk
	err = saveMetadata(db, meta)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(fOut, "Release '%s' removed\n", releaseRemoveRelease)
	return err
}
