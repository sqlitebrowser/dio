package cmd

import (
	"encoding/json"
	"errors"
	"fmt"

	rq "github.com/parnurzeal/gorequest"
)

func getLicences() (list []licenceEntry, err error) {
	resp, body, errs := rq.New().Get(cloud + "/licence_list").End()
	if errs != nil {
		e := fmt.Sprintf("Errors when retrieving the licence list:")
		for _, err := range errs {
			e += err.Error()
		}
		return list, errors.New(e)
	}
	defer resp.Body.Close()
	err = json.Unmarshal([]byte(body), &list)
	if err != nil {
		return list, errors.New(fmt.Sprintf("Error retrieving licence list: '%v'\n", err.Error()))
	}
	return list, err
}
