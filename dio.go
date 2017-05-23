package main // import "github.com/justinclift/dio"

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/urfave/cli"
)

//var configDir string  // TODO: Global dio default should be stored in here.  Probably set it to ~/.dio
var clientDir string // dio attributes for the current directory should be stored in here

func main() {
	app := cli.NewApp()

	app.Name = "dio"
	app.Usage = "Command line interface to DBHub.io"
	app.Version = "0.0.1"

	// Determine the current directory
	d, err := os.Getwd()
	if err != nil {
		log.Printf("Error getting current working directory: %v\n", err.Error())
		os.Exit(7)
	}

	// Set the clientDir variable
	clientDir = filepath.Join(d, ".dio")

	// Set up the commands needed for the cli
	app.Commands = []cli.Command{
		{
			Name:    "branch",
			Aliases: []string{"b"},
			Usage:   "Manipulate branches for a database",
			Subcommands: []cli.Command{
				{
					Name:    "add",
					Aliases: []string{"a"},
					Usage:   "Add a new branch",
					Action: func(c *cli.Context) error {
						err := addBranch(c)
						if err != nil {
							log.Print(err.Error())
						}
						return err
					},
				},
				{
					Name:    "remove",
					Aliases: []string{"rm, del, delete"},
					Usage:   "Remove a branch",
					Action: func(c *cli.Context) error {
						err := removeBranch(c)
						if err != nil {
							log.Print(err.Error())
						}
						return err
					},
				},
				{
					Name:    "show",
					Aliases: []string{"rm, del, delete"},
					Usage:   "Show the branches for a database",
					Action: func(c *cli.Context) error {
						err := showBranches(c)
						if err != nil {
							log.Print(err.Error())
						}
						return err
					},
				},
			},
		},
		{
			Name:  "log",
			Usage: "Show the commit history for a database",
			Action: func(c *cli.Context) error {
				err := showLog(c)
				if err != nil {
					fmt.Println(err.Error())
				}
				return err
			},
		},
		{
			Name:    "tag",
			Aliases: []string{"t"},
			Usage:   "Manipulate tags for a database",
			Subcommands: []cli.Command{
				{
					Name:    "add",
					Aliases: []string{"a"},
					Usage:   "Add a new tag",
					Action: func(c *cli.Context) error {
						err := showTags(c) // TODO: Everything for this
						if err != nil {
							log.Print(err.Error())
						}
						return err
					},
				},
				{
					Name:    "remove",
					Aliases: []string{"rm, del, delete"},
					Usage:   "Remove a tag",
					Action: func(c *cli.Context) error {
						err := showTags(c) // TODO: Everything for this
						if err != nil {
							log.Print(err.Error())
						}
						return err
					},
				},
				{
					Name:    "show",
					Aliases: []string{"s"},
					Usage:   "Show the tags for a database",
					Action: func(c *cli.Context) error {
						err := showTags(c) // TODO: Everything for this
						if err != nil {
							log.Print(err.Error())
						}
						return err
					},
				},
			},
		},
		{
			Name:    "upload",
			Aliases: []string{"up"},
			Usage:   "Upload a database",
			Action: func(c *cli.Context) error {
				err := uploadDB(c)
				if err != nil {
					log.Print(err.Error())
				}
				return err
			},
		},

		// TODO: tags (lightweight and annotated), status, branches, reversion-to-prior-commit
	}
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Run(os.Args)
	os.Exit(0)
}
