package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
)

// Displays whether a database has been modified since the last commit
var statusCmd = &cobra.Command{
	Use:   "status [database name]",
	Short: "Displays whether a database has been modified since the last commit",
	RunE: func(cmd *cobra.Command, args []string) error {

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
		meta, err := localFetchMetadata(db)
		if err != nil {
			return err
		}

		// Retrieve the sha256, file size, and last modified date from the head commit of the active branch
		head, ok := meta.Branches[meta.ActiveBranch]
		if !ok {
			return errors.New("Aborting: info for the active branch isn't found in the local branch cache")
		}
		c, ok := meta.Commits[head.Commit]
		if !ok {
			return errors.New("Aborting: info for the head commit isn't found in the local commit cache")
		}
		metaSHASum := c.Tree.Entries[0].Sha256
		metaFileSize := c.Tree.Entries[0].Size
		metaLastModified := c.Tree.Entries[0].LastModified

		// If the file size or last modified date in the metadata are different from the current file info, then the
		// local file has probably changed.  Well, "probably" for the last modified day, but "definitely" if the file
		// size is different
		fi, err := os.Stat(db)
		if err != nil {
			return err
		}
		fileSize := int(fi.Size())
		lastModified := fi.ModTime()
		if metaFileSize != fileSize || metaLastModified != lastModified {
			fmt.Printf("  * %s: has been changed\n", db)
			return nil
		}

		// * If the file size and last modified date are still the same, we SHA256 checksum and compare the file *

		// TODO: Should we only do this for smaller files (below some TBD threshold)?

		// Read the database from disk, and calculate it's sha256
		b, err := ioutil.ReadFile(db)
		if err != nil {
			return err
		}
		if len(b) != fileSize {
			return errors.New(numFormat.Sprintf("Aborting: # of bytes read (%d) when reading the database "+
				"doesn't match the database file size (%d)", len(b), fileSize))
		}
		s := sha256.Sum256(b)
		shaSum := hex.EncodeToString(s[:])

		// Let the user know whether the file has been changed or not
		if metaSHASum != shaSum {
			fmt.Printf("  * %s: has been changed\n", db)
			return nil
		}
		fmt.Printf("  * %s: unchanged\n", db)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
