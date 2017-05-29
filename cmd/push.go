package cmd

import (
	"fmt"
	"log"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Upload a database",
	Run: func(cmd *cobra.Command, args []string) {

		// TODO: Also send the last modified timestamp for the file, so that can be stored on the remote end too

		// Send the file
		resp, _, errs := rq.New().Post(cloud+"/db_upload").
			Type("multipart").
			Set("Name", "a.db").
			Set("Branch", "master").
			SendFile("/Users/jc/Databases/c.db").
			End()
		if errs != nil {
			log.Print("Errors when sending the database to the cloud:")
			for _, err := range errs {
				log.Print(err.Error())
			}
		}

		if resp.StatusCode != 201 {
			fmt.Printf("Cloud responded with an error: HTTP status %d - '%v'\n", resp.StatusCode,
				resp.Status)
		}
		fmt.Printf("%s - Database upload succeessful\n", cloud)
		//fmt.Printf("Database upload succeeded.  Name: %s, size: %d, branch: %s\n")
	},
}

func init() {
	RootCmd.AddCommand(pushCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// pushCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// pushCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
