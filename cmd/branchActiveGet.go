package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// Returns the name of the active branch for a database
var branchActiveGetCmd = &cobra.Command{
	Use:   "get [database name]",
	Short: "Get the active branch name for a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		return branchActiveGet(args)
	},
}

func init() {
	branchActiveCmd.AddCommand(branchActiveGetCmd)
}

func branchActiveGet(args []string) error {
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

	// Load the local metadata cache, without retrieving updated metadata from the cloud
	meta, err = localFetchMetadata(db, false)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(fOut, "Active branch: %s\n", meta.ActiveBranch)
	return err
}
