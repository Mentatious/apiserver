package main

import (
	"fmt"
	"github.com/Mentatious/mentat-apiserver/services/entry"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/wiedzmin/goodies"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	_, log := goodies.InitLogging(false, false)

	app := kingpin.New("mentat-apiserver", "Mentat API server")
	Host := app.Flag("host", "Host to listen").Short('h').String()
	Port := app.Flag("port", "Port to listen").Short('p').Required().String()
	DBHost := app.Flag("dbhost", "DB host").Short('d').Required().String()

	kingpin.MustParse(app.Parse(os.Args[1:]))

	if *Port == "" {
		log.Infof("No port to listen, exiting....")
		os.Exit(1)
	} else if *DBHost == "" {
		log.Infof("No dbhost to connect, exiting....")
		os.Exit(1)
	}

	bindAddress := fmt.Sprintf("%s:%s", *Host, *Port)
	log.Infof("listening on %s", bindAddress)

	rpcServer := rpc.NewServer()

	rpcServer.RegisterCodec(json2.NewCodec(), "application/json")
	rpcServer.RegisterCodec(json2.NewCodec(), "application/json;charset=UTF-8")

	entryAPI := new(entry.Service)
	entryAPI.Init("entry", *DBHost, log)
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
