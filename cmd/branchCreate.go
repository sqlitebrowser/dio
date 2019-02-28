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

// Creates a branch for a database
var branchCreateCmd = &cobra.Command{
	Use:   "create [database name] --branch xxx --commit yyy",
	Short: "Create a branch for a database",
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

		// Ensure a new branch name and commit ID were given
		if branch == "" {
			return errors.New("No branch name given")
		}
		if commit == "" {
			return errors.New("No commit ID given")
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

		// Ensure a branch with the same name doesn't already exist
		if _, ok := meta.Branches[branch]; ok == true {
			return errors.New("A branch with that name already exists")
		}

		// Make sure the target commit exists in our commit list
		c, ok := meta.Commits[commit]
		if ok != true {
			return errors.New("That commit isn't in the database commit list")
		}

		// Count the number of commits in the new branch
		numCommits := 1
		for c.Parent != "" {
			numCommits++
			c = meta.Commits[c.Parent]
		}

		// Generate the new branch info locally
		newBranch := branchEntry{
			Commit:      commit,
			CommitCount: numCommits,
			Description: msg,
		}

		// Add the new branch to the local metadata cache
		meta.Branches[branch] = newBranch

		// Serialise the updated metadata to JSON
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

		fmt.Printf("Branch '%s' created\n", branch)
		return nil
	},
}

func init() {
	branchCmd.AddCommand(branchCreateCmd)
	branchCreateCmd.Flags().StringVar(&branch, "branch", "", "Name of remote branch to create")
	branchCreateCmd.Flags().StringVar(&commit, "commit", "", "Commit ID for the new branch head")
	branchCreateCmd.Flags().StringVar(&msg, "description", "", "Description of the branch")
}
