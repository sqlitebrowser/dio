package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

func createCommitID(com commit) [32]byte {
	var b bytes.Buffer
	b.WriteString("tree " + hex.EncodeToString(com.tree[:]) + "\n")
	if com.parent != NILSHA256 {
		b.WriteString("parent " + hex.EncodeToString(com.parent[:]) + "\n")
	}
	b.WriteString("author " + com.authorName + " <" + com.authorEmail + "> " +
		com.timestamp.Format(time.UnixDate) + "\n")
	if com.committerEmail != "" {
		b.WriteString("committer " + com.committerName + " <" + com.committerEmail + "> " +
			com.timestamp.Format(time.UnixDate) + "\n")
	}
	b.WriteString("\n" + com.message)
	b.WriteByte(0)
	return sha256.Sum256(b.Bytes())
}

func createDBTreeID(entries []dbTreeEntry) [32]byte {
	var b bytes.Buffer
	for _, j := range entries {
		b.WriteString(string(j.aType))
		b.WriteByte(0)
		b.WriteString(hex.EncodeToString(j.shaSum[:]))
		b.WriteByte(0)
		b.WriteString(j.name + "\n")
	}
	return sha256.Sum256(b.Bytes())
}
