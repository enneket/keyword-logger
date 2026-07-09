package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"keyword-logger/internal/api"
	"keyword-logger/internal/counter"
	"keyword-logger/internal/persist"
	"keyword-logger/internal/recorder"
	"keyword-logger/internal/window"
)

func main() {
	port := flag.Int("port", 5700, "HTTP API port")
	storePath := flag.String("store", "", "data file path")
	flag.Parse()

	if *storePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("cannot get home dir: %v", err)
		}
		*storePath = filepath.Join(home, ".local", "share", "keyword-logger", "stats.json")
	}

	c := counter.New()

	if err := persist.LoadFromFile(*storePath, c); err != nil {
		log.Printf("warning: failed to load existing stats: %v", err)
	}

	tracker, err := window.New()
	if err != nil {
		log.Fatalf("window tracker: %v", err)
	}
	defer tracker.Close()

	tracker.Refresh()

	rec := recorder.New(c, tracker)

	if err := rec.Start(); err != nil {
		log.Fatalf("recorder: %v", err)
	}

	saver := persist.New(*storePath, 10*time.Second, c)
	go saver.Start()

	apiServer := api.New(*port, c)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nshutting down...")
		saver.Stop()
		rec.Stop()
		apiServer.Stop()
		os.Exit(0)
	}()

	fmt.Printf("keyword-logger running on http://127.0.0.1:%d\n", *port)
	fmt.Printf("data stored at %s\n", *storePath)

	if err := apiServer.Start(); err != nil {
		log.Fatalf("api server: %v", err)
	}
}
