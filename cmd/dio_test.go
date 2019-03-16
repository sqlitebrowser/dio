package cmd

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	chk "gopkg.in/check.v1"
)

type DioSuite struct {
	buf    bytes.Buffer
	config string
	dbFile string
	dbName string
	dir    string
	oldOut io.Writer
}

const (
	CONFIG = `[certs]
cachain = "%s"
cert = "%s"

[general]
cloud = "https://localhost:5550"

[user]
name = "Some One"
email = "someone@example.org"\n`
)

var (
	_        = chk.Suite(&DioSuite{})
	showFlag = flag.Bool("show", false, "Don't redirect test command output to /dev/null")
)

func Test(t *testing.T) {
	chk.TestingT(t)
}

func (s *DioSuite) SetUpSuite(c *chk.C) {
	// Create initial config file in a temp directory
	s.dir = c.MkDir()
	fmt.Printf("Temp dir: %s\n", s.dir)
	s.config = filepath.Join(s.dir, "config.toml")
	f, err := os.Create(s.config)
	if err != nil {
		log.Fatalln(err.Error())
	}
	d, err := os.Getwd()
	if err != nil {
		log.Fatalln(err.Error())
	}
	_, err = fmt.Fprintf(f, CONFIG,
		filepath.Join(d, "..", "test_data", "ca-chain-docker.cert.pem"),
		filepath.Join(d, "..", "test_data", "default.cert.pem"))
	if err != nil {
		log.Fatalln(err.Error())
	}
	cfgFile = s.config

	// Add test database
	s.dbName = "19kB.sqlite"
	db, err := ioutil.ReadFile(filepath.Join(d, "..", "test_data", "19kB.sqlite"))
	if err != nil {
		log.Fatalln(err.Error())
	}
	s.dbFile = filepath.Join(s.dir, s.dbName)
	err = ioutil.WriteFile(s.dbFile, db, 0644)
	if err != nil {
		log.Fatalln(err.Error())
	}

	// Set the last modified date of the database file to a known value
	err = os.Chtimes(s.dbFile, time.Now(), time.Date(2019, time.March, 15, 18, 1, 0, 0, time.UTC))
	if err != nil {
		log.Fatalln(err.Error())
	}

	// If not told otherwise, redirect command output to /dev/null
	if !*showFlag {
		fOut, err = os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func (s *DioSuite) SetUpTest(c *chk.C) {
	// TODO: Should we use io.TeeReader if showFlag has been set?
	// Redirect display output to a temp buffer
	s.oldOut = fOut
	fOut = &s.buf
}

func (s *DioSuite) TearDownTest(c *chk.C) {
	// Restore the display output redirection
	fOut = s.oldOut

	// Clear the buffered contents
	s.buf.Reset()
}

// Test the "dio commit" command
func (s *DioSuite) Test0010Commit(c *chk.C) {
	// Set up the replacement functions
	getDatabases = mockGetDatabases
	getLicences = mockGetLicences

	// Change to the temp directory
	err := os.Chdir(s.dir)
	if err != nil {
		log.Fatalln(err.Error())
	}

	// Call the commit code
	commitCmdBranch = "master"
	commitCmdCommit = ""
	commitCmdAuthEmail = "testdefault@dbhub.io"
	commitCmdLicence = "Not specified"
	commitCmdMsg = "The first commit in our test run"
	commitCmdAuthName = "Default test user"
	commitCmdTimestamp = "2019-03-15T18:01:01Z"
	// TODO: Adjust commit() to return the commit ID, so we don't need to hard code it below
	err = commit([]string{s.dbName})
	if err != nil {
		log.Fatalln(err.Error())
	}
	c.Assert(err, chk.IsNil)

	// * Verify the new commit data on disk matches our expectations *

	// Check if the metadata file exists on disk
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	c.Check(meta.Commits, chk.HasLen, 1)

	// Verify the values in the commit data match the values we provided
	com, ok := meta.Commits["e8109ebe6d84b5fb28245e3fb1dbf852fde041abd60fc7f7f46f35128c192889"] // This commit ID is what the given values should generate a commit ID as
	c.Assert(ok, chk.Equals, true)
	c.Check(com.AuthorName, chk.Equals, commitCmdAuthName)
	c.Check(com.AuthorEmail, chk.Equals, commitCmdAuthEmail)
	c.Check(com.Message, chk.Equals, commitCmdMsg)
	c.Check(com.Timestamp, chk.Equals, time.Date(2019, time.March, 15, 18, 1, 1, 0, time.UTC))
	c.Check(com.Parent, chk.Equals, "")
	c.Check(com.OtherParents, chk.IsNil)
	c.Check(com.CommitterName, chk.Equals, "Some One")
	c.Check(com.CommitterEmail, chk.Equals, "someone@example.org")
	c.Check(com.ID, chk.Equals, "e8109ebe6d84b5fb28245e3fb1dbf852fde041abd60fc7f7f46f35128c192889")
	c.Check(com.Tree.Entries[0].EntryType, chk.Equals, dbTreeEntryType(DATABASE))
	c.Check(com.Tree.Entries[0].LastModified.UTC(), chk.Equals, time.Date(2019, time.March, 15, 18, 1, 0, 0, time.UTC))
	c.Check(com.Tree.Entries[0].LicenceSHA, chk.Equals, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855") // e3b... is the SHA256 for the "Not specified" licence option
	c.Check(com.Tree.Entries[0].Size, chk.Equals, 19456)
	c.Check(com.Tree.Entries[0].Name, chk.Equals, s.dbName)

	// Check the database has been written to the cache area using its checksum as filename
	cacheFile := filepath.Join(".dio", s.dbName, "db", com.Tree.Entries[0].Sha256)
	_, err = os.Stat(cacheFile)
	c.Assert(err, chk.IsNil)

	// Verify the contents of the cached database match the size and sha256 recorded in the commit
	b, err := ioutil.ReadFile(cacheFile)
	c.Assert(err, chk.IsNil)
	c.Check(b, chk.HasLen, com.Tree.Entries[0].Size)
	z := sha256.Sum256(b)
	shaSum := hex.EncodeToString(z[:])
	c.Check(shaSum, chk.Equals, com.Tree.Entries[0].Sha256)

	// Verify the branch info
	br, ok := meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "e8109ebe6d84b5fb28245e3fb1dbf852fde041abd60fc7f7f46f35128c192889")
	c.Check(br.CommitCount, chk.Equals, 1)
	c.Check(br.Description, chk.Equals, "")
}

func (s *DioSuite) Test0020Commit2(c *chk.C) {
	// Change the last modified date on the database file
	err := os.Chtimes(s.dbFile, time.Now(), time.Date(2019, time.March, 15, 18, 1, 2, 0, time.UTC))
	if err != nil {
		log.Fatalln(err.Error())
	}

	// Create another commit
	commitCmdMsg = "The second commit in our test run"
	commitCmdTimestamp = "2019-03-15T18:01:03Z"
	err = commit([]string{s.dbName})
	if err != nil {
		log.Fatalln(err.Error())
	}
	c.Assert(err, chk.IsNil)

	// * Verify the new commit data on disk matches our expectations *

	// Check if the metadata file exists on disk
	var meta metaData
	meta, err = localFetchMetadata(s.dbName, false)
	if err != nil {
		log.Fatalln(err.Error())
	}
	c.Assert(err, chk.IsNil)
	c.Check(meta.Commits, chk.HasLen, 2)

	// Verify the values in the commit data match the values we provided
	com, ok := meta.Commits["09d05ae9a69e82be44f61ac22cb7e3fcd15a0783973c283fd723e3228bd6c9da"] // This commit ID is what the given values should generate a commit ID as
	c.Assert(ok, chk.Equals, true)
	c.Check(com.AuthorName, chk.Equals, commitCmdAuthName)
	c.Check(com.AuthorEmail, chk.Equals, commitCmdAuthEmail)
	c.Check(com.Message, chk.Equals, commitCmdMsg)
	c.Check(com.Timestamp, chk.Equals, time.Date(2019, time.March, 15, 18, 1, 3, 0, time.UTC))
	c.Check(com.Parent, chk.Equals, "e8109ebe6d84b5fb28245e3fb1dbf852fde041abd60fc7f7f46f35128c192889")
	c.Check(com.OtherParents, chk.IsNil)
	c.Check(com.CommitterName, chk.Equals, "Some One")
	c.Check(com.CommitterEmail, chk.Equals, "someone@example.org")
	c.Check(com.ID, chk.Equals, "09d05ae9a69e82be44f61ac22cb7e3fcd15a0783973c283fd723e3228bd6c9da")
	c.Check(com.Tree.Entries[0].EntryType, chk.Equals, dbTreeEntryType(DATABASE))
	c.Check(com.Tree.Entries[0].LastModified.UTC(), chk.Equals, time.Date(2019, time.March, 15, 18, 1, 2, 0, time.UTC))
	c.Check(com.Tree.Entries[0].LicenceSHA, chk.Equals, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855") // e3b... is the SHA256 for the "Not specified" licence option
	c.Check(com.Tree.Entries[0].Size, chk.Equals, 19456)
	c.Check(com.Tree.Entries[0].Name, chk.Equals, s.dbName)

	// Check the database has been written to the cache area using its checksum as filename
	cacheFile := filepath.Join(".dio", s.dbName, "db", com.Tree.Entries[0].Sha256)
	_, err = os.Stat(cacheFile)
	c.Assert(err, chk.IsNil)

	// Verify the contents of the cached database match the size and sha256 recorded in the commit
	b, err := ioutil.ReadFile(cacheFile)
	c.Assert(err, chk.IsNil)
	c.Check(b, chk.HasLen, com.Tree.Entries[0].Size)
	z := sha256.Sum256(b)
	shaSum := hex.EncodeToString(z[:])
	c.Check(shaSum, chk.Equals, com.Tree.Entries[0].Sha256)

	// Verify the branch info
	br, ok := meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "09d05ae9a69e82be44f61ac22cb7e3fcd15a0783973c283fd723e3228bd6c9da")
	c.Check(br.CommitCount, chk.Equals, 2)
	c.Check(br.Description, chk.Equals, "")

	// TODO: Now that we can capture the display output for checking, we should probably verify the
	//       info displayed in the output too
}

// Test the "dio branch" commands
func (s *DioSuite) Test0030BranchActiveGet(c *chk.C) {
	// Query the active branch
	err := branchActiveGet([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the active branch is set to "master"
	p := strings.Split(s.buf.String(), ":")
	c.Assert(p, chk.HasLen, 2)
	c.Check(strings.TrimSpace(p[1]), chk.Equals, "master")
}

func (s *DioSuite) Test0040BranchCreate(c *chk.C) {
	// Create a new branch
	branchCreateBranch = "branchtwo"
	branchCreateCommit = "e8109ebe6d84b5fb28245e3fb1dbf852fde041abd60fc7f7f46f35128c192889"
	branchCreateMsg = "A new branch"
	err := branchCreate([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the new branch is in the metadata on disk
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok := meta.Branches["branchtwo"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "e8109ebe6d84b5fb28245e3fb1dbf852fde041abd60fc7f7f46f35128c192889")
	c.Check(br.CommitCount, chk.Equals, 1)
	c.Check(br.Description, chk.Equals, "A new branch")

	// Verify the output given to the user
	p := strings.Split(s.buf.String(), "'")
	c.Assert(p, chk.HasLen, 3)
	c.Check(strings.TrimSpace(p[1]), chk.Equals, branchCreateBranch)
}

func (s *DioSuite) Test0050BranchSet(c *chk.C) {
	// Create a new branch
	branchActiveSetBranch = "branchtwo"
	err := branchActiveSet([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the active branch was changed in the on disk metadata
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	c.Check(meta.ActiveBranch, chk.Equals, branchActiveSetBranch)

	// Verify the output given to the user
	p := strings.Split(s.buf.String(), "'")
	c.Check(strings.TrimSpace(p[1]), chk.Equals, branchCreateBranch)
}

func (s *DioSuite) Test0060BranchList(c *chk.C) {
	// Create a new branch
	err := branchList([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify entries are present for both "master" and "branchtwo"
	lines := bufio.NewScanner(&s.buf)
	var branchTwoFound, masterFound bool
	for lines.Scan() {
		p := strings.Split(lines.Text(), "'")
		if len(p) > 2 && p[1] == "master" {
			c.Assert(p, chk.HasLen, 3)
			masterFound = true
		}
		if len(p) > 2 && p[1] == "branchtwo" {
			c.Assert(p, chk.HasLen, 3)
			branchTwoFound = true
		}
	}
	c.Check(masterFound, chk.Equals, true)
	c.Check(branchTwoFound, chk.Equals, true)
}

// Mocked functions
func mockGetDatabases(url string, user string) (dbList []dbListEntry, err error) {
	dbList = append(dbList, dbListEntry{
		CommitID:     "316b246eda1e1779b21e9ac338cab4a71847c5268c03911ebfed974ffbab03bc",
		DefBranch:    "master",
		LastModified: "12 Mar 19 13:56 AEDT",
		Licence:      "Not specified",
		Name:         "2.5mbv13.sqlite",
		OneLineDesc:  "",
		Public:       true,
		RepoModified: "12 Mar 19 13:59 AEDT",
		SHA256:       "SHA256",
		Size:         2666496,
		Type:         "database",
		URL:          fmt.Sprintf("%s/default/%s", cloud, "2.5mbv13.sqlite?commit=316b246eda1e1779b21e9ac338cab4a71847c5268c03911ebfed974ffbab03bc&branch=master"),
	})
	return
}

func mockGetLicences() (list map[string]licenceEntry, err error) {
	list = make(map[string]licenceEntry)
	list["Not specified"] = licenceEntry{
		FileFormat: "text",
		FullName:   "No licence specified",
		Order:      100,
		Sha256:     "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		URL:        "",
	}
	return
}
