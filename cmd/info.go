package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Displays useful information about the dio installation
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Displays useful information about the dio installation",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Dio version %s\n", DIO_VERSION)

		// Display the path to the dio configuration file
		if confPath := viper.ConfigFileUsed(); confPath != "" {
			fmt.Println("Configuration file used:", confPath)
		}

		fmt.Printf("\n** Connection **\n\n")

		// Display the connection URL used for DBHUB.io
		if found := viper.IsSet("general.cloud"); found == true {
			fmt.Printf("DBHub.io connection URL: %s\n", viper.Get("general.cloud"))
		} else {
			fmt.Println("No custom DBHub.io connection URL is set")
		}

		// Display the path to our CA Chain and user certificate
		if found := viper.IsSet("certs.cachain"); found == true {
			fmt.Printf("Path to CA chain file: %s\n", viper.Get("certs.cachain"))
		} else {
			fmt.Println("Path to CA chain not set in configuration file")
		}
		if found := viper.IsSet("certs.cert"); found == true {
			fmt.Printf("Path to user certificate file: %s\n", viper.Get("certs.cert"))
		} else {
			fmt.Println("Path to user certificate not set in configuration file")
		}

		// TODO: Maybe display the user name, server, and expiry date from the cert file?

		fmt.Printf("\n** Commit defaults **\n\n")

		// Display the user name and email address used for commits
		if found := viper.IsSet("user.name"); found == true {
			fmt.Printf("User name for commits: %s\n", viper.Get("user.name"))
		} else {
			fmt.Println("User name not set in configuration file")
		}
		if found := viper.IsSet("user.email"); found == true {
			fmt.Printf("Email address for commits: %s\n", viper.Get("user.email"))
		} else {
			fmt.Println("Email address not set in configuration file")
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(infoCmd)
}
