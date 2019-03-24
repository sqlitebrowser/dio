package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	releaseCreateCommit, releaseCreateRelease, releaseCreateReleaseDate   string
	releaseCreateCreatorEmail, releaseCreateCreatorName, releaseCreateMsg string
)

// Creates a release for a database
var releaseCreateCmd = &cobra.Command{
	Use:   "create [database name] --release xxx --commit yyy",
	Short: "Create a release for a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		return releaseCreate(args)
	},
}

func init() {
	releaseCmd.AddCommand(releaseCreateCmd)
	releaseCreateCmd.Flags().StringVar(&releaseCreateCommit, "commit", "", "Commit ID for the new release")
	releaseCreateCmd.Flags().StringVar(&releaseCreateCreatorEmail, "email", "", "Email address of release creator")
	releaseCreateCmd.Flags().StringVar(&releaseCreateCreatorName, "name", "", "Name of release creator")
	releaseCreateCmd.Flags().StringVar(&releaseCreateMsg, "message", "", "Description / message for the release")
	releaseCreateCmd.Flags().StringVar(&releaseCreateRelease, "release", "", "Name of release to create")
	releaseCreateCmd.Flags().StringVar(&releaseCreateReleaseDate, "date", "", "Custom timestamp (RFC3339 format) for release")
}

func releaseCreate(args []string) error {
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

	// Ensure a new release name and commit ID were given
	if releaseCreateRelease == "" {
		return errors.New("No release name given")
	}
	if releaseCreateCommit == "" {
		return errors.New("No commit ID given")
	}

	// Make sure we have the email and name of the release creator.  Either by loading it from the config file, or
	// getting it from the command line arguments
	if releaseCreateCreatorEmail == "" {
		if viper.IsSet("user.email") == false {
			return errors.New("No email address provided")
		}
		releaseCreateCreatorEmail = viper.GetString("user.email")
	}

	if releaseCreateCreatorName == "" {
		if viper.IsSet("user.name") == false {
			return errors.New("No name provided")
		}
		releaseCreateCreatorName = viper.GetString("user.name")
	}

	// Make sure the database file exists, and get it's file size
	fileInfo, err := os.Stat(db)
	if os.IsNotExist(err) {
		return err
	}
	size := fileInfo.Size()

	// If a date was given, parse it to ensure the format is correct.  Warn the user if it isn't,
	releaseTimeStamp := time.Now()
	if releaseCreateReleaseDate != "" {
		releaseTimeStamp, err = time.Parse(time.RFC3339, releaseCreateReleaseDate)
		if err != nil {
			return err
		}
	}

	// Load the metadata
	meta, err = loadMetadata(db)
	if err != nil {
		return err
	}

	// Ensure a release with the same name doesn't already exist
	if _, ok := meta.Releases[releaseCreateRelease]; ok == true {
		return errors.New("A release with that name already exists")
	}

	// Generate the new release info locally
	newRelease := releaseEntry{
		Commit:        releaseCreateCommit,
		Date:          releaseTimeStamp,
		Description:   releaseCreateMsg,
		ReleaserEmail: releaseCreateCreatorEmail,
		ReleaserName:  releaseCreateCreatorName,
		Size:          int(size),
	}

	// Add the new release to the local metadata cache
	meta.Releases[releaseCreateRelease] = newRelease

	// Save the updated metadata back to disk
	err = saveMetadata(db, meta)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(fOut, "Release creation succeeded")
	return err
}
