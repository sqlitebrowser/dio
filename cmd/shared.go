package cmd

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	rq "github.com/parnurzeal/gorequest"
)

// Returns a map with the list of licences available on the remote server
func getLicences() (list map[string]licenceEntry, err error) {
	// Retrieve the database list from the cloud
	resp, body, errs := rq.New().TLSClientConfig(&TLSConfig).Get(cloud + "/licence/list").End()
	if errs != nil {
		e := fmt.Sprintln("errors when retrieving the licence list:")
		for _, err := range errs {
			e += fmt.Sprintf(err.Error())
		}
		return list, errors.New(e)
	}
	defer resp.Body.Close()

	// Convert the JSON response to our licence entry structure
	err = json.Unmarshal([]byte(body), &list)
	if err != nil {
		return list, errors.New(fmt.Sprintf("error retrieving licence list: '%v'\n", err.Error()))
	}
	return list, err
}

// getUserAndServer() returns the user name and server from a DBHub.io client certificate
func getUserAndServer() (userAcc string, certServer string, err error) {
	if numCerts := len(TLSConfig.Certificates); numCerts == 0 {
		err = errors.New("No client certificates installed.  Can't proceed.")
		return
	}

	// Parse the client certificate
	// TODO: Add support for multiple certificates
	cert, err := x509.ParseCertificate(TLSConfig.Certificates[0].Certificate[0])
	if err != nil {
		err = errors.New("Couldn't parse cert")
		return
	}

	// Extract the account name and associated server from the certificate
	cn := cert.Subject.CommonName
	if cn == "" {
		// The common name field is empty in the client cert.  Can't proceed.
		err = errors.New("Common name is blank in client certificate")
		return
	}
	s := strings.Split(cn, "@")
	if len(s) < 2 {
		err = errors.New("Missing information in client certificate")
		return
	}
	userAcc = s[0]
	certServer = s[1]
	if userAcc == "" || certServer == "" {
		// Missing details in common name field
		err = errors.New("Missing information in client certificate")
		return
	}

	return
}

// Retrieves database metadata from DBHub.io
func retrieveMetadata(db string) (md string, err error) {
	// Download the database metadata
	resp, md, errs := rq.New().TLSClientConfig(&TLSConfig).Get(cloud + "/metadata/get").
		Query(fmt.Sprintf("username=%s", url.QueryEscape(certUser))).
		Query(fmt.Sprintf("folder=%s", "/")).
		Query(fmt.Sprintf("dbname=%s", url.QueryEscape(db))).
		End()

	if errs != nil {
		log.Print("Errors when downloading database metadata:")
		for _, err := range errs {
			log.Print(err.Error())
		}
		return "", errors.New("Error when downloading database metadata")
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New(fmt.Sprintf("Metadata download failed with an error: HTTP status %d - '%v'\n",
			resp.StatusCode, resp.Status))
	}
	return md, nil
}

// Saves metadata to the local cache
func updateMetadata(db string) error {
	// Create a folder to hold metadata, if it doesn't yet exist
	if _, err := os.Stat(filepath.Join(".dio", db)); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Join(".dio", db), 0770)
		if err != nil {
			return err
		}
	}

	// Download the database metadata
	md, err := retrieveMetadata(db)
	if err != nil {
		return err
	}

	// Write the metadata file to disk
	mdFile := filepath.Join(".dio", db, "metadata.json")
	err = ioutil.WriteFile(mdFile, []byte(md), 0644)
	if err != nil {
		return err
	}

	return nil
}
