package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	pullCmdBranch, pullCmdCommit string
	pullForce                    *bool
)

// Downloads a database from DBHub.io.
var pullCmd = &cobra.Command{
	Use:   "pull [database name]",
	Short: "Download a database from DBHub.io",
	RunE: func(cmd *cobra.Command, args []string) error {
		return pull(args)
	},
}

func init() {
	RootCmd.AddCommand(pullCmd)
	pullCmd.Flags().StringVar(&pullCmdBranch, "branch", "",
		"Remote branch the database will be downloaded from")
	pullCmd.Flags().StringVar(&pullCmdCommit, "commit", "",
		"Commit ID of the database to download")
	pullForce = pullCmd.Flags().BoolP("force", "f", false,
		"Overwrite unsaved changes to the database?")
}

func pull(args []string) error {
	// Ensure a database file was given
	var d, defDB string
	var err error
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

	// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
	// TODO  into a for loop
	if len(args) > 1 {
		return errors.New("Only one database can be downloaded at a time (for now)")
	}

	// TODO: Add a --licence option, for automatically grabbing the licence as well
	//       * Probably save it as <database name>-<license short name>.txt/html

	// Ensure we weren't given potentially conflicting info on what to pull down
	if pullCmdBranch != "" && pullCmdCommit != "" {
		return errors.New("Either a branch name or commit ID can be given.  Not both at the same time!")
	}

	// TODO: Check if the database given is really a username/database combination
	var db, userName string
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

	// Retrieve metadata for the database
	var meta metaData
	meta, err = updateMetadata(userName, db, false) // Don't store the metadata to disk yet, in case the download fails
	if err != nil {
		return err
	}

	// If the database file already exists locally, check whether the file has changed since the last commit, and let
	// the user know.  The --force option on the command line overrides this
	if _, err = os.Stat(db); err == nil {
		if *pullForce == false {
			changed, err := dbChanged(db, meta)
			if err != nil {
				return err
			}
			if changed {
				_, err = fmt.Fprintf(fOut, "%s has been changed since the last commit.  Use --force if you "+
					"really want to overwrite it\n", db)
				return err
			}
		}
	}

	// If given, make sure the requested branch exists
	if pullCmdBranch != "" {
		if _, ok := meta.Branches[pullCmdBranch]; ok == false {
			return errors.New("The requested branch doesn't exist")
		}
	}

	// If no specific branch nor commit were requested, we use the active branch set in the metadata
	if pullCmdBranch == "" && pullCmdCommit == "" {
		pullCmdBranch = meta.ActiveBranch
	}

	// If given, make sure the requested commit exists
	var lastMod time.Time
	var ok bool
	var thisSha string
	var thisCommit commitEntry
	if pullCmdCommit != "" {
		thisCommit, ok = meta.Commits[pullCmdCommit]
		if ok == false {
			return errors.New("The requested commit doesn't exist")
		}
		thisSha = thisCommit.Tree.Entries[0].Sha256
		lastMod = thisCommit.Tree.Entries[0].LastModified
	} else {
		// Determine the sha256 of the database file
		c := meta.Branches[pullCmdBranch].Commit
		thisCommit, ok = meta.Commits[c]
		if ok == false {
			return errors.New("The requested commit doesn't exist")
		}
		thisSha = thisCommit.Tree.Entries[0].Sha256
		lastMod = thisCommit.Tree.Entries[0].LastModified
	}

	// Check if the database file already exists in local cache
	if thisSha != "" {
		if _, err = os.Stat(filepath.Join(".dio", db, "db", thisSha)); err == nil {
			// The database is already in the local cache, so use that instead of downloading from DBHub.io
			var b []byte
			b, err = ioutil.ReadFile(filepath.Join(".dio", db, "db", thisSha))
			if err != nil {
				return err
			}
			err = ioutil.WriteFile(db, b, 0644)
			if err != nil {
				return err
			}
			err = os.Chtimes(db, time.Now(), lastMod)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(fOut, "Database '%s' refreshed from local cache\n", db)
			if err != nil {
				return err
			}
			if pullCmdBranch != "" {
				_, err = fmt.Fprintf(fOut, "  * Branch: '%s'\n", pullCmdBranch)
				if err != nil {
					return err
				}
			}
			if pullCmdCommit != "" {
				_, err = fmt.Fprintf(fOut, "  * Commit: %s\n", pullCmdCommit)
				if err != nil {
					return err
				}
			}
			_, err = numFormat.Fprintf(fOut, "  * Size: %d bytes\n", len(b))
			if err != nil {
				return err
			}

			// Update the branch metadata with the commit info
			var oldBranch branchEntry
			if pullCmdBranch == "" {
				oldBranch = meta.Branches[meta.ActiveBranch]
			} else {
				oldBranch = meta.Branches[pullCmdBranch]
			}
			commitCount := 1
			z := meta.Commits[thisCommit.ID]
			for z.Parent != "" {
				commitCount++
				z = meta.Commits[z.Parent]
			}
			newBranch := branchEntry{
				Commit:      thisCommit.ID,
				CommitCount: commitCount,
				Description: oldBranch.Description,
			}
			if pullCmdBranch == "" {
				meta.Branches[meta.ActiveBranch] = newBranch
			} else {
				meta.Branches[pullCmdBranch] = newBranch
			}

			// Save the updated metadata to disk
			err = saveMetadata(db, meta)
			if err != nil {
				return err
			}

			// If a default database isn't already selected, we use this one as the default
			defDB, err = getDefaultDatabase()
			if err != nil {
				return err
			}
			if defDB == "" {
				err = saveDefaultDatabase(db)
				if err != nil {
					return err
				}
			}
			return nil
		}
	}

	// Download the database file
	// TODO: Use a streaming download approach, so download progress can be shown.  Something like this should help:
	//         https://stackoverflow.com/questions/22108519/how-do-i-read-a-streaming-response-body-using-golangs-net-http-package
	_, err = fmt.Fprintf(fOut, "Downloading '%s' from %s...\n", db, cloud)
	if err != nil {
		return err
	}
	resp, body, err := retrieveDatabase(db, pullCmdBranch, pullCmdCommit)
	if err != nil {
		return err
	}

	// Create the local database cache directory, if it doesn't yet exist
	if _, err = os.Stat(filepath.Join(".dio", db, "db")); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Join(".dio", db, "db"), 0770)
		if err != nil {
			return err
		}
	}

	// Calculate the sha256 of the database file
	s := sha256.Sum256(body)
	shaSum := hex.EncodeToString(s[:])

	// Write the database file to disk in the cache directory
	err = ioutil.WriteFile(filepath.Join(".dio", db, "db", shaSum), body, 0644)
	if err != nil {
		return err
	}

	// Write the database file to disk again, this time in the working directory
	err = ioutil.WriteFile(db, body, 0644)
	if err != nil {
		return err
	}

	// If the headers included the modification-date parameter for the database, set the last accessed and last
	// modified times on the new database file
	if disp := resp.Header.Get("Content-Disposition"); disp != "" {
		s := strings.Split(disp, ";")
		if len(s) == 4 {
			a := strings.TrimLeft(s[2], " ")
			if strings.HasPrefix(a, "modification-date=") {
				b := strings.Split(a, "=")
				c := strings.Trim(b[1], "\"")
				lastMod, err := time.Parse(time.RFC3339, c)
				if err != nil {
					return err
				}
				err = os.Chtimes(db, time.Now(), lastMod)
				if err != nil {
					return err
				}
			}
		}
	}

	// If the server provided a branch name, add it to the local metadata cache
	if branch := resp.Header.Get("Branch"); branch != "" {
		meta.ActiveBranch = branch
	}

	// The download succeeded, so save the updated metadata to disk
	err = saveMetadata(db, meta)
	if err != nil {
		return err
	}

	// If a default database isn't already selected, we use this one as the default
	defDB, err = getDefaultDatabase()
	if err != nil {
		return err
	}
	if defDB == "" {
		err = saveDefaultDatabase(db)
		if err != nil {
			return err
		}
	}

	// Display success message to the user
	comID := resp.Header.Get("Commit-Id")
	_, err = fmt.Fprintln(fOut, "Downloaded complete")
	if err != nil {
		return err
	}
	if pullCmdBranch != "" {
		_, err = fmt.Fprintf(fOut, "  * Branch: '%s'\n", pullCmdBranch)
		if err != nil {
			return err
		}
	}
	if comID != "" {
		_, err = fmt.Fprintf(fOut, "  * Commit: %s\n", comID)
		if err != nil {
			return err
		}
	}
	_, err = numFormat.Fprintf(fOut, "  * Size: %d bytes\n", len(body))
	return err
}
