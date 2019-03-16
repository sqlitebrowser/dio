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
	if len(args) == 0 {
		return errors.New("No database file specified")
	}
	if len(args) > 1 {
		return errors.New("Only one database can be worked with at a time (for now)")
	}

	// Load the local metadata cache, without retrieving updated metadata from the cloud
	db := args[0]
	meta, err := localFetchMetadata(db, false)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(fOut, "Active branch: %s\n", meta.ActiveBranch)
	return err
}
