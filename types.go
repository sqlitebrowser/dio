package main

import "time"

type branch struct {
	Commit string
	Name   string
}

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

type DBTreeEntryType string

const (
	TREE     DBTreeEntryType = "tree"
	DATABASE                 = "db"
	LICENCE                  = "licence"
)

type dbTree struct {
	ID      string
	Entries []dbTreeEntry
}
type dbTreeEntry struct {
	AType   DBTreeEntryType
	Licence string
	ShaSum  string
	Name    string
}

const STORAGEDIR = "/Users/jc/tmp/diostorage"
