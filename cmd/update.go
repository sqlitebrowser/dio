package cmd

import (
	"errors"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

var updateCmdBranch, updateCmdLicence, updateCmdMessage string

// Update the details for a database
var updateCmd = &cobra.Command{
	Use:   "update [database name]",
	Short: "Update the details for a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure a database file was given
		if len(args) == 0 {
			return errors.New("No database file specified")
		}
		// TODO: Allow giving multiple database files on the command line.  Hopefully just needs turning this
		// TODO  into a for loop
		if len(args) > 1 {
			return errors.New("Only one database can be updated at a time (for now)")
		}

		// Ensure a new licence name was given
		if updateCmdLicence == "" {
			return errors.New("Missing licence info")
		}

		// Send the details to the API server
		db := args[0]
		req := rq.New().Post(cloud+"/db_update").
			Set("database", db).
			Set("licence", updateCmdLicence)
		if updateCmdMessage != "" {
			req.Set("message", updateCmdMessage)
		}
		if pushCmdLicence != "" {
			req.Set("branch", updateCmdBranch)
		}
		resp, _, errs := req.End()
		if errs != nil {
			log.Print("Errors when updating database licence:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when updating database licence")
		}
		if resp != nil {
			if resp.StatusCode == http.StatusConflict {
				return errors.New(fmt.Sprintf("'%s' is already %s licenced.", db,
					updateCmdLicence))
			}
			if resp.StatusCode != http.StatusNoContent {
				return errors.New(fmt.Sprintf("Update failed with an error: HTTP status %d - '%v'\n",
					resp.StatusCode, resp.Status))
			}
		}
		fmt.Printf("Licence changed for '%s'\n", db)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVar(&updateCmdBranch, "branch", "",
		"The branch to make this change in")
	updateCmd.Flags().StringVar(&updateCmdLicence, "new-licence", "",
		"The new licence for the database, as per 'dio licence list'")
	updateCmd.Flags().StringVar(&updateCmdMessage, "message", "",
		"Commit message for the update")
}
