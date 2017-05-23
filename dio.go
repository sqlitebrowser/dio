package main // import "github.com/justinclift/dio"

import (
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()

	app.Name = "dio"
	app.Usage = "Command line interface to DBHub.io"
	app.Version = "0.0.1"

	app.Commands = []cli.Command{
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
					Usage:   "Add a new lightweight tag to a database",
					Action: func(c *cli.Context) error {
						err := showTags(c)
						if err != nil {
							log.Print(err.Error())
						}
						return err
					},
				},
				{
					Name:    "remove",
					Aliases: []string{"rm, del, delete"},
					Usage:   "Remove a lightweight tag from a database",
					Action: func(c *cli.Context) error {
						err := showTags(c)
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
