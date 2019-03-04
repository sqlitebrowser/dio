package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/text/message"
)

var (
	cfgFile, cloud string
	certUser       string
	numFormat      *message.Printer
	TLSConfig      tls.Config
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "dio",
	Short: "Command line interface to DBHub.io",
	Long: `dio is a command line interface (CLI) for DBHub.io.

With dio you can send and receive database files to a DBHub.io cloud,
and manipulate its tags and branches.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute adds all child commands to the root command & sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Add support for pretty printing numbers
	numFormat = message.NewPrinter(message.MatchLanguage("en"))

	// Add the global environment variables
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.dio/config.toml)")
	RootCmd.PersistentFlags().StringVar(&cloud, "cloud", "https://dbhub.io:5550",
		"Address of the DBHub.io cloud")

	// Read all of our configuration data now
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".dio" (without extension).
		viper.AddConfigPath(filepath.Join(home, ".dio"))
		viper.SetConfigName("config")
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Error reading config file:", viper.ConfigFileUsed())
	}

	// Make sure the paths to our CA Chain and user certificate have been set
	if found := viper.IsSet("certs.cachain"); found == false {
		log.Fatal("Path to Certificate Authority chain file not set in the config file")
		return
	}
	if found := viper.IsSet("certs.cert"); found == false {
		log.Fatal("Path to user certificate file not set in the config file")
		return
	}

	// If an alternative DBHub.io cloud address is set in the config file, use that
	if found := viper.IsSet("general.cloud"); found == true {
		// If the user provided an override on the command line, that will override this anyway
		cloud = viper.GetString("general.cloud")
	}

	// Read our certificate info, if present
	ourCAPool := x509.NewCertPool()
	chainFile, err := ioutil.ReadFile(viper.GetString("certs.cachain"))
	if err != nil {
		fmt.Println(err)
	}
	ok := ourCAPool.AppendCertsFromPEM(chainFile)
	if !ok {
		fmt.Println("Error when loading certificate chain file")
	}

	// Load a client certificate file
	cert, err := tls.LoadX509KeyPair(viper.GetString("certs.cert"), viper.GetString("certs.cert"))
	if err != nil {
		fmt.Println(err)
	}

	// Load our self signed CA Cert chain, and set TLS1.2 as minimum
	TLSConfig = tls.Config{
		Certificates:             []tls.Certificate{cert},
		ClientCAs:                ourCAPool,
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		RootCAs:                  ourCAPool,
	}

	// Extract the username and server from the TLS certificate
	certUser, _, err = getUserAndServer()
	if err != nil {
		fmt.Println(err)
	}
}
