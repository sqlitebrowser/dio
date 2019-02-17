package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	rq "github.com/parnurzeal/gorequest"
)

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
