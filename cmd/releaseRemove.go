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
		// Ensure a database file was given
		if len(args) == 0 {
			return errors.New("No database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one database can be changed at a time (for now)")
		}

		// Ensure a release name was given
		if releaseRemoveRelease == "" {
			return errors.New("No release name given")
		}

		// Load the metadata
		db := args[0]
		meta, err := loadMetadata(db)
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

		fmt.Printf("Release '%s' removed\n", releaseRemoveRelease)
		return nil
	},
}

func init() {
	releaseCmd.AddCommand(releaseRemoveCmd)
	releaseRemoveCmd.Flags().StringVar(&releaseRemoveRelease, "release", "", "Name of release to remove")
}
