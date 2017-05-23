package main // import "github.com/justinclift/dio"

import (
	"fmt"
	"os"
	"sort"

	"github.com/urfave/cli"
)

func main() {
	var language string

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
				return uploadDB(c)
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Action = func(c *cli.Context) error {
		name := "someone"
		if c.NArg() > 0 {
			name = c.Args().Get(0)
		}
		if language == "spanish" {
			fmt.Println("Hola", name)
		} else {
			fmt.Println("Hello", name)
		}
		return nil
	}

	// TODO: Remember to add tags and annotated tags capability

	app.Run(os.Args)
	os.Exit(0)
}
