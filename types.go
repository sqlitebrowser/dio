package main

import "time"

type branch struct {
	Commit [32]byte
	Name   string
}

type commit struct {
	AuthorEmail    string
	AuthorName     string
	CommitterEmail string
	CommitterName  string
	ID             [32]byte
	Message        string
	Parent         [32]byte
	Timestamp      time.Time
	Tree           [32]byte
}

type DBTreeEntryType string

const (
	TREE     DBTreeEntryType = "tree"
	DATABASE                 = "db"
	LICENCE                  = "licence"
)

type dbTree struct {
	ID      [32]byte
	Entries []dbTreeEntry
}
type dbTreeEntry struct {
	AType   DBTreeEntryType
	Licence [32]byte
	ShaSum  [32]byte
	Name    string
}

var NILSHA256 = [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

const STORAGEDIR = "/Users/jc/tmp/newdatamodel"
