package main

/*
#cgo CFLAGS: -x objective-c -Wno-deprecated-declarations
#cgo LDFLAGS: -framework ApplicationServices

#include <ApplicationServices/ApplicationServices.h>

static int hasListenEventAccess() {
    return CGPreflightListenEventAccess() ? 1 : 0;
}
*/
import "C"

import (
	"log"
	"os"
	"time"
)

// StartPermissionWatcher polls Input Monitoring grant state.
// If it transitions from denied to granted (or starts granted but the
// tap hasn't seen events for a while), the process exits so launchd
// (KeepAlive=true) can respawn — recreating the event tap with the
// new permission, no manual kickstart needed.
func StartPermissionWatcher() {
	go func() {
		startGranted := C.hasListenEventAccess() == 1
		log.Printf("Input Monitoring at startup: granted=%v", startGranted)

		// If we started without permission, watch for the grant.
		if !startGranted {
			// A process that launched while DENIED can keep reading a cached
			// "denied" from CGPreflightListenEventAccess even after the user
			// flips the toggle ON — so polling alone may never observe the
			// grant. Poll briefly (covers the case where preflight does update
			// live), then exit so launchd (KeepAlive + ThrottleInterval≈10s)
			// respawns into a FRESH process that reads the current grant
			// correctly at startup. This makes "enable in Settings → it just
			// works" true with no manual restart.
			deadline := time.Now().Add(8 * time.Second)
			for {
				time.Sleep(2 * time.Second)
				if C.hasListenEventAccess() == 1 {
					log.Println("Input Monitoring just granted — exiting so launchd respawns with a live tap")
					os.Exit(0)
				}
				if time.Now().After(deadline) {
					log.Println("Input Monitoring still denied — exiting to force a fresh permission read on respawn")
					os.Exit(0)
				}
			}
		}

		// We started granted — but if the tap is silently dead (zero events
		// in 60s while permission is true), respawn anyway to self-heal.
		time.Sleep(60 * time.Second)
		if eventCount == 0 {
			log.Println("permission granted but no events in 60s — respawning to rebind tap")
			os.Exit(0)
		}
	}()
}
