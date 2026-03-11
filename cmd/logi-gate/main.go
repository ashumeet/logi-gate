package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"golang.design/x/hotkey"
	"golang.design/x/hotkey/mainthread"
)

func main() {
	if len(os.Args) < 2 {
		mainthread.Init(runDaemon)
		return
	}

	command := os.Args[1]

	switch command {
	case "scan":
		PrintStatus()
	case "switch":
		if len(os.Args) < 3 {
			fmt.Println("Error: Specify channel (1, 2, or 3)")
			return
		}
		c, _ := strconv.Atoi(os.Args[2])
		executeSwitch(uint8(c))
	case "daemon":
		mainthread.Init(runDaemon)
	default:
		printUsage()
	}
}

func runDaemon() {
	f, _ := os.OpenFile("/tmp/logigate.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	logger := log.New(f, "[DAEMON] ", log.LstdFlags)

	manager, err := NewManager()
	if err != nil {
		fmt.Printf("CRITICAL: Failed to init HID manager: %v\n", err)
		return
	}

	mods := []hotkey.Modifier{hotkey.ModCtrl, hotkey.ModOption, hotkey.ModCmd}
	hk1 := hotkey.New(mods, hotkey.KeyJ)
	hk2 := hotkey.New(mods, hotkey.KeyK)
	hk3 := hotkey.New(mods, hotkey.KeyL)

	// Robust Error Checking for Registration
	if err := hk1.Register(); err != nil {
		fmt.Printf("!!! Failed to register Mac 1 (Ctrl+Opt+Cmd+J): %v\n", err)
		fmt.Println("TIP: Ensure you have granted Accessibility permissions to this binary.")
	}
	if err := hk2.Register(); err != nil {
		fmt.Printf("!!! Failed to register Mac 2 (Ctrl+Opt+Cmd+K): %v\n", err)
	}
	if err := hk3.Register(); err != nil {
		fmt.Printf("!!! Failed to register Mac 3 (Ctrl+Opt+Cmd+L): %v\n", err)
	}

	fmt.Println("LogiGate Daemon: Online and Listening.")
	fmt.Println("  Ctrl+Opt+Cmd+J -> Mac 1")
	fmt.Println("  Ctrl+Opt+Cmd+K -> Mac 2")
	fmt.Println("  Ctrl+Opt+Cmd+L -> Mac 3")

	for {
		var target uint8
		select {
		case <-hk1.Keydown(): target = 1
		case <-hk2.Keydown(): target = 2
		case <-hk3.Keydown(): target = 3
		}

		if target > 0 {
			logger.Printf("Hotkey Pressed: Switch to %d\n", target)
			err := manager.SwitchAll(target)
			if err != nil {
				logger.Printf("Switch failed: %v\n", err)
				fmt.Printf("Switch to %d Failed: %v\n", target, err)
			} else {
				logger.Printf("Switch successful.\n")
				fmt.Printf("Switched all devices to Channel %d\n", target)
			}
		}
	}
}

func executeSwitch(channel uint8) {
	manager, err := NewManager()
	if err != nil { return }
	manager.SwitchAll(channel)
}

func printUsage() {
	fmt.Println("LogiGate Professional")
}
