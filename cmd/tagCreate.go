package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

var tagAnno *bool
var tagDate string                   // Optional
var tagEmail, tagName, tagMsg string // Only for annotated commits

// Creates a tag for a database
var tagCreateCmd = &cobra.Command{
	Use:   "create [database] --tag xxx --commit yyy",
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

		// If we're creating an annotated tag, ensure the required values are all present
		if *tagAnno == true {
			if tagEmail == "" || tagName == "" || tagMsg == "" {
				return errors.New("Email, name, and message are all required for annotated tags")
			}
		}

		// TODO: If a date was given, parse it to ensure the format is correct.  Warn the user if it isn't,
		// TODO  and display the correct format.  Ideally we'd be able to parse several formats, but I haven't
		// TODO  yet looked for a simple way to do that.

		// Create the tag
		file := args[0]
		r := rq.New().Post(cloud+"/tag_create").
			Set("tag", tag).
			Set("commit", commit).
			Set("database", file)
		if *tagAnno == true {
			// We're creating an annotated tag, so add the required extra information
			if tagDate != "" {
				r.Set("date", tagDate)
			}
			r.Set("taggeremail", tagEmail).
				Set("taggername", tagName).
				Set("msg", tagMsg)
		}
		resp, _, errs := r.End()
		if errs != nil {
			log.Print("Errors when creating tag:")
			for _, err := range errs {
				log.Print(err.Error())
			}
			return errors.New("Error when creating tag")
		}
		if resp.StatusCode != http.StatusNoContent {
			if resp.StatusCode == http.StatusNotFound {
				return errors.New("Requested database or commit not found")
			}
			if resp.StatusCode == http.StatusConflict {
				return errors.New("Requested tag already exists")
			}
			return errors.New(fmt.Sprintf("Tag creation failed with an error: HTTP status %d - '%v'\n",
				resp.StatusCode, resp.Status))
		}

		fmt.Println("Tag creation succeeded")
		return nil
	},
}

func init() {
	tagCmd.AddCommand(tagCreateCmd)
	tagAnno = tagCreateCmd.Flags().BoolP("annotated", "a", false, "Create an annotated tag")
	tagCreateCmd.Flags().StringVar(&commit, "commit", "", "Commit ID for the new tag")
	tagCreateCmd.Flags().StringVar(&tag, "tag", "", "Name of remote tag to create")
	tagCreateCmd.Flags().StringVar(&tagDate, "date", "", "(Optional) Custom date for annotated tag")
	tagCreateCmd.Flags().StringVar(&tagEmail, "email", "", "(Annotated) Email address of tagger")
	tagCreateCmd.Flags().StringVar(&tagName, "name", "", "(Annotated) Name of tagger")
	tagCreateCmd.Flags().StringVar(&tagMsg, "message", "", "(Annotated) Text message to include")
}
