package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

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
	var d, db, userName string
	var err error
	var meta metaData
	if len(args) == 0 {
		d, err = getDefaultDatabase()
		if err != nil {
			return err
		}
		if d == "" {
			// No database name was given on the command line, and we don't have a default database selected
			return errors.New("No database file specified")
		}
	} else {
		d = args[0]
	}
	if len(args) > 1 {
		return errors.New("Only one database can be worked with at a time (for now)")
	}

	// Split database name into username/database parts
	s := strings.Split(d, "/")
	switch len(s) {
	case 1:
		// Probably a database belonging to the user
		userName = certUser
		db = d
	case 2:
		// Probably a username/database string
		userName = s[0]
		db = s[1]
	default:
		return errors.New("Can't parse the given database name")
	}

	// If there is a local metadata cache for the requested database, use that.  Otherwise, retrieve it from the
	// server first (without storing it)
	meta = metaData{}
	md, err := ioutil.ReadFile(filepath.Join(".dio", userName, db, "metadata.json"))
	if err == nil {
		err = json.Unmarshal([]byte(md), &meta)
		if err != nil {
			return err
		}
	} else {
		// No local cache, so retrieve the info from the server
		meta, _, err = retrieveMetadata(userName, db)
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
	_, err = fmt.Fprintf(fOut, "Branches for %s/%s:\n\n", userName, db)
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
