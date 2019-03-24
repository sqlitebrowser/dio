package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	tagCreateCommit, tagCreateDate, tagCreateEmail string
	tagCreateMsg, tagCreateName, tagCreateTag      string
)

// Creates a tag for a database
var tagCreateCmd = &cobra.Command{
	Use:   "create [database name] --tag xxx --commit yyy",
	Short: "Create a tag for a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tagCreate(args)
	},
}

func init() {
	tagCmd.AddCommand(tagCreateCmd)
	tagCreateCmd.Flags().StringVar(&tagCreateCommit, "commit", "", "Commit ID for the new tag")
	tagCreateCmd.Flags().StringVar(&tagCreateDate, "date", "", "Custom timestamp (RFC3339 format) for tag")
	tagCreateCmd.Flags().StringVar(&tagCreateEmail, "email", "", "Email address of tagger")
	tagCreateCmd.Flags().StringVar(&tagCreateMsg, "message", "", "Description / message for the tag")
	tagCreateCmd.Flags().StringVar(&tagCreateName, "name", "", "Name of tagger")
	tagCreateCmd.Flags().StringVar(&tagCreateTag, "tag", "", "Name of tag to create")
}

func tagCreate(args []string) error {
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

	// Ensure a new tag name and commit ID were given
	if tagCreateTag == "" {
		return errors.New("No tag name given")
	}
	if tagCreateCommit == "" {
		return errors.New("No commit ID given")
	}

	// Make sure we have the email and name of the tag creator.  Either by loading it from the config file, or
	// getting it from the command line arguments
	if tagCreateEmail == "" {
		if viper.IsSet("user.email") == false {
			return errors.New("No email address provided")
		}
		tagCreateEmail = viper.GetString("user.email")
	}

	if tagCreateName == "" {
		if viper.IsSet("user.name") == false {
			return errors.New("No name provided")
		}
		tagCreateName = viper.GetString("user.name")
	}

	// If a date was given, parse it to ensure the format is correct.  Warn the user if it isn't,
	tagTimeStamp := time.Now()
	if tagCreateDate != "" {
		tagTimeStamp, err = time.Parse(time.RFC3339, tagCreateDate)
		if err != nil {
			return err
		}
	}

	// Load the metadata
	meta, err = loadMetadata(db)
	if err != nil {
		return err
	}

	// Ensure a tag with the same name doesn't already exist
	if _, ok := meta.Tags[tagCreateTag]; ok == true {
		return errors.New("A tag with that name already exists")
	}

	// Generate the new tag info locally
	newTag := tagEntry{
		Commit:      tagCreateCommit,
		Date:        tagTimeStamp,
		Description: tagCreateMsg,
		TaggerEmail: tagCreateEmail,
		TaggerName:  tagCreateName,
	}

	// Add the new tag to the local metadata cache
	meta.Tags[tagCreateTag] = newTag

	// Save the updated metadata back to disk
	err = saveMetadata(db, meta)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(fOut, "Tag creation succeeded")
	return err
}
