package main // import "github.com/justinclift/dio"

import (
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
		{
			Name:  "log",
			Usage: "Show the commit history for a database",
			Action: func(c *cli.Context) error {
				err := showLog(c)
				if err != nil {
					log.Print(err.Error())
				}
				return err
			},
		},
	}

	sort.Sort(cli.CommandsByName(app.Commands))

	// TODO: Remember to add tags and annotated tags capability

	app.Run(os.Args)
	os.Exit(0)
}
