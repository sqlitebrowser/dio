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

// Removes a branch from a database
var branchRemoveCmd = &cobra.Command{
	Use:   "remove [database name] --branch xxx",
	Short: "Removes a branch from a database",
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

		// Ensure a branch name was given
		if branch == "" {
			return errors.New("No branch name given")
		}

		// If there isn't a local metadata cache for the requested database, retrieve it from the server (and store it)
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
			return err
		}
		meta := metaData{}
		err = json.Unmarshal([]byte(md), &meta)
		if err != nil {
			return err
		}

		// Check if the branch exists
		if _, ok := meta.Branches[branch]; ok != true {
			return errors.New("A branch with that name doesn't exist")
		}

		// Remove the branch
		delete(meta.Branches, branch)

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

		fmt.Printf("Branch '%s' removed\n", branch)
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchRemoveCmd)
	branchRemoveCmd.Flags().StringVar(&branch, "branch", "", "Name of remote branch to remove")
}
