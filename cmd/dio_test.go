package cmd

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	chk "gopkg.in/check.v1"
)

type DioSuite struct {
	config string
	dir    string
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
	db, err := ioutil.ReadFile(filepath.Join(d, "..", "test_data", "19kB.sqlite"))
	if err != nil {
		log.Fatalln(err.Error())
	}
	dbFile := filepath.Join(s.dir, "19kB.sqlite")
	err = ioutil.WriteFile(dbFile, db, 0644)
	if err != nil {
		log.Fatalln(err.Error())
	}

	// Set the last modified date of the database file to a known value
	lastMod := time.Date(2019, time.March, 15, 18, 1, 0, 0, time.UTC)
	if err != nil {
		log.Fatalln(err.Error())
	}
	err = os.Chtimes(dbFile, time.Now(), lastMod)
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

func (s *DioSuite) TearDownSuite(c *chk.C) {
	if !*showFlag {
		err := fOut.Close()
		if err != nil {
			log.Fatalln(err)
		}
	}
}

// Test the "dio commit" command
func (s *DioSuite) TestCommit(c *chk.C) {
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
	dbName := "19kB.sqlite"
	err = commit([]string{dbName})
	if err != nil {
		log.Fatalln(err.Error())
	}
	c.Assert(err, chk.IsNil)

	// * Verify the new commit data on disk matches our expectations *

	// Check if the metadata file exists on disk
	var meta metaData
	meta, err = localFetchMetadata(dbName, false)
	if err != nil {
		log.Fatalln(err.Error())
	}
	c.Assert(err, chk.IsNil)
	c.Check(len(meta.Commits), chk.Equals, 1)

	// Verify the values in the commit data match the values we provided
	dbShaSum := "e8cab91dec32b3990b427b28380e4e052288054f99c4894742f07dee0c924efd"
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
	c.Check(com.Tree.Entries[0].Sha256, chk.Equals, dbShaSum)
	c.Check(com.Tree.Entries[0].Size, chk.Equals, 19456)
	c.Check(com.Tree.Entries[0].Name, chk.Equals, dbName)

	// Verify the branch info
	br, ok := meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "e8109ebe6d84b5fb28245e3fb1dbf852fde041abd60fc7f7f46f35128c192889")
	c.Check(br.CommitCount, chk.Equals, 1)
	c.Check(br.Description, chk.Equals, "")

	// Check the database has been written to the cache area using its checksum as filename
	// TODO: Should we read in the db file to calculate its sha256, and do the same for the cached one?
	_, err = os.Stat(filepath.Join(".dio", dbName, "db", dbShaSum))
	c.Assert(err, chk.IsNil)
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