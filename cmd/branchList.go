package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

// Displays the list of branches for a remote database
var branchListCmd = &cobra.Command{
	Use:   "list [database name]",
	Short: "List the branches for your database on a DBHub.io cloud",
	RunE: func(cmd *cobra.Command, args []string) error {
		return branchList(args)
	},
}

func init() {
	branchCmd.AddCommand(branchListCmd)
}

func branchList(args []string) error {
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
	meta := metaData{}
	md, err := ioutil.ReadFile(filepath.Join(".dio", db, "metadata.json"))
	if err == nil {
		err = json.Unmarshal([]byte(md), &meta)
		if err != nil {
			return err
		}
	} else {
		// No local cache, so retrieve the info from the server
		meta, _, err = retrieveMetadata(db)
		if err != nil {
			return err
		}
	}

	// Sort the list alphabetically
	var sortedKeys []string
	for k := range meta.Branches {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Display the list of branches
	_, err = fmt.Fprintf(fOut, "Branches for %s:\n\n", db)
	if err != nil {
		return err
	}
	for _, i := range sortedKeys {
		_, err = fmt.Fprintf(fOut, "  * '%s' - Commit: %s\n", i, meta.Branches[i].Commit)
		if err != nil {
			return err
		}
		if meta.Branches[i].Description != "" {
			_, err = fmt.Fprintf(fOut, "\n      %s\n\n", meta.Branches[i].Description)
			if err != nil {
				return err
			}
		}
	}

	// Extra newline is needed in some cases for consistency
	finalSortedKey := sortedKeys[len(sortedKeys)-1]
	if meta.Branches[finalSortedKey].Description == "" {
		_, err = fmt.Fprintln(fOut)
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(fOut, "    Active branch: %s\n\n", meta.ActiveBranch)
	return err
}
