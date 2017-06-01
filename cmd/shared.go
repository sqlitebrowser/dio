package cmd

import (
	"encoding/json"
	"log"
	"time"

	rq "github.com/parnurzeal/gorequest"
)

type branchEntry struct {
	Commit      string `json:"commit"`
	Description string `json:"description"`
}

type commitEntry struct {
	AuthorEmail    string    `json:"author_email"`
	AuthorName     string    `json:"author_name"`
	CommitterEmail string    `json:"committer_email"`
	CommitterName  string    `json:"committer_name"`
	ID             string    `json:"id"`
	Message        string    `json:"message"`
	Parent         string    `json:"parent"`
	Timestamp      time.Time `json:"timestamp"`
	Tree           string    `json:"tree"`
}

var errorInfo struct {
	Condition string   `json:"error_condition"`
	Data      []string `json:"data"`
}

type tagType string

const (
	SIMPLE    tagType = "simple"
	ANNOTATED         = "annotated"
)

type tagEntry struct {
	Commit      string    `json:"commit"`
	Date        time.Time `json:"date"`
	Message     string    `json:"message"`
	TagType     tagType   `json:"type"`
	TaggerEmail string    `json:"email"`
	TaggerName  string    `json:"name"`
}

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
