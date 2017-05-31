package main

import "time"

type branchEntry struct {
	Commit      string `json:"commit"`
	Description string `json:"description"`
}

type commit struct {
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

type dbListEntry struct {
	Database     string    `json:"database"`
	LastModified time.Time `json:"last_modified"`
	Size         int       `json:"size"`
}

type dbTreeEntryType string

const (
	TREE     dbTreeEntryType = "tree"
	DATABASE                 = "db"
	LICENCE                  = "licence"
)

type dbTree struct {
	ID      string        `json:"id"`
	Entries []dbTreeEntry `json:"entries"`
}
type dbTreeEntry struct {
	AType         dbTreeEntryType `json:"type"`
	Last_Modified time.Time       `json:"last_modified"`
	Licence       string          `json:"licence"`
	Name          string          `json:"name"`
	Sha256        string          `json:"sha256"`
	Size          int             `json:"size"`
}

const STORAGEDIR = "/Users/jc/tmp/dioapistorage"

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
