package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed bin/hidapitester
var engineBinary []byte

type Manager struct {
	Devices    []ManagedDevice
	enginePath string
}

type ManagedDevice struct {
	Name      string
	PID       string
	SwitchIdx string
}

func NewManager() (*Manager, error) {
	enginePath := "/usr/local/bin/logigate-engine"
	if _, err := os.Stat(enginePath); os.IsNotExist(err) {
		enginePath = filepath.Join(os.TempDir(), "logigate-engine")
		_ = os.WriteFile(enginePath, engineBinary, 0755)
	}

	m := &Manager{enginePath: enginePath}
	
	cmd := exec.Command("sudo", enginePath, "--list-detail")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return m, fmt.Errorf("failed to list devices: %w", err)
	}

	m.Devices = discoverDevices(string(out), enginePath)

	return m, nil
}

func discoverDevices(output string, enginePath string) []ManagedDevice {
	var devices []ManagedDevice
	lines := strings.Split(output, "\n")
	
	var currentPID, currentName string

	for _, line := range lines {
		if strings.Contains(line, "046D/") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				currentName = strings.TrimSpace(parts[1])
				idParts := strings.Split(parts[0], "/")
				if len(idParts) > 1 {
					currentPID = idParts[1]
				}
			}
		}

		if currentPID != "" && strings.Contains(line, "usagePage:     0xFF43") {
			idx, err := probeFeatureIndex(currentPID, enginePath)
			// CRITICAL FIX: If idx is empty or "0x00", it means the device 
			// does NOT support Easy-Switch (like Litra Glow).
			if err == nil && idx != "" && idx != "0x00" {
				devices = append(devices, ManagedDevice{
					Name:      currentName,
					PID:       currentPID,
					SwitchIdx: idx,
				})
			}
			currentPID = "" 
		}
	}
	return devices
}

func probeFeatureIndex(pid string, enginePath string) (string, error) {
	// 1. Fallback for your specific devices (The 'Gold Standard')
	if pid == "B034" { return "0x0A", nil } // MX Master 3S
	if pid == "B364" { return "0x09", nil } // ERGO K860

	// 2. Dynamic Discovery for other devices
	payload := "0x11,0xFF,0x00,0x00,0x1E,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00"
	cmd := exec.Command("sudo", enginePath, "--vidpid", "046D:"+pid, "--usage", "0x0202", "--usagePage", "0xFF43", "--open", "--length", "20", "--send-output", payload, "--read-input", "20")
	out, _ := cmd.CombinedOutput()

	outputStr := string(out)
	if idx := strings.Index(outputStr, "read 20 bytes:"); idx != -1 {
		hexPart := outputStr[idx+len("read 20 bytes:"):]
		parts := strings.Fields(hexPart)
		if len(parts) >= 5 {
			return "0x" + strings.ToUpper(parts[4]), nil
		}
	}
	return "", fmt.Errorf("not supported")
}

func (m *Manager) SwitchAll(channel uint8) error {
	hexChan := fmt.Sprintf("0x%02X", channel-1)
	for _, d := range m.Devices {
		fmt.Printf("Switching %s (Idx: %s) -> %d... ", d.Name, d.SwitchIdx, channel)
		payload := fmt.Sprintf("0x11,0xFF,%s,0x1E,%s,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00", d.SwitchIdx, hexChan)
		exec.Command("sudo", m.enginePath, "--vidpid", "046D:"+d.PID, "--usage", "0x0202", "--usagePage", "0xFF43", "--open", "--length", "20", "--send-output", payload).Run()
		fmt.Println("DONE")
	}
	return nil
}

func PrintStatus() {
	m, _ := NewManager()
	if len(m.Devices) == 0 {
		fmt.Println("No Logitech Easy-Switch devices detected.")
		return
	}
	fmt.Printf("Detected %d Sync-Ready Device(s):\n", len(m.Devices))
	for _, d := range m.Devices {
		fmt.Printf("- %s (PID: %s, Feature Index: %s)\n", d.Name, d.PID, d.SwitchIdx)
	}
}
