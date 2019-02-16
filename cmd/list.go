package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"

	rq "github.com/parnurzeal/gorequest"
	"github.com/spf13/cobra"
)

// Displays a list of the databases on the DBHub.io server.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Returns a list of available databases",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: In the real code, we'd likely include things like # stars and fork count too

		// Load our self signed CA chain
		ourCAPool := x509.NewCertPool()
		chainFile, err := ioutil.ReadFile("/home/jc/git_repos/src/github.com/sqlitebrowser/dbhub.io/docker/certs/ca-chain-docker.cert.pem")
		//chainFile, err := ioutil.ReadFile(com.Conf.DB4S.CAChain)
		if err != nil {
			fmt.Printf("Error opening Certificate Authority chain file: %v\n", err)
			return err
		}
		ok := ourCAPool.AppendCertsFromPEM(chainFile)
		if !ok {
			fmt.Println("Error appending certificate file")
			return errors.New("error appending certificate file")
		}

		// Load a client certificate file
		certFile, err := ioutil.ReadFile("/home/jc/default.cert.pem")
		if err != nil {
			fmt.Printf("Error opening Certificate Authority chain file: %v\n", err)
			return err
		}

		//// Decode the client certificate (PEM format)
		cert, _ := pem.Decode(certFile)
		if cert == nil || cert.Type != "CERTIFICATE" {
			return errors.New("file didn't contain a certificate")
		}

		//fmt.Printf("PEM Block found of type: %v\n", cert.Type)

		//_, err = x509.ParseCertificate(cert.Bytes)
		//foo, err := x509.ParseCertificate(certFile)
		foo, err := x509.ParseCertificate(cert.Bytes)
		if err != nil {
			return err
		}

		var userCert []tls.Certificate

		var oneCert tls.Certificate
		oneCert.Leaf = foo

		//userCert = append(userCert, &foo)

		// Load our self signed CA Cert chain, and set TLS1.2 as minimum
		newTLSConfig := &tls.Config{
			//ClientAuth:               tls.RequireAndVerifyClientCert,
			Certificates:             userCert,
			ClientCAs:                ourCAPool,
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			RootCAs:                  ourCAPool,
		}

		fmt.Println("Sending request...")

		resp, _, errs := rq.New().
		//resp, body, errs := rq.New().
			TLSClientConfig(newTLSConfig).
			//TLSClientConfig(&tls.Config{ InsecureSkipVerify: true}).
			Get(cloud + "/dbist").
			End()
		if errs != nil {
			e := fmt.Sprintln("Errors when retrieving the database list:")
			for _, err := range errs {
				e += fmt.Sprintf(err.Error())
			}
			return errors.New(e)
		}
		defer resp.Body.Close()

		//// Retrieve the database list from the cloud
		//resp, body, errs := rq.New().Get(cloud + "/db_list").End()
		//if errs != nil {
		//	e := fmt.Sprintln("Errors when retrieving the database list:")
		//	for _, err := range errs {
		//		e += fmt.Sprintf(err.Error())
		//	}
		//	return errors.New(e)
		//}
		//defer resp.Body.Close()
		//var list []dbListEntry
		//err := json.Unmarshal([]byte(body), &list)
		//if err != nil {
		//	log.Printf("Error retrieving database list: '%v'\n", err.Error())
		//	return err
		//}
		//
		//// Display the list of databases
		//if len(list) == 0 {
		//	fmt.Printf("Cloud '%s' has no databases\n", cloud)
		//	return nil
		//}
		//fmt.Printf("Databases on %s\n\n", cloud)
		//for _, j := range list {
		//	fmt.Printf("  * Database: %s\n\n", j.Database)
		//	fmt.Printf("      Size: %d bytes\n", j.Size)
		//	if j.Licence != "" {
		//		fmt.Printf("      Licence: %s\n", j.Licence)
		//	} else {
		//		fmt.Println("      Licence: Not specified")
		//	}
		//	fmt.Printf("      Last Modified: %s\n\n", j.LastModified)
		//}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}
