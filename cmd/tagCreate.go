package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var tagDate string // Optional

// Creates a tag for a database
var tagCreateCmd = &cobra.Command{
	Use:   "create [database name] --tag xxx --commit yyy",
	Short: "Create a tag for a database",
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

		// Ensure a new tag name and commit ID were given
		if tag == "" {
			return errors.New("No tag name given")
		}
		if commit == "" {
			return errors.New("No commit ID given")
		}

		// Make sure we have the email and name of the tag creator.  Either by loading it from the config file, or
		// getting it from the command line arguments
		if email == "" {
			if viper.IsSet("user.email") == false {
				return errors.New("No email address provided")
			}
			email = viper.GetString("user.email")
		}

		if name == "" {
			if viper.IsSet("user.name") == false {
				return errors.New("No name provided")
			}
			name = viper.GetString("user.name")
		}

		// If a date was given, parse it to ensure the format is correct.  Warn the user if it isn't,
		tagTimeStamp := time.Now()
		var err error
		if tagDate != "" {
			tagTimeStamp, err = time.Parse(time.RFC3339, tagDate)
			if err != nil {
				return err
			}
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
		list := metaData{}
		err = json.Unmarshal([]byte(md), &list)
		if err != nil {
			return err
		}

		// Ensure a tag with the same name doesn't already exist
		if _, ok := list.Tags[tag]; ok == true {
			return errors.New("A tag with that name already exists")
		}

		// Generate the new tag info locally
		newTag := tagEntry{
			Commit:      commit,
			Date:        tagTimeStamp,
			Description: msg,
			TaggerEmail: email,
			TaggerName:  name,
		}

		// Add the new tag to the local metadata cache
		list.Tags[tag] = newTag

		// Serialise the updated metadata to JSON
		jsonString, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			return err
		}

		// Write the updated metadata to disk
		mdFile := filepath.Join(".dio", db, "metadata.json")
		err = ioutil.WriteFile(mdFile, []byte(jsonString), 0644)
		if err != nil {
			return err
		}

		fmt.Println("Tag creation succeeded")
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagCreateCmd)
	tagCreateCmd.Flags().StringVar(&commit, "commit", "", "Commit ID for the new tag")
	tagCreateCmd.Flags().StringVar(&tag, "tag", "", "Name of tag to create")
	tagCreateCmd.Flags().StringVar(&tagDate, "date", "", "Custom timestamp (RFC3339 format) for tag")
	tagCreateCmd.Flags().StringVar(&email, "email", "", "Email address of tagger")
	tagCreateCmd.Flags().StringVar(&name, "name", "", "Name of tagger")
	tagCreateCmd.Flags().StringVar(&msg, "message", "", "Description / message for the tag")
}
