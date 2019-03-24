package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// Selects the default database, or if no database name is given it displays the default database
var selectCmd = &cobra.Command{
	Use:   "select",
	Short: "Selects the default database used by all dio commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		return selectDefault(args)
	},
}

func init() {
	RootCmd.AddCommand(selectCmd)
}

func selectDefault(args []string) error {
	// Ensure a database file was given
	var db string
	var err error
	if len(args) == 0 {
		db, err = getDefaultDatabase()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(fOut, "Default database: '%s'\n", db)
		if err != nil {
			return err
		}
		return nil
	}
	if len(args) > 1 {
		return errors.New("Only one database can be selected as the default (for now)")
	}

	// Save the given text string as the default database
	// TODO: Add some error checking here (eg does the database exist locally or remotely?)
	db = args[0]
	err = saveDefaultDatabase(db)
	if err != nil {
		return err
	}
	return nil
}
