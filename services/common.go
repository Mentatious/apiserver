package services

import (
	"fmt"
	"go.uber.org/zap"
	"gopkg.in/mgo.v2"
	"os"
)

// BaseService ... basic RPC service definition
type BaseService struct {
	name    string
	Log     *zap.SugaredLogger
	Session *mgo.Session
}

// Init ... initialize service
func (s *BaseService) Init(name, dbhost string, log *zap.SugaredLogger) {
	s.name = name
	s.Log = log
	s.Log.Infof("Initializing '%s' service...", s.name)
	session, err := mgo.Dial(dbhost)
	if err != nil {
		fmt.Printf("%s, exiting...\n", err)
		os.Exit(1)
	}
	s.Session = session
	s.Session.SetMode(mgo.Monotonic, true)
}

// Destroy ... destroy service
func (s *BaseService) Destroy() {
	s.Log.Infof("Destroying '%s' service...", s.name)
	s.Session.Close()
}
