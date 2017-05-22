package main

import "time"

type branch struct {
	commit [32]byte
	name   string
}

type commit struct {
	authorEmail    string
	authorName     string
	committerEmail string
	committerName  string
	id             [32]byte
	message        string
	parent         [32]byte
	timestamp      time.Time
	tree           [32]byte
}

type DBTreeEntryType string

const (
	TREE     DBTreeEntryType = "tree"
	DATABASE                 = "db"
	LICENCE                  = "licence"
)

type dbTree struct {
	id      [32]byte
	entries []dbTreeEntry
}
type dbTreeEntry struct {
	aType   DBTreeEntryType
	licence [32]byte
	shaSum  [32]byte
	name    string
}

var NILSHA256 = [32]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

const STORAGEDIR = "/Users/jc/tmp/newdatamodel"
