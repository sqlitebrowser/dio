package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	branch, cfgFile, cloud, commit, email, name, msg, tag string
	TLSConfig                                             tls.Config
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
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.dio/config.yaml)")
	RootCmd.PersistentFlags().StringVar(&cloud, "cloud", "https://docker-dev.dbhub.io:5550",
		"Address of the DBHub.io cloud")

	// Read our certificate info, if present
	// TODO: Read the certificate from a proper location
	ourCAPool := x509.NewCertPool()
	chainFile, err := ioutil.ReadFile("/home/jc/git_repos/src/github.com/sqlitebrowser/dbhub.io/docker/certs/ca-chain-docker.cert.pem")
	if err != nil {
		fmt.Println(err)
	}
	ok := ourCAPool.AppendCertsFromPEM(chainFile)
	if !ok {
		fmt.Println("Error when loading certificate chain file")
	}

	// Load a client certificate file
	// TODO: Read the certificate from a proper location
	cert, err := tls.LoadX509KeyPair("/home/jc/default.cert.pem", "/home/jc/default.cert.pem")
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
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
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

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Error reading config file:", viper.ConfigFileUsed())
	}
}
