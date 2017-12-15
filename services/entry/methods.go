package entry

import (
	"fmt"
	"github.com/satori/go.uuid"
	"gopkg.in/mgo.v2/bson"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	uidMissing = "User ID is missing"
)

// Add ... add entry to DB
func (s *Service) Add(r *http.Request, args *AddEntryArgs, result *AddResponse) error {
	if args.UserID == "" {
		result.Message = uidMissing
		return nil
	}
	entryType := args.Type
	if entryType == "" {
		result.Message = "Entry type is missing"
		return nil
	} else if (entryType != EntryTypePim) && (entryType != EntryTypeBookmark) && (entryType != EntryTypeOrg) {
		result.Message = "Unknown entry type"
		return nil
	}
	content := args.Content
	if content == "" {
		result.Message = "Empty content not allowed"
		return nil
	}
	s.Log.Infof("received '%s' entry: '%s'", entryType, content)

	coll := s.Session.DB(MentatDatabase).C(args.UserID)

	entry := Entry{}
	mgoErr := coll.Find(bson.M{"content": content}).One(&entry)
	if mgoErr != nil {
		if mgoErr.Error() == MongoNotFound {
			entry.Type = args.Type
			entry.Content = content
			tags := args.Tags
			if len(tags) > 0 {
				var lowerTags []string
				for _, tag := range tags {
					lowerTags = append(lowerTags, strings.ToLower(tag))
				}
				tags := lowerTags
				entry.Tags = tags
			}
			if args.Scheduled != "" {
				scheduled, err := time.Parse(DatetimeLayout, args.Scheduled)
				if err != nil {
					return err
				}
				entry.Scheduled = scheduled
			}
			if args.Deadline != "" {
				deadline, err := time.Parse(DatetimeLayout, args.Deadline)
				if err != nil {
					return err
				}
				entry.Deadline = deadline
			}

			now := time.Now()
			entry.AddedAt = now
			entry.ModifiedAt = now

			if args.Priority != "" {
				rexp, err := regexp.Compile("\\#[A-Z]$")
				if err != nil {
					panic(err) // sentinel, should fail, because such error is predictable
				}
				if rexp.Match([]byte(args.Priority)) {
					entry.Priority = args.Priority
				} else {
					result.Message = "Malformed priority value"
					return nil
				}
			}

			if args.TodoStatus != "" {
				entry.TodoStatus = strings.ToUpper(args.TodoStatus)
			}

			if (PostMetadata{}) != args.Metadata {
				entry.Metadata = args.Metadata
			}

			entry.UUID = uuid.NewV4().String()
			mgoErr = coll.Insert(&entry)
			if mgoErr != nil {
				s.Log.Infof("failed to insert entry: %s", mgoErr.Error())
				result.Message = fmt.Sprintf("failed to insert entry: %s", mgoErr.Error())
				return nil
			}
			result.Message = entry.UUID
			return nil
		}
		s.Log.Infof("mgo error: %s", mgoErr)
		result.Message = fmt.Sprintf("mgo error: %s", mgoErr)
		return nil
	}
	result.Message = "Already exists, skipping"
	return nil
}

// Update ... Update entry in DB
func (s *Service) Update(r *http.Request, args *UpdateEntryArgs, result *UpdateResponse) error {
	// Since there is no fixed data schema, we can update as we like, so be careful
	if args.UserID == "" {
		result.Message = uidMissing
		return nil
	}
	uuid := args.UUID
	if uuid != "" {
		coll := s.Session.DB(MentatDatabase).C(args.UserID)
		entry := Entry{}
		mgoErr := coll.Find(bson.M{"uuid": uuid}).One(&entry)
		if mgoErr != nil {
			if mgoErr.Error() == MongoNotFound {
				result.Message = "No entry with provided UUID"
				return nil
			}
			s.Log.Infof("mgo error: %s", mgoErr)
			result.Message = fmt.Sprintf("mgo error: %s", mgoErr)
			return nil
		}
		// TODO: maybe use reflection
		if args.Type != "" {
			entry.Type = args.Type
		}
		if args.Content != "" {
			entry.Content = args.Content
		}
		if len(args.Tags) > 0 {
			entry.Tags = args.Tags
		}
		if args.Scheduled != "" {
			scheduled, err := time.Parse(DatetimeLayout, args.Scheduled)
			if err != nil {
				return err
			}
			entry.Scheduled = scheduled
		}
		if args.Deadline != "" {
			deadline, err := time.Parse(DatetimeLayout, args.Deadline)
			if err != nil {
				return err
			}
			entry.Deadline = deadline
		}

		if args.Priority != "" {
			rexp, err := regexp.Compile("\\#[A-Z]$")
			if err != nil {
				panic(err) // sentinel, should fail, because such error is predictable
			}
			if rexp.Match([]byte(args.Priority)) {
				entry.Priority = args.Priority
			} else {
				result.Message = "Malformed priority value"
				return nil
			}
		}

		if args.TodoStatus != "" {
			entry.TodoStatus = strings.ToUpper(args.TodoStatus)
		}
		entry.ModifiedAt = time.Now()
		_, err := coll.Upsert(bson.M{"uuid": uuid}, entry)
		if err != nil {
			result.Message = fmt.Sprintf("update failed: %s", err)
			return nil
		}
		result.Message = "updated"
		return nil
	}
	result.Message = "No UUID found, cannot proceed with updating"
	return nil
}

// Cleanup ... Cleanup DB
func (s *Service) Cleanup(r *http.Request, args *CleanupArgs, result *CleanupResponse) error {
	if args.UserID == "" {
		result.Error = uidMissing
		result.Deleted = 0
		return nil
	}
	entryTypes := []string{EntryTypePim, EntryTypeBookmark, EntryTypeOrg}
	if len(args.Types) > 0 {
		entryTypes = args.Types
	}
	coll := s.Session.DB(MentatDatabase).C(args.UserID)
	changed, err := coll.RemoveAll(bson.M{"type": bson.M{"$in": entryTypes}})
	if err != nil {
		result.Error = fmt.Sprintf("cleanup failed: %s", err)
		result.Deleted = 0
		return nil
	}
	result.Deleted = changed.Removed
	return nil
}

// Stats ... Show DB stats (overall and type-wise entries count)
func (s *Service) Stats(r *http.Request, args *StatsArgs, result *StatsResponse) error {
	if args.UserID == "" {
		result.Error = uidMissing
		return nil
	}
	result.Whole = -1
	result.Bookmarks = -1
	result.Pim = -1
	result.Org = -1
	coll := s.Session.DB(MentatDatabase).C(args.UserID)
	wholeCount, err := coll.Count()
	if err != nil {
		result.Error = fmt.Sprintf("failed getting stats/whole count: %s", err)
		return nil
	}
	result.Whole = wholeCount
	if args.Detailed {
		var entries []Entry
		err := coll.Find(bson.M{"type": "bookmark"}).All(&entries)
		if err != nil {
			result.Error = fmt.Sprintf("failed getting stats/bookmarks count: %s", err)
			return nil
		}
		result.Bookmarks = len(entries)
		err = coll.Find(bson.M{"type": "pim"}).All(&entries)
		if err != nil {
			result.Error = fmt.Sprintf("failed getting stats/pim count: %s", err)
			return nil
		}
		result.Pim = len(entries)
		err = coll.Find(bson.M{"type": "org"}).All(&entries)
		if err != nil {
			result.Error = fmt.Sprintf("failed getting stats/org count: %s", err)
			return nil
		}
		result.Org = len(entries)
	}
	return nil
}

// Delete ... Remove 1+ selected entries
func (s *Service) Delete(r *http.Request, args *DeleteEntryArgs, result *DeleteResponse) error {
	if args.UserID == "" {
		result.Error = uidMissing
		result.Deleted = -1
		return nil
	}
	coll := s.Session.DB(MentatDatabase).C(args.UserID)
	UUIDsToDelete := args.UUIDs
	if len(UUIDsToDelete) > 0 {
		if len(UUIDsToDelete) > BatchDeleteThreshold {
			changed, err := coll.RemoveAll(bson.M{"type": bson.M{"$in": args.UUIDs}})
			if err != nil {
				result.Error = fmt.Sprintf("cleanup failed: %s", err)
				result.Deleted = -1
				return nil
			}
			result.Deleted = changed.Removed
			return nil
		}
		deletedCount := 0
		var failedEntries []string
		for _, uuid := range UUIDsToDelete {
			err := coll.Remove(bson.M{"uuid": uuid})
			if err == nil {
				deletedCount++
			} else {
				failedEntries = append(failedEntries, uuid)
			}
		}
		if len(failedEntries) > 0 {
			result.Error = fmt.Sprintf("failed to delete entries: %s", strings.Join(failedEntries, ", "))
		}
		result.Deleted = deletedCount
		return nil
	}
	result.Error = "No UUIDs provided"
	result.Deleted = -1
	return nil
}

// Search ... Search entries
func (s *Service) Search(r *http.Request, args *SearchEntryArgs, result *SearchResponse) error {
	// TODO: add metadata searching
	// TODO: add fuzzy tags searching
	if args.UserID == "" {
		result.Error = uidMissing
		result.Entries = []Entry{}
		result.Count = 0
		return nil
	}
	var searchClauses []bson.M
	var searchQuery bson.M
	var entries []Entry
	entryTypes := []string{EntryTypePim, EntryTypeBookmark, EntryTypeOrg}
	if len(args.Types) > 0 {
		entryTypes = args.Types
	}
	var entryTypesClauses []bson.M
	for _, entryType := range entryTypes {
		entryTypesClauses = append(entryTypesClauses, bson.M{"type": entryType})
	}
	searchClauses = append(searchClauses, bson.M{"$or": entryTypesClauses})
	if args.Content != "" {
		var entryContentClauses []bson.M
		entryContentClauses = append(entryContentClauses, bson.M{"content": bson.M{"$regex": args.Content, "$options": "i"}})
		entryContentClauses = append(entryContentClauses, bson.M{"metadata.description": bson.M{"$regex": args.Content, "$options": "i"}})
		searchClauses = append(searchClauses, bson.M{"$or": entryContentClauses})
	}
	if len(args.Tags) > 0 {
		searchClauses = append(searchClauses, bson.M{"tags": bson.M{"$in": args.Tags}})
	}
	if args.Priority != "" {
		searchClauses = append(searchClauses, bson.M{"priority": args.Priority})
	}
	coll := s.Session.DB(MentatDatabase).C(args.UserID)
	if len(searchClauses) > 0 {
		searchQuery = bson.M{"$and": searchClauses}
	}
	mgoErr := coll.Find(searchQuery).All(&entries)
	if mgoErr != nil {
		if mgoErr.Error() == MongoNotFound {
			result.Entries = []Entry{}
			result.Count = 0
			return nil
		}
		s.Log.Infof("mgo error: %s", mgoErr)
		result.Error = fmt.Sprintf("mgo error: %s", mgoErr)
		result.Entries = []Entry{}
		result.Count = 0
		return nil
	}
	result.Entries = entries
	result.Count = len(entries)
	return nil
}
