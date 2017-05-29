package main

import "time"

type commit struct {
	AuthorEmail    string
	AuthorName     string
	CommitterEmail string
	CommitterName  string
	ID             string
	Message        string
	Parent         string
	Timestamp      time.Time
	Tree           string
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
	ID      string
	Entries []dbTreeEntry
}
type dbTreeEntry struct {
	AType         dbTreeEntryType
	Last_Modified time.Time
	Licence       string
	Sha256        string
	Size          int
	Name          string
}

const STORAGEDIR = "/Users/jc/tmp/dioapistorage"

type tagType string

const (
	SIMPLE    tagType = "simple"
	ANNOTATED         = "annotated"
)

type tagEntry struct {
	Commit      string
	Date        time.Time
	Message     string
	TagType     tagType
	TaggerEmail string
	TaggerName  string
}
