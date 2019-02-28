package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

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
		if tag == "" {
			return errors.New("No tag name given")
		}

		// If there isn't a local metadata cache for the requested database, retrieve it from the server (and store  it)
		db := args[0]
		if _, err := os.Stat(filepath.Join(".dio", db, "metadata.json")); os.IsNotExist(err) {
			err := updateMetadata(db)
			if err != nil {
				return err
			}
		}

		// Read in the metadata cache
		md, err := ioutil.ReadFile(filepath.Join(".dio", db, "metadata.json"))
		if err != nil {
			if err != nil {
				return err
			}
		}
		meta := metaData{}
		err = json.Unmarshal([]byte(md), &meta)
		if err != nil {
			return err
		}

		// Check if the tag exists
		if _, ok := meta.Tags[tag]; ok != true {
			return errors.New("A tag with that name doesn't exist")
		}

		// Remove the tag
		delete(meta.Tags, tag)

		// Serialise the updated metadata back to JSON
		jsonString, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return err
		}

		// Write the updated metadata to disk
		mdFile := filepath.Join(".dio", db, "metadata.json")
		err = ioutil.WriteFile(mdFile, []byte(jsonString), 0644)
		if err != nil {
			return err
		}

		fmt.Printf("Tag '%s' removed\n", tag)
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagRemoveCmd)
	tagRemoveCmd.Flags().StringVar(&tag, "tag", "", "Name of remote tag to remove")
}
