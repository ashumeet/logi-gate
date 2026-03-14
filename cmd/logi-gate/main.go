package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: logi-gate [1|2|3|scan]")
		return
	}

	arg := os.Args[1]

	// Handle direct channel numbers (1, 2, 3)
	if arg == "1" || arg == "2" || arg == "3" {
		c, _ := strconv.Atoi(arg)
		executeSwitch(uint8(c))
		return
	}

	switch arg {
	case "scan":
		PrintStatus()
	default:
		fmt.Println("Usage: logi-gate [1|2|3|scan]")
	}
}

func executeSwitch(channel uint8) {
	m, err := NewManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	m.SwitchAll(channel)
}
