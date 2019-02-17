package cmd

import (
	"github.com/spf13/cobra"
)

// Displays a list of the available licences.
var licenceListCmd = &cobra.Command{
	Use:   "list",
	Short: "Displays a list of the known licences",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Retrieve the list of known licences
		licList, err := getLicences()
		if err != nil {
			return err
		}

		// Display the list of licences
		if len(licList) == 0 {
			fmt.Printf("Cloud '%s' knows no licences\n", cloud)
			return nil
		}
		fmt.Printf("Licences on %s\n\n", cloud)
		for i, j := range licList {
			fmt.Printf("  * Full name: %s\n", j.FullName)
			fmt.Printf("    ID: %s\n", i)
			fmt.Printf("    Source URL: %s\n", j.URL)
			fmt.Printf("    SHA256: %s\n\n", j.SHA256)
		}
		return nil
	},
}

func init() {
	licenceCmd.AddCommand(licenceListCmd)
}
