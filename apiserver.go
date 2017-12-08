package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/satori/go.uuid"
	"github.com/simonleung8/flags"
	"go.uber.org/zap"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

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

// Mentat DB traits
const (
	MentatDatabase   = "mentat-database"
	MentatCollection = "mentat-entries"
	DatetimeLayout   = "0000-00-00T00:00:00.000Z" // TODO: maybe use appropriate constant from "time" module
	mongoNotFound    = "not found"
)

// EntryService ... Entries RPC service
type EntryService struct {
	DBHost  string
	log     *zap.SugaredLogger
	session *mgo.Session
}

// Init ... initialize EntryService
func (s *EntryService) Init(dbhost string, log *zap.SugaredLogger) {
	s.log = log
	s.log.Infof("Initializing EntryService...")
	session, err := mgo.Dial(dbhost)
	if err != nil {
		panic(err)
	}
	s.session = session
	s.session.SetMode(mgo.Monotonic, true)
}

// Destroy ... destroy EntryService instance
func (s *EntryService) Destroy() {
	s.log.Infof("Destroying EntryService...")
	s.session.Close()
}

// AddEntryArgs ... args for AddEntry method
type AddEntryArgs struct {
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

// Add ... add entry to DB
func (s *EntryService) Add(r *http.Request, args *AddEntryArgs, result *AddResponse) error {
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
	s.log.Infof("received '%s' entry: '%s'", entryType, content)

	coll := s.session.DB(MentatDatabase).C(MentatCollection)

	entry := Entry{}
	mgoErr := coll.Find(bson.M{"content": content}).One(&entry)
	if mgoErr != nil {
		if mgoErr.Error() == mongoNotFound {
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
				s.log.Infof("failed to insert entry: %s", mgoErr.Error())
				result.Message = fmt.Sprintf("failed to insert entry: %s", mgoErr.Error())
				return nil
			}
			result.Message = entry.UUID
			return nil
		}
		s.log.Infof("mgo error: %s", mgoErr)
		result.Message = fmt.Sprintf("mgo error: %s", mgoErr)
		return nil
	}
	result.Message = "Already exists, skipping"
	return nil
}

// InitLogging ... Initialize loggers
func InitLogging(debug bool, showLoc bool) (*zap.Logger, *zap.SugaredLogger) {
	var rawlog *zap.Logger
	var log *zap.SugaredLogger
	var cfg zap.Config
	var err error
	if debug {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}
	cfg.DisableCaller = !showLoc
	rawlog, err = cfg.Build()
	if err != nil {
		panic(err)
	}
	log = rawlog.Sugar()
	return rawlog, log
}

func main() {
	_, log := InitLogging(false, false)

	fc := flags.New()
	fc.NewStringFlag("host", "h", "host to listen")
	fc.NewStringFlag("port", "p", "port to listen")
	fc.NewStringFlag("dbhost", "d", "DB host")
	fc.Parse(os.Args...)

	host := fc.String("host")
	port := fc.String("port")
	dbhost := fc.String("dbhost")

	if port == "" {
		log.Infof("No port to listen, exiting....")
		os.Exit(1)
	} else if dbhost == "" {
		log.Infof("No dbhost to connect, exiting....")
		os.Exit(1)
	}

	bindAddress := fmt.Sprintf("%s:%s", host, port)
	log.Infof("listening on %s", bindAddress)

	rpcServer := rpc.NewServer()

	rpcServer.RegisterCodec(json2.NewCodec(), "application/json")
	rpcServer.RegisterCodec(json2.NewCodec(), "application/json;charset=UTF-8")

	entryAPI := new(EntryService)
	entryAPI.Init(dbhost, log)
	defer entryAPI.Destroy()

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		entryAPI.Destroy()
		fmt.Println("Caught ^C, exiting...")
		os.Exit(1)
	}()

	rpcServer.RegisterService(entryAPI, "entry")

	router := mux.NewRouter()

	router.Handle("/mentat/v1/", rpcServer)

	http.ListenAndServe(bindAddress, router)
}
