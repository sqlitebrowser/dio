package cmd

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
`
)

var (
	_            = chk.Suite(&DioSuite{})
	licFile      string
	remoteServer = flag.String("remote", "https://localhost:5550", "URL of remote server to test against")
	showFlag     = flag.Bool("show", false, "Don't redirect test command output to /dev/null")
	tempDir      string
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
	defer f.Close()
	d, err := os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}
	_, err = fmt.Fprintf(f, CONFIG,
		filepath.Join(d, "..", "test_data", "ca-chain-docker.cert.pem"),
		filepath.Join(tempDir, "default.cert.pem"),
		*remoteServer)
	if err != nil {
		log.Fatalln(err)
	}

	// Generate new client certificate for testing purposes
	err = genTestCert(*remoteServer, filepath.Join(tempDir, "default.cert.pem"))
	if err != nil {
		log.Fatal(err)
	}

	// Seed the database
	err = seedTests(*remoteServer)
	if err != nil {
		log.Fatal(err)
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
	chainFile, err := os.ReadFile(filepath.Join(d, "..", "test_data", "ca-chain-docker.cert.pem"))
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

	// Load our self signed CA Cert chain, and set TLS1.2 as minimum
	TLSConfig = tls.Config{
		Certificates:             []tls.Certificate{cert},
		ClientCAs:                ourCAPool,
		InsecureSkipVerify:       true,
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		RootCAs:                  ourCAPool,
	}

	var email string
	certUser, email, _, err = getUserAndServer()
	if err != nil {
		log.Fatalln(err)
	}
	viper.Set("user.email", email)

	// Add test database
	s.dbName = "19kB.sqlite"
	db, err := os.ReadFile(filepath.Join(d, "..", "test_data", s.dbName))
	if err != nil {
		log.Fatalln(err)
	}
	s.dbFile = filepath.Join(tempDir, s.dbName)
	err = os.WriteFile(s.dbFile, db, 0644)
	if err != nil {
		log.Fatalln(err)
	}

	// Set the last modified date of the database file to a known value
	err = os.Chtimes(s.dbFile, time.Now(), time.Date(2019, time.March, 15, 18, 1, 0, 0, time.UTC))
	if err != nil {
		log.Fatalln(err)
	}

	// Add a test licence
	lic, err := os.ReadFile(filepath.Join(d, "..", "LICENSE"))
	if err != nil {
		log.Fatalln(err)
	}
	licFile = filepath.Join(tempDir, "test.licence")
	err = os.WriteFile(licFile, lic, 0644)
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

func (s *DioSuite) TearDownTest(c *chk.C) {
	// Restore the display output redirection
	fOut = s.oldOut

	// Clear the buffered contents
	s.buf.Reset()
}

// Test the "dio commit" command
func (s *DioSuite) Test0010_Commit(c *chk.C) {
	// Call the commit code
	commitCmdBranch = "main"
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
	com, ok := meta.Commits["59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941"] // This commit ID is what the given values should generate a commit ID as
	c.Assert(ok, chk.Equals, true)
	c.Check(com.AuthorName, chk.Equals, commitCmdAuthName)
	c.Check(com.AuthorEmail, chk.Equals, commitCmdAuthEmail)
	c.Check(com.Message, chk.Equals, commitCmdMsg)
	c.Check(com.Timestamp, chk.Equals, time.Date(2019, time.March, 15, 18, 1, 1, 0, time.UTC))
	c.Check(com.Parent, chk.Equals, "")
	c.Check(com.OtherParents, chk.IsNil)
	c.Check(com.CommitterName, chk.Equals, "Some One")
	c.Check(com.CommitterEmail, chk.Equals, "default@docker-dev.dbhub.io")
	c.Check(com.ID, chk.Equals, "59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941")
	c.Check(com.Tree.Entries[0].EntryType, chk.Equals, dbTreeEntryType(DATABASE))
	c.Check(com.Tree.Entries[0].LastModified.UTC(), chk.Equals, time.Date(2019, time.March, 15, 18, 1, 0, 0, time.UTC))
	c.Check(com.Tree.Entries[0].LicenceSHA, chk.Equals, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855") // e3b... is the SHA256 for the "Not specified" licence option
	c.Check(int(com.Tree.Entries[0].Size), chk.Equals, 19456)
	c.Check(com.Tree.Entries[0].Name, chk.Equals, s.dbName)

	// Check the database has been written to the cache area using its checksum as filename
	cacheFile := filepath.Join(".dio", s.dbName, "db", com.Tree.Entries[0].Sha256)
	_, err = os.Stat(cacheFile)
	c.Assert(err, chk.IsNil)

	// Verify the contents of the cached database match the size and sha256 recorded in the commit
	b, err := os.ReadFile(cacheFile)
	c.Assert(err, chk.IsNil)
	c.Check(b, chk.HasLen, int(com.Tree.Entries[0].Size))
	z := sha256.Sum256(b)
	shaSum := hex.EncodeToString(z[:])
	c.Check(shaSum, chk.Equals, com.Tree.Entries[0].Sha256)

	// Verify the branch info
	br, ok := meta.Branches["main"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941")
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
	com, ok := meta.Commits["70815d687cfc614b23dfb2f66f8fa0a8cb7bad199e05d4142db883690eefeba5"] // This commit ID is what the given values should generate a commit ID as
	c.Assert(ok, chk.Equals, true)
	c.Check(com.AuthorName, chk.Equals, commitCmdAuthName)
	c.Check(com.AuthorEmail, chk.Equals, commitCmdAuthEmail)
	c.Check(com.Message, chk.Equals, commitCmdMsg)
	c.Check(com.Timestamp, chk.Equals, time.Date(2019, time.March, 15, 18, 1, 3, 0, time.UTC))
	c.Check(com.Parent, chk.Equals, "59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941")
	c.Check(com.OtherParents, chk.IsNil)
	c.Check(com.CommitterName, chk.Equals, "Some One")
	c.Check(com.CommitterEmail, chk.Equals, "default@docker-dev.dbhub.io")
	c.Check(com.ID, chk.Equals, "70815d687cfc614b23dfb2f66f8fa0a8cb7bad199e05d4142db883690eefeba5")
	c.Check(com.Tree.Entries[0].EntryType, chk.Equals, dbTreeEntryType(DATABASE))
	c.Check(com.Tree.Entries[0].LastModified.UTC(), chk.Equals, time.Date(2019, time.March, 15, 18, 1, 2, 0, time.UTC))
	c.Check(com.Tree.Entries[0].LicenceSHA, chk.Equals, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855") // e3b... is the SHA256 for the "Not specified" licence option
	c.Check(int(com.Tree.Entries[0].Size), chk.Equals, 19456)
	c.Check(com.Tree.Entries[0].Name, chk.Equals, s.dbName)

	// Check the database has been written to the cache area using its checksum as filename
	cacheFile := filepath.Join(".dio", s.dbName, "db", com.Tree.Entries[0].Sha256)
	_, err = os.Stat(cacheFile)
	c.Assert(err, chk.IsNil)

	// Verify the contents of the cached database match the size and sha256 recorded in the commit
	b, err := os.ReadFile(cacheFile)
	c.Assert(err, chk.IsNil)
	c.Check(b, chk.HasLen, int(com.Tree.Entries[0].Size))
	z := sha256.Sum256(b)
	shaSum := hex.EncodeToString(z[:])
	c.Check(shaSum, chk.Equals, com.Tree.Entries[0].Sha256)

	// Verify the branch info
	br, ok := meta.Branches["main"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "70815d687cfc614b23dfb2f66f8fa0a8cb7bad199e05d4142db883690eefeba5")
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

	// Verify the active branch is set to "main"
	p := strings.Split(s.buf.String(), ":")
	c.Assert(p, chk.HasLen, 2)
	c.Check(strings.TrimSpace(p[1]), chk.Equals, "main")
}

func (s *DioSuite) Test0040_BranchCreate(c *chk.C) {
	// Create a new branch
	branchCreateBranch = "branchtwo"
	branchCreateCommit = "59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941"
	branchCreateMsg = "A new branch"
	err := branchCreate([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the new branch is in the metadata on disk
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok := meta.Branches["branchtwo"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941")
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

	// Verify entries are present for both "main" and "branchtwo"
	lines := bufio.NewScanner(&s.buf)
	var branchTwoFound, mainFound bool
	for lines.Scan() {
		p := strings.Split(lines.Text(), "'")
		if len(p) > 2 && p[1] == "main" {
			c.Check(p, chk.HasLen, 3)
			mainFound = true
		}
		if len(p) > 2 && p[1] == "branchtwo" {
			c.Check(p, chk.HasLen, 3)
			branchTwoFound = true
		}
	}
	c.Check(mainFound, chk.Equals, true)
	c.Check(branchTwoFound, chk.Equals, true)
}

func (s *DioSuite) Test0070_BranchRemoveFail(c *chk.C) {
	// Attempt to remove the branch (should fail)
	branchRemoveBranch = "branchtwo"
	err := branchRemove([]string{s.dbName})
	c.Assert(err, chk.Not(chk.IsNil))

	// Make sure both the "main" and "branchtwo" branches are still present on disk
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	_, ok := meta.Branches["main"]
	c.Assert(ok, chk.Equals, true)
	_, ok = meta.Branches["branchtwo"]
	c.Assert(ok, chk.Equals, true)

	// TODO: When the display of error messages to the user is a bit better finalised,
	//       add a check of the output here
}

func (s *DioSuite) Test0080_BranchSetMain(c *chk.C) {
	// Switch to the main branch
	branchActiveSetBranch = "main"
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

	// Make sure the "main" branch is still present on disk, but "branchtwo" isn't
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	_, ok := meta.Branches["main"]
	c.Assert(ok, chk.Equals, true)
	_, ok = meta.Branches["branchtwo"]
	c.Assert(ok, chk.Equals, false)

	// Verify the output given to the user
	p := strings.Split(s.buf.String(), "'")
	c.Check(strings.TrimSpace(p[1]), chk.Equals, branchRemoveBranch)
}

func (s *DioSuite) Test0100_BranchRevert(c *chk.C) {
	// Verify that (prior to the revert) the main branch still points to the 2nd commit
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok := meta.Branches["main"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "70815d687cfc614b23dfb2f66f8fa0a8cb7bad199e05d4142db883690eefeba5")
	c.Check(br.CommitCount, chk.Equals, 2)
	c.Check(br.Description, chk.Equals, "")

	// Revert the main branch back to the original commit
	branchRevertBranch = "main"
	branchRevertCommit = "59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941"
	err = branchRevert([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the main branch now points to the original commit
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok = meta.Branches["main"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Commit, chk.Equals, "59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941")
	c.Check(br.CommitCount, chk.Equals, 1)
	c.Check(br.Description, chk.Equals, "")

	// Verify the output given to the user
	c.Check(strings.TrimSpace(s.buf.String()), chk.Equals, "Branch reverted")
}

func (s *DioSuite) Test0110_BranchUpdateChgDesc(c *chk.C) {
	// Verify that (prior to the update) the main branch has an empty description
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok := meta.Branches["main"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Description, chk.Equals, "")

	// Update description for the main branch
	branchUpdateBranch = "main"
	branchUpdateMsg = "This is a new description"
	err = branchUpdate([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the description was correctly updated
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok = meta.Branches["main"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Description, chk.Equals, branchUpdateMsg)
}

func (s *DioSuite) Test0120_BranchUpdateDelDesc(c *chk.C) {
	// Verify that (prior to the update) the main branch has a non-empty description
	meta, err := localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok := meta.Branches["main"]
	c.Assert(ok, chk.Equals, true)
	c.Check(br.Description, chk.Not(chk.Equals), "")

	// Delete the description for the main branch
	*descDel = true
	err = branchUpdate([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the description was deleted
	meta, err = localFetchMetadata(s.dbName, false)
	c.Assert(err, chk.IsNil)
	br, ok = meta.Branches["main"]
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
	tagCreateCommit = "59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941"
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
	releaseCreateCommit = "59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941"
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
			if len(p) >= 2 && strings.TrimSpace(p[1]) == "59b72b78cb83bdba371438cb36950fe007265445a63068ae5586c9cc19203941" {
				c.Check(p, chk.HasLen, 2)
				comFound = true
			}
		}
	}
	c.Check(comFound, chk.Equals, true)
}

func (s *DioSuite) Test0200_StatusUnchanged(c *chk.C) {
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
}

func (s *DioSuite) Test0210_StatusChanged(c *chk.C) {
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
	b, err := os.ReadFile(licFile)
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
	b, err := os.ReadFile(licFile)
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
	y, err := os.ReadFile(licFile)
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
	b, err := os.ReadFile(s.dbFile)
	c.Assert(err, chk.IsNil)
	z := sha256.Sum256(b)
	origSHASum := hex.EncodeToString(z[:])

	// Remove the local copy of our test database
	err = os.Remove(s.dbFile)
	c.Assert(err, chk.IsNil)

	// Grab the database from local cache
	pullCmdBranch = "main"
	pullCmdCommit = ""
	*pullForce = true
	err = pull([]string{s.dbName})
	c.Assert(err, chk.IsNil)

	// Verify the SHA256 of the retrieved database matches
	b, err = os.ReadFile(s.dbFile)
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
	pushCmdBranch = "main"
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
	b, err := os.ReadFile(newDB)
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
	pullCmdCommit = "9a78d0c8c13c0442eb24367f3561d0cbb5676100bd69d147ec3bde4cdbaefa49"
	*pullForce = true
	err = pull([]string{newDB})
	c.Assert(err, chk.IsNil)

	// Verify the SHA256 of the retrieved database matches
	b, err = os.ReadFile(newDB)
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
	pushCmdBranch = "main"
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
	commitCmdBranch = "main"
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
	pushCmdBranch = "main"
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
	pullCmdCommit = "601e78fb16a715e37e86fc57ba3415e2684481cffea0455eb3463dc086e22177"
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

// genTestCert retrieves a client certificate from the remote server
func genTestCert(server, outputPath string) (err error) {
	// Disable https cert validation for our tests
	insecureTLS := tls.Config{InsecureSkipVerify: true}
	insecureTransport := http.Transport{TLSClientConfig: &insecureTLS}
	client := http.Client{Transport: &insecureTransport}

	// Request the new client certificate
	resp, err := client.Get(strings.Replace(server, "5550", "9443", 1) + "/x/test/gencert")
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("client certificate generation request failure. http code '%d' returned", resp.StatusCode)
		return
	}

	// Write the certificate to the filesystem
	rawCert, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	err = os.WriteFile(outputPath, rawCert, 0640)
	if err != nil {
		return
	}
	return
}

// seedTests retrieves a client certificate from the remote server
func seedTests(server string) (err error) {
	// Disable https cert validation for our tests
	insecureTLS := tls.Config{InsecureSkipVerify: true}
	insecureTransport := http.Transport{TLSClientConfig: &insecureTLS}
	client := http.Client{Transport: &insecureTransport}

	// Seed the database
	resp, err := client.Get(strings.Replace(server, "5550", "9443", 1) + "/x/test/seed")
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("seeding database failed. http code '%d' returned", resp.StatusCode)
		return
	}
	return
}
