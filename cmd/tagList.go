package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// Displays the list of tags for a remote database
var tagListCmd = &cobra.Command{
	Use:   "tags [database name]",
	Short: "Displays a list of tags for a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a database file was given
		if len(args) == 0 {
			return errors.New("No database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one database can be worked with at a time (for now)")
		}

		// If there is a local metadata cache for the requested database, use that.  Otherwise, retrieve it from the
		// server first (without storing it)
		db := args[0]
		md, err := ioutil.ReadFile(filepath.Join(".dio", db, "metadata.json"))
		if err != nil {
			// No local cache, so retrieve the info from the server
			temp, err := retrieveMetadata(db)
			if err != nil {
				return err
			}
			md = []byte(temp)
		}
		meta := metaData{}
		err = json.Unmarshal([]byte(md), &meta)
		if err != nil {
			return err
		}

		if len(meta.Tags) == 0 {
			fmt.Printf("Database %s has no tags\n", db)
			return nil
		}

		// Sort the list alphabetically
		var sortedKeys []string
		for k := range meta.Tags {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Strings(sortedKeys)

		// Display the list of tags
		fmt.Printf("Tags for %s:\n\n", db)
		for _, i := range sortedKeys {
			fmt.Printf("  * %s : commit %s\n\n", i, meta.Tags[i].Commit)
			fmt.Printf("      Author: %s <%s>\n", meta.Tags[i].TaggerName, meta.Tags[i].TaggerEmail)
			fmt.Printf("      Date: %s\n", meta.Tags[i].Date.Format(time.UnixDate))
			fmt.Printf("      Message: %s\n\n", meta.Tags[i].Description)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	RootCmd.AddCommand(tagListCmd)
}
