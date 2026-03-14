package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

//go:embed bin/hidapitester
var engineBinary []byte

type Manager struct {
	enginePath string
	Devices    []ManagedDevice
}

type ManagedDevice struct {
	Name      string
	PID       string
	Path      string
	SwitchIdx string
}

func NewManager() (*Manager, error) {
	enginePath := "/usr/local/bin/logigate-engine"
	if _, err := os.Stat(enginePath); os.IsNotExist(err) {
		enginePath = "/tmp/logigate-engine"
		_ = os.WriteFile(enginePath, engineBinary, 0755)
	}

	m := &Manager{enginePath: enginePath}

	cmd := exec.Command("sudo", "-n", m.enginePath, "--list-detail")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return m, fmt.Errorf("list failed: %w", err)
	}

	m.Devices = discoverDevices(string(out), m.enginePath)
	return m, nil
}

func discoverDevices(output string, enginePath string) []ManagedDevice {
	var devices []ManagedDevice
	lines := strings.Split(output, "\n")
	var currentPID, currentName, currentPath string

	for _, line := range lines {
		if strings.Contains(line, "046D/") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				currentName = strings.TrimSpace(parts[1])
				idParts := strings.Split(parts[0], "/")
				if len(idParts) > 1 { currentPID = idParts[1] }
			}
		}
		if strings.Contains(line, "path: ") {
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "  path: "))
		}

		if currentPID != "" && strings.Contains(line, "usagePage:     0xFF43") {
			// Fast path: use hardcoded indices based on device PID
			idx := ""
			if currentPID == "B034" { idx = "0x0A" } // MX Master 3S
			if currentPID == "B364" { idx = "0x09" } // ERGO K860
			
			if idx == "" {
				idx, _ = probeFeatureIndex(currentPath, enginePath)
			}

			if idx != "" && idx != "0x00" {
				devices = append(devices, ManagedDevice{
					Name:      currentName,
					PID:       currentPID,
					Path:      currentPath,
					SwitchIdx: idx,
				})
			}
			currentPID = "" 
		}
	}
	return devices
}

func probeFeatureIndex(path string, enginePath string) (string, error) {
	payload := "0x11,0xFF,0x00,0x00,0x1E,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00"
	cmd := exec.Command("sudo", "-n", enginePath, "--open-path", path, "--length", "20", "--send-output", payload, "--read-input", "20")
	out, _ := cmd.CombinedOutput()
	outputStr := string(out)
	if idx := strings.Index(outputStr, "read 20 bytes:"); idx != -1 {
		hexPart := outputStr[idx+len("read 20 bytes:"):]
		parts := strings.Fields(hexPart)
		if len(parts) >= 5 {
			if parts[4] == "00" { return "", nil }
			return "0x" + strings.ToUpper(parts[4]), nil
		}
	}
	return "", nil
}

func (m *Manager) SwitchAll(channel uint8) error {
	hexChan := fmt.Sprintf("0x%02X", channel-1)
	for _, d := range m.Devices {
		fmt.Printf("Switching %s -> %d... ", d.Name, channel)
		// THE VALIDATED PAYLOAD from HARDWARE_PROTOCOL.md: 11 01 [Idx] 1E [Channel]
		payload := fmt.Sprintf("0x11,0x01,%s,0x1E,%s,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00", d.SwitchIdx, hexChan)
		exec.Command("sudo", "-n", m.enginePath, "--open-path", d.Path, "--length", "20", "--send-output", payload).Run()
		time.Sleep(50 * time.Millisecond)
		fmt.Println("DONE")
	}
	return nil
}

func PrintStatus() {
	m, err := NewManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	for _, d := range m.Devices {
		fmt.Printf("- %s (Idx: %s)\n", d.Name, d.SwitchIdx)
	}
}
