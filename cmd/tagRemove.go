package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var tagRemoveTag string

// Removes a tag from a database
var tagRemoveCmd = &cobra.Command{
	Use:   "remove [database name] --tag xxx",
	Short: "Remove a tag from a database",
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

		// Ensure a tag name was given
		if tagRemoveTag == "" {
			return errors.New("No tag name given")
		}

		// Load the metadata
		db := args[0]
		meta, err := loadMetadata(db)
		if err != nil {
			return err
		}

		// Check if the tag exists
		if _, ok := meta.Tags[tagRemoveTag]; ok != true {
			return errors.New("A tag with that name doesn't exist")
		}

		// Remove the tag
		delete(meta.Tags, tagRemoveTag)

		// Save the updated metadata back to disk
		err = saveMetadata(db, meta)
		if err != nil {
			return err
		}

		fmt.Printf("Tag '%s' removed\n", tagRemoveTag)
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagRemoveCmd)
	tagRemoveCmd.Flags().StringVar(&tagRemoveTag, "tag", "", "Name of remote tag to remove")
}
