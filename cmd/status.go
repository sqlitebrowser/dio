package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// Displays whether a database has been modified since the last commit
var statusCmd = &cobra.Command{
	Use:   "status [database name]",
	Short: "Displays whether a database has been modified since the last commit",
	RunE: func(cmd *cobra.Command, args []string) error {
		return status(args)
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}

func status(args []string) error {
	// TODO: If no database name is given, should we show the status for all databases in the current directory?

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
	meta, err := localFetchMetadata(db, true)
	if err != nil {
		return err
	}

	// Check if the file has changed, and let the user know
	changed, err := dbChanged(db, meta)
	if err != nil {
		return err
	}
	if changed {
		_, err = fmt.Fprintf(fOut, "  * '%s': has been changed\n", db)
		if err != nil {
			return err
		}
		return nil
	}
	_, err = fmt.Fprintf(fOut, "  * '%s': unchanged\n", db)
	return err
}
