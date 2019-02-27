package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

var licenceListDisplayOrder bool

// Custom slice types, used for sorting the licences by display order
type displayOrder struct {
	order int
	key   string
}

func (p displayOrder) String() string {
	return fmt.Sprintf("Licence ID: %v, Display order: %v", p.key, p.order)
}

type displayOrderSlice []displayOrder

func (p displayOrderSlice) Len() int {
	return len(p)
}

func (p displayOrderSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p displayOrderSlice) Less(i, j int) bool {
	return p[i].order < p[j].order
}

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

		// Sort the licences by display order
		var licOrder displayOrderSlice
		for i, j := range licList {
			licOrder = append(licOrder, displayOrder{key: i, order: j.Order})
		}
		sort.Sort(displayOrderSlice(licOrder))

		// Display the licences
		for _, j := range licOrder {
			astShown := false
			if n := licList[j.key].FullName; n != "" {
				fmt.Printf("  * Full name: %s\n", n)
				astShown = true
			}

			// Include the asterisk if the Full Name line wasn't displayed
			if astShown {
				fmt.Printf("    ")
			} else {
				fmt.Printf("  * ")
				astShown = true
			}
			fmt.Printf("ID: %s\n", j.key)

			if s := licList[j.key].URL; s != "" {
				fmt.Printf("    Source URL: %s\n", s)
			}
			if licenceListDisplayOrder {
				fmt.Printf("    Display order: %d\n", licList[j.key].Order)
			}
			fmt.Printf("    SHA256: %s\n\n", licList[j.key].Sha256)
		}
		return nil
	},
}

func init() {
	licenceCmd.AddCommand(licenceListCmd)
	licenceListCmd.Flags().BoolVar(&licenceListDisplayOrder, "display-order", false,
		"Show the display order number of each licence")
}
