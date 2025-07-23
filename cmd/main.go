package main

import (
	"context"
	"fileserve/store"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

var defaultDBPath string

func init() {
	p, err := os.UserHomeDir()
	checkError(err)
	defaultDBPath = filepath.Join(p, ".fileserve")
}

func main() {
	var addressFlag = flag.String("address", "127.0.0.1:8080", "The IP address to listen at: address:port")
	var storeFlag = flag.String("store", defaultDBPath, "The directory to store files at")
	flag.Parse()

	httpServer := http.Server{
		Addr: *addressFlag,
	}

	fileStore, err := store.NewSQL3FileStore(*storeFlag)
	checkError(err)

	httpServer.RegisterOnShutdown(func() { fileStore.Close() })
	routeHandlers := NewRoutHandler(fileStore)
	registerHandlers(routeHandlers)

	go func() {
		err = httpServer.ListenAndServe()
		checkError(err)
	}()

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, os.Interrupt)
	<-termChan

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = httpServer.Shutdown(ctx)
	checkError(err)

	os.Exit(0)
}

func checkError(e error) {
	if e != nil {
		fatal(e.Error())
	}
}

func fatal(message string) {
	fmt.Fprintf(os.Stderr, "%s", message)
	os.Exit(1)
}
