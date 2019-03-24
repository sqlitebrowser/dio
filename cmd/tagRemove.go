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
		return tagRemove(args)
	},
}

func init() {
	tagCmd.AddCommand(tagRemoveCmd)
	tagRemoveCmd.Flags().StringVar(&tagRemoveTag, "tag", "", "Name of remote tag to remove")
}

func tagRemove(args []string) error {
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

	// Ensure a tag name was given
	if tagRemoveTag == "" {
		return errors.New("No tag name given")
	}

	// Load the metadata
	meta, err = loadMetadata(db)
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

	_, err = fmt.Fprintf(fOut, "Tag '%s' removed\n", tagRemoveTag)
	return err
}
