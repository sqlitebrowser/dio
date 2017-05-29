package cmd

import (
	"encoding/json"
	"log"

	rq "github.com/parnurzeal/gorequest"
)

// Retrieves the list of available databases from the remote cloud
func getDBList() []dbListEntry {
	resp, body, errs := rq.New().Get(cloud + "/db_list").End()
	if errs != nil {
		log.Print("Errors when retrieving the database list:")
		for _, err := range errs {
			log.Print(err.Error())
		}
		return []dbListEntry{}
	}
	defer resp.Body.Close()
	var list []dbListEntry
	err := json.Unmarshal([]byte(body), &list)
	if err != nil {
		log.Printf("Error retrieving database list: '%v'\n", err.Error())
		return []dbListEntry{}
	}
	return list
}
