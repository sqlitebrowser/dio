package cmd

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	chk "gopkg.in/check.v1"
)

type DioSuite struct {
	buf    bytes.Buffer
	config string
	dbFile string
	dbName string
	oldOut io.Writer
}

const (
	CONFIG = `[certs]
cachain = "%s"
cert = "%s"

[general]
cloud = "%s"

[user]
name = "Some One"
email = "someone@example.org"
`
)

var (
	_       = chk.Suite(&DioSuite{})
	licFile string
	licList = map[string]licenceEntry{"Not specified": {
		FileFormat: "text",
		FullName:   "No licence specified",
		Order:      100,
		Sha256:     "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		URL:        "",
	}}
	newServer     *http.Server
	origDir       string
	remoteServer  = flag.String("remote", "", "URL of remote server to test against, instead of using mock server")
	showFlag      = flag.Bool("show", false, "Don't redirect test command output to /dev/null")
	tempDir       string
	mockDBEntries = []dbListEntry{{
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
	}}
	mockMetaData = map[string]metaData{}
)

func Test(t *testing.T) {
	chk.TestingT(t)
}

func (s *DioSuite) SetUpSuite(c *chk.C) {
	// Create initial config file in a temp directory
	tempDir = c.MkDir()
	fmt.Printf("Temp dir: %s\n", tempDir)
	s.config = filepath.Join(tempDir, "config.toml")
	f, err := os.Create(s.config)
	if err != nil {
		log.Fatalln(err)
	}
	d, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}
	origDir = d
	mockAddr := "https://localhost:5551"
	if *remoteServer != "" {
		mockAddr = *remoteServer
	}
	_, err = fmt.Fprintf(f, CONFIG,
		filepath.Join(d, "..", "test_data", "ca-chain-docker.cert.pem"),
		filepath.Join(d, "..", "test_data", "default.cert.pem"),
		mockAddr)
	if err != nil {
		log.Fatalln(err)
	}

	// Drop any old config loaded automatically by viper, and use our temporary test config instead
	viper.Reset()
	viper.SetConfigFile(s.config)
	if err = viper.ReadInConfig(); err != nil {
		log.Fatalf("Error loading test config file: %s", err)
		return
	}
	cloud = viper.GetString("general.cloud")

	// Use our testing certificates
	ourCAPool := x509.NewCertPool()
	chainFile, err := ioutil.ReadFile(filepath.Join(d, "..", "test_data", "ca-chain-docker.cert.pem"))
	if err != nil {
		log.Fatalln(err)
	}
	ok := ourCAPool.AppendCertsFromPEM(chainFile)
	if !ok {
		log.Fatalln("Error when loading certificate chain file")
	}
	testCert := filepath.Join(d, "..", "test_data", "default.cert.pem")
	cert, err := tls.LoadX509KeyPair(testCert, testCert)
	if err != nil {
		log.Fatalln(err)
	}
	TLSConfig.Certificates = []tls.Certificate{cert}
	certUser, _, err = getUserAndServer()
	if err != nil {
		log.Fatalln(err)
	}

	// Add test database
	s.dbName = "19kB.sqlite"
	db, err := ioutil.ReadFile(filepath.Join(d, "..", "test_data", s.dbName))
	if err != nil {
		log.Fatalln(err)
	}
	s.dbFile = filepath.Join(tempDir, s.dbName)
	err = ioutil.WriteFile(s.dbFile, db, 0644)
	if err != nil {
		log.Fatalln(err)
	}

	// Set the last modified date of the database file to a known value
	err = os.Chtimes(s.dbFile, time.Now(), time.Date(2019, time.March, 15, 18, 1, 0, 0, time.UTC))
	if err != nil {
		log.Fatalln(err)
	}

	// Add a test licence
	lic, err := ioutil.ReadFile(filepath.Join(d, "..", "LICENSE"))
	if err != nil {
		log.Fatalln(err)
	}
	licFile = filepath.Join(tempDir, "test.licence")
	err = ioutil.WriteFile(licFile, lic, 0644)
	if err != nil {
		log.Fatalln(err)
	}

	// If not told otherwise, redirect command output to /dev/null
	if !*showFlag {
		fOut, err = os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalln(err)
		}
	}

	// If we're not testing against a remote server, use our mock pieces
	if *remoteServer == "" {
		// Set up the replacement functions
		getLicences = mockGetLicences

		// Start our mock https server
		go mockServer()
		time.Sleep(250 * time.Millisecond) // Small pause here, to allow the mock server to finish starting
	}

	// Change to the temp directory
	err = os.Chdir(tempDir)
	c.Assert(err, chk.IsNil)
}

func (s *DioSuite) SetUpTest(c *chk.C) {
	// TODO: Should we use io.TeeReader if showFlag has been set?
	// Redirect display output to a temp buffer
	s.oldOut = fOut
	fOut = &s.buf
}

func (s *DioSuite) TearDownSuite(c *chk.C) {
	if *remoteServer == "" {
		// Shut down the mock server
		_ = newServer.Close()
	}
}

func (s *DioSuite) TearDownTest(c *chk.C) {
	// Restore the display output redirection
	fOut = s.oldOut

	// Clear the buffered contents
	s.buf.Reset()
}

// Test the "dio commit" command
func (s *DioSuite) Test0010_Commit(c *chk.C) {
	// Call the commit code
	commitCmdBranch = "master"
	commitCmdCommit = ""
	commitCmdAuthEmail = "testdefault@dbhub.io"
	commitCmdLicence = "Not specified"
	commitCmdMsg = "The first commit in our test run"
	commitCmdAuthName = "Default test user"
	commitCmdTimestamp = time.Date(2019, time.March, 15, 18, 1, 1, 0, time.UTC).Format(time.RFC3339)
	// TODO: Adjust commit() to return the commit ID, so we don't need to hard code it below
	err := commit([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// * Verify the new commit data on disk matches our expectations *

	// Check if the metadata file exists on disk
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	c.Check(meta.Commits, chk.HasLen, 1)

	// Verify the values in the commit data match the values we provided
	com, ok := meta.Commits["485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb"] // This commit ID is what the given values should generate a commit ID as
	c.Assert(ok, chk.Equals, true)
	c.Check(com.AuthorName, chk.Equals, commitCmdAuthName)
	c.Check(com.AuthorEmail, chk.Equals, commitCmdAuthEmail)
	c.Check(com.Message, chk.Equals, commitCmdMsg)
	c.Check(com.Timestamp, chk.Equals, time.Date(2019, time.March, 15, 18, 1, 1, 0, time.UTC))
	c.Check(com.Parent, chk.Equals, "")
	c.Check(com.OtherParents, chk.IsNil)
	c.Check(com.CommitterName, chk.Equals, "Some One")
	c.Check(com.CommitterEmail, chk.Equals, "someone@example.org")
	c.Check(com.ID, chk.Equals, "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb")
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
	c.Check(br.Commit, chk.Equals, "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb")
	c.Check(br.CommitCount, chk.Equals, 1)
	c.Check(br.Description, chk.Equals, "")
}

func (s *DioSuite) Test0020_Commit2(c *chk.C) {
	// Change the last modified date on the database file
	err := os.Chtimes(s.dbFile, time.Now(), time.Date(2019, time.March, 15, 18, 1, 2, 0, time.UTC))
	c.Assert(err, chk.IsNil)

	// Create another commit
	commitCmdMsg = "The second commit in our test run"
	commitCmdTimestamp = "2019-03-15T18:01:03Z"
	err = commit([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// * Verify the new commit data on disk matches our expectations *

	// Check if the metadata file exists on disk
	var meta metaData
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	c.Check(meta.Commits, chk.HasLen, 2)

	// Verify the values in the commit data match the values we provided
	com, ok := meta.Commits["9c4eab0f96134063f856fc48604f49c7c1e225a79f3eebc4e226dc01b4cfe9bc"] // This commit ID is what the given values should generate a commit ID as
	c.Assert(ok, chk.Equals, true)
	c.Check(com.AuthorName, chk.Equals, commitCmdAuthName)
	c.Check(com.AuthorEmail, chk.Equals, commitCmdAuthEmail)
	c.Check(com.Message, chk.Equals, commitCmdMsg)
	c.Check(com.Timestamp, chk.Equals, time.Date(2019, time.March, 15, 18, 1, 3, 0, time.UTC))
	c.Check(com.Parent, chk.Equals, "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb")
	c.Check(com.OtherParents, chk.IsNil)
	c.Check(com.CommitterName, chk.Equals, "Some One")
	c.Check(com.CommitterEmail, chk.Equals, "someone@example.org")
	c.Check(com.ID, chk.Equals, "9c4eab0f96134063f856fc48604f49c7c1e225a79f3eebc4e226dc01b4cfe9bc")
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
	c.Check(br.Commit, chk.Equals, "9c4eab0f96134063f856fc48604f49c7c1e225a79f3eebc4e226dc01b4cfe9bc")
	c.Check(br.CommitCount, chk.Equals, 2)
	c.Check(br.Description, chk.Equals, "")

	// TODO: Now that we can capture the display output for checking, we should probably verify the
	//       info displayed in the output here too
}

// Test the "dio branch" commands
func (s *DioSuite) Test0030_BranchActiveGet(c *chk.C) {
	// Query the active branch
	err := branchActiveGet([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the active branch is set to "master"
	p := strings.Split(s.buf.String(), ":")
	c.Assert(p, chk.HasLen, 2)
	c.Check(strings.TrimSpace(p[1]), chk.Equals, "master")
}

func (s *DioSuite) Test0040_BranchCreate(c *chk.C) {
	// Create a new branch
	branchCreateBranch = "branchtwo"
	branchCreateCommit = "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb"
	branchCreateMsg = "A new branch"
	err := branchCreate([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the new branch is in the metadata on disk
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok := meta.Branches["branchtwo"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb")
	c.Check(br.CommitCount, chk.Equals, 1)
	c.Check(br.Description, chk.Equals, "A new branch")

	// Verify the output given to the user
	p := strings.Split(s.buf.String(), "'")
	c.Assert(p, chk.HasLen, 3)
	c.Check(strings.TrimSpace(p[1]), chk.Equals, branchCreateBranch)
}

func (s *DioSuite) Test0050_BranchSetBranchTwo(c *chk.C) {
	// Switch to the new branch
	branchActiveSetBranch = "branchtwo"
	err := branchActiveSet([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the active branch was changed in the on disk metadata
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	c.Check(meta.ActiveBranch, chk.Equals, branchActiveSetBranch)

	// Verify the output given to the user
	p := strings.Split(s.buf.String(), "'")
	c.Check(strings.TrimSpace(p[1]), chk.Equals, branchActiveSetBranch)
}

func (s *DioSuite) Test0060_BranchList(c *chk.C) {
	// Create a new branch
	err := branchList([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify entries are present for both "master" and "branchtwo"
	lines := bufio.NewScanner(&s.buf)
	var branchTwoFound, masterFound bool
	for lines.Scan() {
		p := strings.Split(lines.Text(), "'")
		if len(p) > 2 && p[1] == "master" {
			c.Check(p, chk.HasLen, 3)
			masterFound = true
		}
		if len(p) > 2 && p[1] == "branchtwo" {
			c.Check(p, chk.HasLen, 3)
			branchTwoFound = true
		}
	}
	c.Check(masterFound, chk.Equals, true)
	c.Check(branchTwoFound, chk.Equals, true)
}

func (s *DioSuite) Test0070_BranchRemoveFail(c *chk.C) {
	// Attempt to remove the branch (should fail)
	branchRemoveBranch = "branchtwo"
	err := branchRemove([]string{s.dbName})
	c.Assert(err, chk.Not(chk.IsNil))

	// Make sure both the "master" and "branchtwo" branches are still present on disk
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	_, ok := meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	_, ok = meta.Branches["branchtwo"]
	c.Assert(ok, chk.Equals, true)

	// TODO: When the display of error messages to the user is a bit better finalised,
	//       add a check of the output here
}

func (s *DioSuite) Test0080_BranchSetMaster(c *chk.C) {
	// Switch to the master branch
	branchActiveSetBranch = "master"
	err := branchActiveSet([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the active branch was changed in the on disk metadata
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	c.Check(meta.ActiveBranch, chk.Equals, branchActiveSetBranch)

	// Verify the output given to the user
	p := strings.Split(s.buf.String(), "'")
	c.Check(strings.TrimSpace(p[1]), chk.Equals, branchActiveSetBranch)
}

func (s *DioSuite) Test0090_BranchRemoveSuccess(c *chk.C) {
	// Attempt to remove the branch (should succeed)
	branchRemoveBranch = "branchtwo"
	err := branchRemove([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Make sure the "master" branch is still present on disk, but "branchtwo" isn't
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	_, ok := meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	_, ok = meta.Branches["branchtwo"]
	c.Assert(ok, chk.Equals, false)

	// Verify the output given to the user
	p := strings.Split(s.buf.String(), "'")
	c.Check(strings.TrimSpace(p[1]), chk.Equals, branchRemoveBranch)
}

func (s *DioSuite) Test0100_BranchRevert(c *chk.C) {
	// Verify that (prior to the revert) the master branch still points to the 2nd commit
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok := meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "9c4eab0f96134063f856fc48604f49c7c1e225a79f3eebc4e226dc01b4cfe9bc")
	c.Check(br.CommitCount, chk.Equals, 2)
	c.Check(br.Description, chk.Equals, "")

	// Revert the master branch back to the original commit
	branchRevertBranch = "master"
	branchRevertCommit = "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb"
	err = branchRevert([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the master branch now points to the original commit
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok = meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb")
	c.Check(br.CommitCount, chk.Equals, 1)
	c.Check(br.Description, chk.Equals, "")

	// Verify the output given to the user
	c.Check(strings.TrimSpace(s.buf.String()), chk.Equals, "Branch reverted")
}

func (s *DioSuite) Test0110_BranchUpdateChgDesc(c *chk.C) {
	// Verify that (prior to the update) the master branch has an empty description
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok := meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Description, chk.Equals, "")

	// Update description for the master branch
	branchUpdateBranch = "master"
	branchUpdateMsg = "This is a new description"
	err = branchUpdate([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the description was correctly updated
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok = meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Description, chk.Equals, branchUpdateMsg)
}

func (s *DioSuite) Test0120_BranchUpdateDelDesc(c *chk.C) {
	// Verify that (prior to the update) the master branch has a non-empty description
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok := meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Description, chk.Not(chk.Equals), "")

	// Delete the description for the master branch
	*descDel = true
	err = branchUpdate([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the description was deleted
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok = meta.Branches["master"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Description, chk.Equals, "")

	// Verify the output given to the user
	c.Check(strings.TrimSpace(s.buf.String()), chk.Equals, "Branch updated")
}

func (s *DioSuite) Test0130_TagCreate(c *chk.C) {
	// Check the tag to be created doesn't yet exist
	tagCreateTag = "testtag1"
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	z, ok := meta.Tags[tagCreateTag]
	c.Assert(ok, chk.Equals, false)

	// Create the tag
	tagCreateCommit = "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb"
	tagCreateDate = "2019-03-15T18:01:05Z"
	tagCreateEmail = "sometagger@example.org"
	tagCreateMsg = "This is a test tag"
	tagCreateName = "A test tagger"
	err = tagCreate([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Check the tag was created
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	z, ok = meta.Tags[tagCreateTag]
	c.Assert(ok, chk.Equals, true)
	c.Check(z.Commit, chk.Equals, tagCreateCommit)
	c.Check(z.Date, chk.Equals, time.Date(2019, time.March, 15, 18, 1, 5, 0, time.UTC))
	c.Check(z.Description, chk.Equals, tagCreateMsg)
	c.Check(z.TaggerName, chk.Equals, tagCreateName)
	c.Check(z.TaggerEmail, chk.Equals, tagCreateEmail)

	// Verify the output given to the user
	c.Check(strings.TrimSpace(s.buf.String()), chk.Equals, "Tag creation succeeded")
}

func (s *DioSuite) Test0140_TagList(c *chk.C) {
	// Retrieve the tag list
	err := tagList([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// The tag created by the previous test should be listed
	lines := bufio.NewScanner(&s.buf)
	var tagFound bool
	for lines.Scan() {
		l := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(l, "*") {
			p := strings.Split(lines.Text(), "'")
			if len(p) > 2 && p[1] == tagCreateTag {
				c.Check(p, chk.HasLen, 3)
				tagFound = true
			}
		}
	}
	c.Check(tagFound, chk.Equals, true)
}

func (s *DioSuite) Test0150_TagRemove(c *chk.C) {
	// Verify that (prior to this removal) the tag exists
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	_, ok := meta.Tags[tagCreateTag]
	c.Assert(ok, chk.Equals, true)

	// Remove the tag
	tagRemoveTag = tagCreateTag
	err = tagRemove([]string{s.dbName})
	c.Check(err, chk.IsNil)

	// Verify the tag has been removed
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	_, ok = meta.Tags[tagCreateTag]
	c.Check(ok, chk.Equals, false)

	// Verify the output given to the user
	p := strings.Split(s.buf.String(), "'")
	c.Check(strings.TrimSpace(p[1]), chk.Equals, tagCreateTag)
}

func (s *DioSuite) Test0160_ReleaseCreate(c *chk.C) {
	// Check the release to be created doesn't yet exist
	releaseCreateRelease = "testrelease1"
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	z, ok := meta.Releases[releaseCreateRelease]
	c.Assert(ok, chk.Equals, false)

	// Create the release
	releaseCreateCommit = "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb"
	releaseCreateCreatorEmail = "somereleaser@example.org"
	releaseCreateCreatorName = "A test releaser"
	releaseCreateMsg = "This is a test release"
	releaseCreateReleaseDate = "2019-03-15T18:01:06Z"
	err = releaseCreate([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Check the release was created
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	z, ok = meta.Releases[releaseCreateRelease]
	c.Assert(ok, chk.Equals, true)
	c.Check(z.Commit, chk.Equals, releaseCreateCommit)
	c.Check(z.Date, chk.Equals, time.Date(2019, time.March, 15, 18, 1, 6, 0, time.UTC))
	c.Check(z.Description, chk.Equals, releaseCreateMsg)
	c.Check(z.ReleaserName, chk.Equals, releaseCreateCreatorName)
	c.Check(z.ReleaserEmail, chk.Equals, releaseCreateCreatorEmail)

	// Verify the output given to the user
	c.Check(strings.TrimSpace(s.buf.String()), chk.Equals, "Release creation succeeded")
}

func (s *DioSuite) Test0170_ReleaseList(c *chk.C) {
	// Retrieve the release list
	err := releaseList([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// The release created by the previous test should be listed
	lines := bufio.NewScanner(&s.buf)
	var relFound bool
	for lines.Scan() {
		l := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(l, "*") {
			p := strings.Split(lines.Text(), "'")
			if len(p) > 2 && p[1] == releaseCreateRelease {
				c.Check(p, chk.HasLen, 3)
				relFound = true
			}
		}
	}
	c.Check(relFound, chk.Equals, true)
}

func (s *DioSuite) Test0180_ReleaseRemove(c *chk.C) {
	// Verify that (prior to this removal) the release exists
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	_, ok := meta.Releases[releaseCreateRelease]
	c.Assert(ok, chk.Equals, true)

	// Remove the release
	releaseRemoveRelease = releaseCreateRelease
	err = releaseRemove([]string{s.dbName})
	c.Check(err, chk.IsNil)

	// Verify the release has been removed
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	_, ok = meta.Releases[releaseCreateRelease]
	c.Check(ok, chk.Equals, false)

	// Verify the output given to the user
	p := strings.Split(s.buf.String(), "'")
	c.Check(strings.TrimSpace(p[1]), chk.Equals, releaseCreateRelease)
}

func (s *DioSuite) Test0190_Log(c *chk.C) {
	// Retrieve the commit list
	err := branchLog([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// The original commit should be listed
	lines := bufio.NewScanner(&s.buf)
	var comFound bool
	for lines.Scan() {
		l := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(l, "*") {
			p := strings.Split(lines.Text(), ":")
			if len(p) >= 2 && strings.TrimSpace(p[1]) == "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb" {
				c.Check(p, chk.HasLen, 2)
				comFound = true
			}
		}
	}
	c.Check(comFound, chk.Equals, true)
}

func (s *DioSuite) Test0200_StatusUnchanged(c *chk.C) {
	// If we're not using a remote server, then mock the retrieveMetadata() function
	var oldRet func(db string) (metaData, bool, error)
	if *remoteServer == "" {
		oldRet = retrieveMetadata
		retrieveMetadata = mockRetrieveMetadata
	}

	// Run the status check command
	err := status([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the output
	lines := bufio.NewScanner(&s.buf)
	numEntries := 0
	unchangedFound := false
	for lines.Scan() {
		l := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(l, "*") {
			numEntries++
			p := strings.Split(lines.Text(), "'")
			if len(p) >= 2 && strings.TrimSpace(p[1]) == s.dbName {
				c.Check(p, chk.HasLen, 3)
				if strings.HasSuffix(p[2], "unchanged") {
					unchangedFound = true
				}
			}
		}
	}
	c.Check(numEntries, chk.Equals, 1)
	c.Check(unchangedFound, chk.Equals, true)

	// Restore the original mocked function
	if *remoteServer == "" {
		retrieveMetadata = oldRet
	}
}

func (s *DioSuite) Test0210_StatusChanged(c *chk.C) {
	// If we're not using a remote server, then mock the retrieveMetadata() function
	var oldRet func(db string) (metaData, bool, error)
	if *remoteServer == "" {
		oldRet = retrieveMetadata
		retrieveMetadata = mockRetrieveMetadata
	}

	// Get the current last modified date for the database file
	f, err := os.Stat(s.dbFile)
	c.Assert(err, chk.IsNil)
	lastMod := f.ModTime()

	// Update last modified date on the database file
	err = os.Chtimes(s.dbFile, time.Now(), time.Date(2019, time.March, 15, 18, 1, 8, 0, time.UTC))
	c.Assert(err, chk.IsNil)

	// Run the status check command
	err = status([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the output
	lines := bufio.NewScanner(&s.buf)
	numEntries := 0
	changedFound := false
	for lines.Scan() {
		l := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(l, "*") {
			numEntries++
			p := strings.Split(lines.Text(), "'")
			if len(p) >= 2 && strings.TrimSpace(p[1]) == s.dbName {
				c.Check(p, chk.HasLen, 3)
				if strings.HasSuffix(p[2], "changed") {
					changedFound = true
				}
			}
		}
	}
	c.Check(numEntries, chk.Equals, 1)
	c.Check(changedFound, chk.Equals, true)

	// Restore the last modified date on the database file
	err = os.Chtimes(s.dbFile, time.Now(), lastMod)
	c.Assert(err, chk.IsNil)

	// Restore the original mocked function
	if *remoteServer == "" {
		retrieveMetadata = oldRet
	}
}

func (s *DioSuite) Test0220_LicenceList(c *chk.C) {
	// Retrieve the licence list
	err := licenceList()
	c.Assert(err, chk.IsNil)

	// Make sure an entry for "No licence specified" is given on the output
	numEntries := 0
	licFound := false
	lines := bufio.NewScanner(&s.buf)
	for lines.Scan() {
		l := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(l, "*") {
			numEntries++
			p := strings.Split(lines.Text(), ":")
			if len(p) >= 2 && strings.TrimSpace(p[1]) == "No licence specified" {
				c.Check(p, chk.HasLen, 2)
				licFound = true
			}
		}
	}
	c.Check(licFound, chk.Equals, true)
}

func (s *DioSuite) Test0230_LicenceAdd(c *chk.C) {
	// Add a licence to the testing server
	licenceAddDisplayOrder = 2000
	licenceAddFileFormat = "text"
	licenceAddFullName = "GNU AFFERO GENERAL PUBLIC LICENSE"
	licenceAddFile = licFile
	licenceAddURL = "https://www.gnu.org/licenses/agpl-3.0.en.html"
	err := licenceAdd([]string{"AGPL3"})
	c.Assert(err, chk.IsNil)

	// Calculate the SHA256 of the licence file
	b, err := ioutil.ReadFile(licFile)
	c.Assert(err, chk.IsNil)
	z := sha256.Sum256(b)
	shaSum := hex.EncodeToString(z[:])

	// Verify the info sent via licenceAdd
	licList, err := getLicences()
	c.Assert(err, chk.IsNil)

	licVerify := licList["AGPL3"]
	c.Assert(licVerify.FileFormat, chk.Equals, licenceAddFileFormat)
	c.Assert(licVerify.FullName, chk.Equals, licenceAddFullName)
	c.Assert(licVerify.Order, chk.Equals, licenceAddDisplayOrder)
	c.Assert(licVerify.Sha256, chk.Equals, shaSum)
	c.Assert(licVerify.URL, chk.Equals, licenceAddURL)
}

func (s *DioSuite) Test0240_LicenceGet(c *chk.C) {
	// Calculate the SHA256 of the original licence file
	b, err := ioutil.ReadFile(licFile)
	c.Assert(err, chk.IsNil)
	z := sha256.Sum256(b)
	origSHASum := hex.EncodeToString(z[:])

	// Make sure "AGPL3.txt" doesn't exist in the current directory before the call to licenceGet()
	getFile := filepath.Join(tempDir, "AGPL3.txt")
	_, err = os.Stat(getFile)
	c.Assert(err, chk.Not(chk.IsNil))

	// Retrieve the AGPL3 licence from the test server using licenceGet()
	err = licenceGet([]string{"AGPL3"})
	c.Assert(err, chk.IsNil)

	// Verify the AGPL3.txt licence now exists, and its contents match what's expected
	_, err = os.Stat(getFile)
	c.Assert(err, chk.IsNil)
	y, err := ioutil.ReadFile(licFile)
	c.Assert(err, chk.IsNil)
	z = sha256.Sum256(y)
	newSHASum := hex.EncodeToString(z[:])
	c.Check(newSHASum, chk.Equals, origSHASum)
}

func (s *DioSuite) Test0250_LicenceRemove(c *chk.C) {
	// Verify the AGPL3 licence exists on the test server prior to our licenceRemove() call
	oldEntries := 0
	licFound := false
	err := licenceList()
	c.Assert(err, chk.IsNil)
	lines := bufio.NewScanner(&s.buf)
	for lines.Scan() {
		l := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(l, "*") {
			oldEntries++
			p := strings.Split(lines.Text(), ":")
			if len(p) >= 2 && strings.TrimSpace(p[1]) == "GNU AFFERO GENERAL PUBLIC LICENSE" {
				c.Check(p, chk.HasLen, 2)
				licFound = true
			}
		}
	}
	c.Check(licFound, chk.Equals, true)

	// Run the licence removal call
	err = licenceRemove([]string{"AGPL3"})
	c.Assert(err, chk.IsNil)

	// Verify the licence has been removed
	err = licenceList()
	c.Assert(err, chk.IsNil)
	numEntries := 0
	licFound = false
	lines = bufio.NewScanner(&s.buf)
	for lines.Scan() {
		l := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(l, "*") {
			numEntries++
			p := strings.Split(lines.Text(), ":")
			if len(p) >= 2 && strings.TrimSpace(p[1]) == "AGPL3" {
				c.Check(p, chk.HasLen, 2)
				licFound = true
			}
		}
	}
	c.Check(numEntries, chk.Equals, oldEntries-1)
	c.Check(licFound, chk.Equals, false)
}

func (s *DioSuite) Test0260_PullLocal(c *chk.C) {
	// Calculate the SHA256 of the test database
	b, err := ioutil.ReadFile(s.dbFile)
	c.Assert(err, chk.IsNil)
	z := sha256.Sum256(b)
	origSHASum := hex.EncodeToString(z[:])

	// Remove the local copy of our test database
	err = os.Remove(s.dbFile)
	c.Assert(err, chk.IsNil)

	// Grab the database from local cache
	pullCmdBranch = "master"
	pullCmdCommit = ""
	*pullForce = true
	err = pull([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the SHA256 of the retrieved database matches
	b, err = ioutil.ReadFile(s.dbFile)
	c.Assert(err, chk.IsNil)
	z = sha256.Sum256(b)
	newSHASum := hex.EncodeToString(z[:])
	c.Check(newSHASum, chk.Equals, origSHASum)
}

// Tests pushing a database with no local commit data, and which doesn't yet exist on the remote server (should succeed)
func (s *DioSuite) Test0270_PushCompletelyNewDB(c *chk.C) {
	// Make sure the new database isn't yet shown on the remote server
	newDB := "19kBv2.sqlite"
	dbList, err := getDatabases(cloud, "default")
	c.Assert(err, chk.IsNil)
	dbFound := false
	for _, j := range dbList {
		if j.Name == newDB {
			dbFound = true
		}
	}
	c.Assert(dbFound, chk.Equals, false)

	// Rename our test database from 19kB.sqlite to 19kBv2.sqlite, to give us a "new" database with no local metadata
	err = os.Rename(s.dbFile, newDB)
	c.Assert(err, chk.IsNil)
	err = os.Chtimes(newDB, time.Now(), time.Date(2019, time.March, 15, 18, 2, 0, 0, time.UTC))
	c.Assert(err, chk.IsNil)

	// Send the test database to the server
	pushCmdName = "Default test user"
	pushCmdBranch = "master"
	pushCmdCommit = ""
	pushCmdDB = newDB
	pushCmdEmail = "testdefault@dbhub.io"
	pushCmdForce = false
	pushCmdLicence = "Not specified"
	pushCmdMsg = "Test message"
	pushCmdPublic = false
	pushCmdTimestamp = time.Date(2019, time.March, 15, 18, 30, 0, 0, time.UTC).Format(time.RFC3339)
	err = push([]string{newDB})
	c.Assert(err, chk.IsNil)

	// Verify the new test database is on the server
	dbList, err = getDatabases(cloud, "default")
	c.Assert(err, chk.IsNil)
	dbFound = false
	for _, j := range dbList {
		if j.Name == newDB {
			dbFound = true
		}
	}
	c.Assert(dbFound, chk.Equals, true)
}

func (s *DioSuite) Test0280_PullRemote(c *chk.C) {
	// Calculate the SHA256 of the test database
	newDB := "19kBv2.sqlite"
	b, err := ioutil.ReadFile(newDB)
	c.Assert(err, chk.IsNil)
	z := sha256.Sum256(b)
	origSHASum := hex.EncodeToString(z[:])

	// Rename the local copy of our test database
	err = os.Rename(newDB, newDB+"-renamed")
	c.Assert(err, chk.IsNil)

	// Remove the local metadata and cache for our test database
	metaDir := filepath.Join(".dio", newDB)
	err = os.RemoveAll(metaDir)
	c.Assert(err, chk.IsNil)

	// Grab the database from our test server
	pullCmdBranch = ""
	pullCmdCommit = "eba04c5fe44ec4545c098d092f0231ed672949b3c93651821d3f1102c56e85eb"
	*pullForce = true
	err = pull([]string{newDB})
	c.Assert(err, chk.IsNil)

	// Verify the SHA256 of the retrieved database matches
	b, err = ioutil.ReadFile(newDB)
	c.Assert(err, chk.IsNil)
	z = sha256.Sum256(b)
	newSHASum := hex.EncodeToString(z[:])
	c.Check(newSHASum, chk.Equals, origSHASum)
}

// Tests pushing a database with no local commit data, but already exists on the remote server (should fail)
func (s *DioSuite) Test0290_PushExistingDBConflict(c *chk.C) {
	// Verify 19kBv2.sqlite exists on the test server
	newDB := "19kBv2.sqlite"
	dbList, err := getDatabases(cloud, "default")
	c.Assert(err, chk.IsNil)
	dbFound := false
	for _, j := range dbList {
		if j.Name == newDB {
			dbFound = true
		}
	}
	c.Assert(dbFound, chk.Equals, true)

	// Remove the local metadata for 19kBv2.sqlite
	metaDir := filepath.Join(".dio", newDB)
	err = os.RemoveAll(metaDir)
	c.Assert(err, chk.IsNil)

	// Push the database to the test server (should fail)
	pushCmdName = "Default test user"
	pushCmdBranch = "master"
	pushCmdCommit = ""
	pushCmdDB = newDB
	pushCmdEmail = "testdefault@dbhub.io"
	pushCmdForce = false
	pushCmdLicence = "Not specified"
	pushCmdMsg = "Test message"
	pushCmdPublic = false
	err = push([]string{newDB})
	c.Check(err, chk.Not(chk.IsNil))
}

// Tests pushing a database with local commit data, which doesn't yet exist on the remote server (should succeed)
func (s *DioSuite) Test0300_PushNewLocalDBAndMetadata(c *chk.C) {
	// Rename the test database to "19kBv3.sqlite"
	newDB := "19kBv3.sqlite"
	err := os.Rename("19kBv2.sqlite", newDB)
	c.Assert(err, chk.IsNil)
	err = os.Chtimes(newDB, time.Now(), time.Date(2019, time.March, 15, 18, 1, 10, 0, time.UTC))
	c.Assert(err, chk.IsNil)

	// Create a commit using the new test database
	commitCmdBranch = "master"
	commitCmdCommit = ""
	commitCmdAuthEmail = "testdefault@dbhub.io"
	commitCmdLicence = "Not specified"
	commitCmdMsg = "Test message"
	commitCmdAuthName = "Default test user"
	commitCmdTimestamp = time.Date(2019, time.March, 15, 18, 1, 10, 0, time.UTC).Format(time.RFC3339)
	err = commit([]string{newDB})

	// Verify the database doesn't exist remotely yet
	dbList, err := getDatabases(cloud, "default")
	c.Assert(err, chk.IsNil)
	dbFound := false
	for _, j := range dbList {
		if j.Name == newDB {
			dbFound = true
		}
	}
	c.Assert(dbFound, chk.Equals, false)

	// Push the database to the server
	pushCmdName = ""
	pushCmdBranch = ""
	pushCmdCommit = ""
	pushCmdDB = newDB
	pushCmdEmail = ""
	pushCmdForce = false
	pushCmdLicence = ""
	pushCmdMsg = ""
	pushCmdPublic = false
	err = push([]string{newDB})
	c.Check(err, chk.IsNil)

	// Verify the database has been created
	dbList, err = getDatabases(cloud, "default")
	c.Assert(err, chk.IsNil)
	dbFound = false
	for _, j := range dbList {
		if j.Name == newDB {
			dbFound = true
		}
	}
	c.Assert(dbFound, chk.Equals, true)
}

// Tests pushing a database with local commit data, which already exists remotely, and the local commits add to the remote (should succeed)
func (s *DioSuite) Test0310_PushLocalDBAndMetadata(c *chk.C) {
	// Create another local commit for 19kbv3.sqlite
	newDB := "19kBv3.sqlite"
	err := os.Chtimes(newDB, time.Now(), time.Date(2019, time.March, 15, 18, 11, 0, 0, time.UTC))
	c.Assert(err, chk.IsNil)
	commitCmdMsg = "Test message"
	commitCmdTimestamp = time.Date(2019, time.March, 15, 18, 1, 10, 0, time.UTC).Format(time.RFC3339)
	err = commit([]string{newDB})
	c.Check(err, chk.IsNil)

	// Push the new commit to the server
	pushCmdName = "Default test user"
	pushCmdBranch = "master"
	pushCmdCommit = ""
	pushCmdDB = s.dbName
	pushCmdEmail = "testdefault@dbhub.io"
	pushCmdForce = false
	pushCmdLicence = "Not specified"
	pushCmdMsg = "Test message"
	pushCmdPublic = false
	err = push([]string{newDB})
	c.Check(err, chk.IsNil)

	// Verify the server now has two commits for the database
	meta, _, err := retrieveMetadata(newDB)
	c.Assert(err, chk.IsNil)
	c.Check(meta.Commits, chk.HasLen, 2)
}

// Tests pushing a database with local commit data, which already exists remotely, and the local commits are behind the remote (should fail)
func (s *DioSuite) Test0320_PushOldLocalMetadataDBConflict(c *chk.C) {
	// We use the same 19bKv3.sqlite database from the previous test, reverting it back to it's original commit, then pushing

	// Revert back to the original commit
	newDB := "19kBv3.sqlite"
	pullCmdCommit = "c9a3e3bd641ca17a79a02cc07cc2ecd0c92a89a5437cf3b96221d57575a6f79f"
	*pullForce = true
	err := pull([]string{newDB})
	c.Assert(err, chk.IsNil)

	// Push the database to the server
	pushCmdName = ""
	pushCmdBranch = ""
	pushCmdCommit = ""
	pushCmdDB = newDB
	pushCmdEmail = ""
	pushCmdForce = false
	pushCmdLicence = ""
	pushCmdMsg = ""
	pushCmdPublic = false
	err = push([]string{newDB})
	c.Check(err, chk.Not(chk.IsNil))
}

// Mocked functions
func mockGetLicences() (map[string]licenceEntry, error) {
	return licList, nil
}

// Returns metadata of a database with a single commit, on the master branch
func mockRetrieveMetadata(db string) (meta metaData, onCloud bool, err error) {
	meta.Branches = make(map[string]branchEntry)
	meta.Commits = make(map[string]commitEntry)
	meta.Commits["485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb"] = commitEntry{
		ID:             "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb",
		CommitterEmail: "someone@example.org",
		CommitterName:  "Some One",
		AuthorEmail:    "testdefault@dbhub.io",
		AuthorName:     "Default test user",
		Message:        "The first commit in our test run",
		Timestamp:      time.Date(2019, time.March, 15, 18, 1, 1, 0, time.UTC),
		Tree: dbTree{
			ID: "8983130ceda4a2e39a3ad002945d57748987494907f475539fe766f8893cc278",
			Entries: []dbTreeEntry{{
				EntryType:    dbTreeEntryType(DATABASE),
				LastModified: time.Date(2019, time.March, 15, 18, 1, 0, 0, time.UTC),
				LicenceSHA:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				Name:         "19kB.sqlite",
				Sha256:       "e8cab91dec32b3990b427b28380e4e052288054f99c4894742f07dee0c924efd",
				Size:         19456},
			},
		},
	}
	meta.Branches["master"] = branchEntry{
		Commit:      "485ca5b2014c1520e7952ad97fed0a8024349b43cea7711a6c98706c3d7e55cb",
		CommitCount: 1,
		Description: "",
	}
	meta.DefBranch = "master"

	// No need for tags nor releases at this stage
	meta.Tags = make(map[string]tagEntry)
	meta.Releases = make(map[string]releaseEntry)
	return
}

func mockServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/default", mockServerDatabaseListHandler)
	mux.HandleFunc("/default/19kBv2.sqlite", mockServerPushPullSwitchHandler)
	mux.HandleFunc("/default/19kBv3.sqlite", mockServerNewDBPushHandler)
	mux.HandleFunc("/licence/add", mockServerLicenceAddHandler)
	mux.HandleFunc("/licence/get", mockServerLicenceGetHandler)
	mux.HandleFunc("/licence/remove", mockServerLicenceRemoveHandler)
	mux.HandleFunc("/metadata/get", mockServerMetadataGetHandler)
	newServer = &http.Server{
		Addr:         "localhost:5551",
		Handler:      mux,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}
	_ = newServer.ListenAndServeTLS(filepath.Join(origDir, "..", "test_data", "docker-dev.dbhub.io.cert.pem"),
		filepath.Join(origDir, "..", "test_data", "docker-dev.dbhub.io.key.pem"))
}

func mockServerDatabaseListHandler(w http.ResponseWriter, r *http.Request) {
	// Convert the database entries to JSON
	var msg bytes.Buffer
	enc := json.NewEncoder(&msg)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(mockDBEntries); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dbList := msg.Bytes()
	_, _ = fmt.Fprintf(w, "%s", dbList)
}

func mockServerLicenceAddHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the form variables
	licID := r.FormValue("licence_id")
	licName := r.FormValue("licence_name")
	do := r.FormValue("display_order")
	dispOrder, err := strconv.Atoi(do)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ff := r.FormValue("file_format")
	su := r.FormValue("source_url")
	tempFile, _, err := r.FormFile("file1")
	if err != nil {
		log.Printf("Uploading licence failed: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer tempFile.Close()

	// Calculate the SHA256 of the uploaded licence text
	licText := new(bytes.Buffer)
	_, err = io.Copy(licText, tempFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpSHA := sha256.Sum256(licText.Bytes())
	licSHA := hex.EncodeToString(tmpSHA[:])

	// Add the licence to the in memory licence list
	licList[licID] = licenceEntry{
		FullName:   licName,
		FileFormat: ff,
		Sha256:     licSHA,
		Order:      dispOrder,
		URL:        su,
	}

	// Send a success message back to the client
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, "Success")
}

func mockServerLicenceGetHandler(w http.ResponseWriter, r *http.Request) {
	// Make sure the correct licence is being requested
	if r.FormValue("licence") != "AGPL3" {
		http.Error(w, "Wrong licence requested", http.StatusNotFound)
		return
	}

	// Send the licence file to the client
	lic, err := ioutil.ReadFile(licFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=AGPL3.txt")
	w.Header().Set("Content-Type", "text/plain")
	_, err = fmt.Fprint(w, lic)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func mockServerLicenceRemoveHandler(w http.ResponseWriter, r *http.Request) {
	// Make sure the correct licence is being requested
	if r.FormValue("licence_id") != "AGPL3" {
		http.Error(w, "Wrong licence requested", http.StatusNotFound)
		return
	}

	// Remove the licence
	delete(licList, "AGPL3")

	// Send a success message back to the client
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "Success")
}

func mockServerMetadataGetHandler(w http.ResponseWriter, r *http.Request) {
	var info struct {
		Branches  map[string]branchEntry  `json:"branches"`
		Commits   map[string]commitEntry  `json:"commits"`
		DefBranch string                  `json:"default_branch"`
		Releases  map[string]releaseEntry `json:"releases"`
		Tags      map[string]tagEntry     `json:"tags"`
	}

	// Return the metadata for the requested database
	db := r.FormValue("dbname")
	meta, ok := mockMetaData[db]
	if !ok {
		// If we have no info for the database, just return a blank structure
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	info.Branches = meta.Branches
	info.DefBranch = meta.DefBranch
	info.Commits = meta.Commits
	info.Releases = meta.Releases
	info.Tags = meta.Tags
	jsonList, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		errMsg := fmt.Sprintf("Error when JSON marshalling the branch list: %v\n", err)
		log.Print(errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}
	fmt.Fprintf(w, string(jsonList))
}

func mockServerNewDBPushHandler(w http.ResponseWriter, r *http.Request) {
	expected := map[string]string{
		"authoremail":    "testdefault@dbhub.io",
		"authorname":     "Default test user",
		"branch":         "master",
		"commit":         "",
		"commitmsg":      "Test message",
		"committername":  "Some One",
		"committeremail": "someone@example.org",
		"dbshasum":       "e8cab91dec32b3990b427b28380e4e052288054f99c4894742f07dee0c924efd",
		"lastmodified":   time.Date(2019, time.March, 15, 18, 2, 0, 0, time.UTC).Format(time.RFC3339),
		"licence":        "",
		"public":         "false",
	}

	// Make sure the uploaded database file matches the expected SHASUM
	tempFile, hdr, err := r.FormFile("file1")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer tempFile.Close()
	s := sha256.New()
	var numBytes int64
	numBytes, err = io.Copy(s, tempFile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	shaSum := hex.EncodeToString(s.Sum(nil))
	if expected["dbshasum"] != shaSum {
		http.Error(w, "SHA256 of uploaded database doesn't match expected SHA256", http.StatusBadRequest)
		return
	}

	// Values for specific databases
	switch hdr.Filename {
	case "19kBv2.sqlite":
		expected["licence"] = "Not specified"
		expected["lastmodified"] = time.Date(2019, time.March, 15, 18, 2, 0, 0, time.UTC).Format(time.RFC3339)
		expected["committimestamp"] = time.Date(2019, time.March, 15, 18, 30, 0, 0, time.UTC).Format(time.RFC3339)

	case "19kBv3.sqlite":
		expected["lastmodified"] = time.Date(2019, time.March, 15, 18, 1, 10, 0, time.UTC).Format(time.RFC3339)
		expected["committimestamp"] = time.Date(2019, time.March, 15, 18, 1, 10, 0, time.UTC).Format(time.RFC3339)
	}

	// If the database already exists on the mock server, specific cases are handled in various ways
	for _, j := range mockDBEntries {
		if j.Name == hdr.Filename {
			// * Yep, the database is already on the mock server *

			// If no (parent) commit ID was provided, we fail, otherwise we use it for generating a second commit
			commitID := r.FormValue("commit")
			if commitID == "" {
				http.Error(w, "No commit ID was provided.  You probably need to upgrade your client before trying this "+
					"again.", http.StatusUpgradeRequired)
				return
			}
			expected["commit"] = commitID
			expected["licence"] = "Not specified"
			expected["lastmodified"] = time.Date(2019, time.March, 15, 18, 11, 00, 0, time.UTC).Format(time.RFC3339)
		}
	}

	// Check the remaining values match expectations
	for name, value := range expected {
		someValue := r.FormValue(name)
		if someValue != value {
			http.Error(w, fmt.Sprintf("incorrect %s value", name), http.StatusBadRequest)
			return
		}
	}

	// Generate a new commit ID
	z, err := time.Parse(time.RFC3339, expected["lastmodified"])
	lastMod := z.UTC()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var commitTime time.Time
	if expected["committimestamp"] == "" {
		commitTime = time.Now().UTC()
	} else {
		z, err = time.Parse(time.RFC3339, expected["committimestamp"])
		commitTime = z.UTC()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	var e dbTreeEntry
	e.EntryType = DATABASE
	e.LastModified = lastMod
	e.LicenceSHA = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // SHA256 of "Not specified" licence
	e.Name = hdr.Filename
	e.Sha256 = shaSum
	e.Size = int(numBytes)
	var t dbTree
	t.Entries = append(t.Entries, e)
	t.ID = createDBTreeID(t.Entries)
	newCom := commitEntry{
		AuthorName:     expected["authorname"],
		AuthorEmail:    expected["authoremail"],
		CommitterName:  expected["committername"],
		CommitterEmail: expected["committeremail"],
		Message:        expected["commitmsg"],
		Parent:         expected["commit"],
		Timestamp:      commitTime,
		Tree:           t,
	}
	newCom.ID = createCommitID(newCom)

	// Add the new database to the internal mock server database list
	entry := dbListEntry{
		CommitID:     newCom.ID,
		DefBranch:    expected["branch"],
		LastModified: lastMod.UTC().Format(time.RFC3339),
		Licence:      expected["licence"],
		Name:         hdr.Filename,
		OneLineDesc:  "A testing database",
		Public:       false,
		RepoModified: lastMod.UTC().Format(time.RFC3339),
		SHA256:       expected["dbshasum"],
		Size:         int(numBytes),
		Type:         "database",
		URL:          fmt.Sprintf("%s/default/%s?commit=%s&branch=%s", cloud, hdr.Filename, newCom.ID, expected["branch"]), // TODO: Is this the right URL, or is it supposed to be the user defined source URL?
	}
	mockDBEntries = append(mockDBEntries, entry)

	// Add the new commit info to the internal mock server metadata list
	meta, ok := mockMetaData[hdr.Filename]
	if !ok {
		meta = metaData{
			Branches:  make(map[string]branchEntry),
			DefBranch: "",
			Commits:   make(map[string]commitEntry),
			Releases:  make(map[string]releaseEntry),
			Tags:      make(map[string]tagEntry),
		}
	}
	meta.Commits[newCom.ID] = newCom
	meta.DefBranch = expected["branch"]
	mockMetaData[hdr.Filename] = meta

	// Increment the commit counter if this push is for a subsequent commit for the database
	commitCount := 1
	br, ok := meta.Branches[expected["branch"]]
	if ok {
		commitCount = br.CommitCount + 1
	}
	meta.Branches[expected["branch"]] = branchEntry{
		Commit:      newCom.ID,
		CommitCount: commitCount,
	}

	// Return an appropriate response to the calling function
	u := fmt.Sprintf("https://%s/default/%s", cloud, hdr.Filename)
	u += fmt.Sprintf(`?branch=%s&commit=%s`, expected["branch"], newCom.ID)
	m := map[string]string{"commit_id": newCom.ID, "url": u}

	// Convert to JSON
	var msg bytes.Buffer
	enc := json.NewEncoder(&msg)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send return message back to the caller
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, msg.String())
}

func mockServerPullHandler(w http.ResponseWriter, r *http.Request) {
	// This code is copied from the DB4S end point retrieveDatabase() call, with the values for 19kbv2.sqlite added
	db := "19kBv2.sqlite"
	br := mockMetaData[db].DefBranch
	com := mockMetaData[db].Branches[br].Commit
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; modification-date="%s";`,
		url.QueryEscape(db), mockMetaData[db].Commits[com].Timestamp.UTC().Format(time.RFC3339)))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", 19456))
	w.Header().Set("Content-Type", "application/x-sqlite3")
	w.Header().Set("Branch", br)
	w.Header().Set("Commit-ID", com)
	path := filepath.Join(tempDir, db+"-renamed")
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func mockServerPushPullSwitchHandler(w http.ResponseWriter, r *http.Request) {
	// The handler to use depends upon the request type
	reqType := r.Method
	if reqType == "GET" {
		mockServerPullHandler(w, r)
	} else {
		mockServerNewDBPushHandler(w, r)
	}
	return
}
