package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/text/message"
)

const (
	DIO_VERSION = "0.3.1"
)

var (
	certUser       string
	cfgFile, cloud string
	fOut           = io.Writer(os.Stdout)
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

	// When run from go test we skip this, as we generate a temporary config file in the test suite setup
	if os.Getenv("IS_TESTING") == "yes" {
		return
	}

	// Add the global environment variables
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		fmt.Sprintf("config file (default is %s)", filepath.Join("$HOME", ".dio", "config.toml")))
	RootCmd.PersistentFlags().StringVar(&cloud, "cloud", "https://db4s.dbhub.io",
		"Address of the DBHub.io cloud")

	// Read all of our configuration data now
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search for config in ".dio" subdirectory under the users home directory
		p := filepath.Join(home, ".dio")
		viper.AddConfigPath(p)
		viper.SetConfigName("config")
		cfgFile = filepath.Join(p, "config.toml")
	}

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		// No configuration file was found, so generate a default one and let the user know they need to supply the
		// missing info
		errInner := generateConfig(cfgFile)
		if errInner != nil {
			log.Fatalln(errInner)
			return
		}
		log.Fatalf("No usable configuration file was found, so a default one has been generated in: %s\n"+
			"Please update it with your name, and the path to your DBHub.io user certificate file.\n", cfgFile)
		return
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
		log.Fatal(err)
	}
	ok := ourCAPool.AppendCertsFromPEM(chainFile)
	if !ok {
		log.Fatal("Error when loading certificate chain file")
	}

	// TODO: Check if the client certificate file is present
	certFile := viper.GetString("certs.cert")
	if _, err = os.Stat(certFile); err != nil {
		log.Fatalf("Please download your client certificate from DBHub.io, then update the configuration "+
			"file '%s' with its path", cfgFile)
	}

	// Load a client certificate file
	cert, err := tls.LoadX509KeyPair(certFile, certFile)
	if err != nil {
		log.Fatal(err)
	}

	// Load our self signed CA Cert chain, and set TLS1.2 as minimum
	TLSConfig = tls.Config{
		Certificates:             []tls.Certificate{cert},
		ClientCAs:                ourCAPool,
		InsecureSkipVerify:       true,
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		RootCAs:                  ourCAPool,
	}

	// Extract the username and email from the TLS certificate
	var email string
	certUser, email, _, err = getUserAndServer()
	if err != nil {
		log.Fatal(err)
	}
	viper.Set("user.email", email)
}
