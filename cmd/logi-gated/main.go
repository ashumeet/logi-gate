package main

import (
	"log"
	"runtime"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("logi-gated starting")

	cfg := LoadConfig()
	srv := NewServer(cfg)

	go srv.Serve()

	StartDisplayWatcher()
	StartEventTap(cfg)
}
