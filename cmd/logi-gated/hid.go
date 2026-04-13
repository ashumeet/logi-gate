package main

import (
	"log"
	"os/exec"
)

// LogiGateBin is the existing, working switcher CLI. The daemon only signals it.
// It runs as a child of the daemon (launchd-spawned, already permitted for HID/TCC),
// so the frontmost app is never in the responsibility chain.
const LogiGateBin = "/usr/local/bin/logi-gate"

func Switch(channel int) {
	if channel < 1 || channel > 3 {
		return
	}
	arg := ""
	switch channel {
	case 1:
		arg = "1"
	case 2:
		arg = "2"
	case 3:
		arg = "3"
	}
	if err := exec.Command(LogiGateBin, arg).Run(); err != nil {
		log.Printf("switch %d failed: %v", channel, err)
	}
}
