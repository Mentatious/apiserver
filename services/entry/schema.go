package entry

import (
	"time"

	"github.com/Mentatious/mentat-apiserver/services"
)

// TODO: move [some of these] to config storage (file/whatever)
// Mentat DB traits
const (
	MentatDatabase       = "mentat-database"
	MentatCollection     = "mentat-entries"
	DatetimeLayout       = "0000-00-00T00:00:00.000Z" // TODO: maybe use appropriate constant from "time" module
	MongoNotFound        = "not found"
	BatchDeleteThreshold = 10 // if we delete more than this amount of records, use batch removal mode
)

// Service ... Entries RPC service
type Service struct {
	services.BaseService
}

// PostMetadata ... metadata for post
type PostMetadata struct {
	Description     string
	TimeAddedOrigin string
	HashOrigin      string
	MetaOrigin      string
	From            string
}

// Entry ... db representation
type Entry struct {
	Content    string
	Type       string
	Tags       []string
	Scheduled  time.Time
	Deadline   time.Time
	AddedAt    time.Time
	ModifiedAt time.Time
	Priority   string
	TodoStatus string
	Metadata   PostMetadata
	UUID       string
}

// entry type constants
const (
	EntryTypePim      = "pim"
	EntryTypeBookmark = "bookmark"
	EntryTypeOrg      = "org"
)

// AddEntryArgs ... args for Add method
type AddEntryArgs struct {
	UserID     string
	Type       string
	Content    string
	Tags       []string
	Metadata   PostMetadata
	Scheduled  string
	Deadline   string
	Priority   string
	TodoStatus string
}

// AddResponse ... JSON-RPC response for Add method
type AddResponse struct {
	Message string
}

// UpdateEntryArgs ... args for Update method
type UpdateEntryArgs struct {
	UserID     string
	UUID       string
	Type       string
	Content    string
	Tags       []string
	Scheduled  string
	Deadline   string
	Priority   string
	TodoStatus string
}

// UpdateResponse ... JSON-RPC response for Update method
type UpdateResponse struct {
	Message string
}

// CleanupArgs ... args for Cleanup method
type CleanupArgs struct {
	UserID string
	Types  []string
}

// CleanupResponse ... JSON-RPC response for Cleanup method
type CleanupResponse struct {
	Error   string
	Deleted int
}

// StatsArgs ... args for Stats method
type StatsArgs struct {
	UserID   string
	Detailed bool
}

// StatsResponse ... JSON-RPC response for Stats method
type StatsResponse struct {
	Error     string
	Whole     int
	Bookmarks int
	Pim       int
	Org       int
}

// DeleteEntryArgs ... args for Delete method
type DeleteEntryArgs struct {
	UserID string
	UUIDs  []string
}

// DeleteResponse ... JSON-RPC response for Delete method
type DeleteResponse struct {
	Error   string
	Deleted int
}

// SearchEntryArgs ... args for Search method
type SearchEntryArgs struct {
	UserID   string
	Types    []string
	Content  string
	Tags     []string
	Priority []string
}

// SearchResponse ... JSON-RPC response for Search method
type SearchResponse struct {
	Error   string
	Count   int
	Entries []Entry
}
