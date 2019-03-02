package cmd

import "time"

type branchEntries struct {
	Branch  string
	Entries []commitEntry
}

type branchEntry struct {
	Commit      string `json:"commit"`
	CommitCount int    `json:"commit_count"`
	Description string `json:"description"`
}

type commitEntry struct {
	AuthorEmail    string    `json:"author_email"`
	AuthorName     string    `json:"author_name"`
	CommitterEmail string    `json:"committer_email"`
	CommitterName  string    `json:"committer_name"`
	ID             string    `json:"id"`
	Message        string    `json:"message"`
	Parent         string    `json:"parent"`
	Timestamp      time.Time `json:"timestamp"`
	Tree           dbTree    `json:"tree"`
}

type CommitList struct {
	Commits []commitEntry `json:"commits"`
}

type dbListEntry struct {
	CommitID     string `json:"commit_id"`
	DefBranch    string `json:"default_branch"`
	LastModified string `json:"last_modified"`
	Licence      string `json:"licence"`
	Name         string `json:"name"`
	OneLineDesc  string `json:"one_line_description"`
	Public       bool   `json:"public"`
	RepoModified string `json:"repo_modified"`
	SHA256       string `json:"sha256"`
	Size         int    `json:"size"`
	Type         string `json:"type"`
	URL          string `json:"url"`
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
	AType        dbTreeEntryType `json:"type"`
	LastModified time.Time       `json:"last_modified"`
	Licence      string          `json:"licence"`
	Name         string          `json:"name"`
	Sha256       string          `json:"sha256"`
	Size         int             `json:"size"`
}

type errorInfo struct {
	Condition string   `json:"error_condition"`
	Data      []string `json:"data"`
}

type licenceEntry struct {
	FileFormat string `json:"file_format"`
	FullName   string `json:"full_name"`
	Order      int    `json:"order"`
	Sha256     string `json:"sha256"`
	URL        string `json:"url"`
}

type metaData struct {
	ActiveBranch string                  `json:"active_branch"` // The local branch
	Branches     map[string]branchEntry  `json:"branches"`
	Commits      map[string]commitEntry  `json:"commits"`
	DefBranch    string                  `json:"default_branch"` // The default branch *on the server*
	Releases     map[string]ReleaseEntry `json:"releases"`
	Tags         map[string]tagEntry     `json:"tags"`
}

type ReleaseEntry struct {
	Commit        string    `json:"commit"`
	Date          time.Time `json:"date"`
	Description   string    `json:"description"`
	ReleaserEmail string    `json:"email"`
	ReleaserName  string    `json:"name"`
	Size          int       `json:"size"`
}

type tagEntry struct {
	Commit      string    `json:"commit"`
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	TaggerEmail string    `json:"email"`
	TaggerName  string    `json:"name"`
}
