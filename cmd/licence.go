package cmd

import (
	"github.com/spf13/cobra"
)

// licenceCmd represents the licence command
var licenceCmd = &cobra.Command{
	Use:   "licence",
	Short: "List, retrieve, and update licences on DBHub.io",
	Long: `List, retrieve, and update licences on DBHub.io

The special word 'all' can be used with 'get' for retrieving all licences.`,
	Example: `
  $ dio licence get CC0
  Downloading licences...

  * CC0: Licence 'CC0.txt' downloaded

  Completed

  $ dio licence get all
  Downloading licences...

    * CC-BY-NC-4.0: Licence 'CC-BY-NC-4.0.txt' downloaded
    * CC-BY-SA-4.0: Licence 'CC-BY-SA-4.0.txt' downloaded
    * CC0: Licence 'CC0.txt' downloaded
    * ODbL-1.0: Licence 'ODbL-1.0.txt' downloaded
    * UK-OGL-3: Licence 'UK-OGL-3.html' downloaded
    * CC-BY-4.0: Licence 'CC-BY-4.0.txt' downloaded
    * CC-BY-IGO-3.0: Licence 'CC-BY-IGO-3.0.html' downloaded

  Completed`,
}

func init() {
	RootCmd.AddCommand(licenceCmd)
}
