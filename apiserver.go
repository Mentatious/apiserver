package main

import (
	"fmt"
	"github.com/Mentatious/mentat-apiserver/services/entry"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/simonleung8/flags"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

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

	entryAPI := new(entry.Service)
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
