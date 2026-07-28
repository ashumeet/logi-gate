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

	cache := loadIndexCache()
	m.Devices = discoverDevices(string(out), m.enginePath, cache)
	cache.save()
	return m, nil
}

func discoverDevices(output string, enginePath string, cache *indexCache) []ManagedDevice {
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
			// Resolve the ChangeHost feature index. It's a stable per-model
			// property, so we cache it by PID: a cache hit skips the ~470ms
			// probe entirely (the whole reason switching was slow). Only a
			// never-seen model pays the probe cost, once.
			idx, ok := cache.get(currentPID)
			if !ok {
				idx, _ = probeFeatureIndex(currentPath, enginePath)
				if idx != "" && idx != "0x00" {
					cache.put(currentPID, idx)
				}
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

// probeFeatureIndex resolves the device's ChangeHost (Easy-Switch) feature
// index via the HID++ 2.0 root-feature query: getFeature(featureId=0x1814).
// 0x1814 is the ChangeHost feature — the one SwitchAll targets to move hosts.
//
// Request:  11 FF 00 00 18 14 ...   (root feature 0x00, func getFeature, arg 0x1814)
// Response: 11 FF 00 00 <IDX> ...   (byte 4 = index of feature 0x1814 on this device)
//
// NB: it MUST query 0x1814, not 0x1E00. Probing 0x1E00 returns the index of a
// different feature (0x1B on the MX Master, 0x1A on the K860) — those are not
// ChangeHost, so a switch sent to them silently no-ops. The correct 0x1814
// query returns 0x0A on the MX Master (matches the hardware-validated value)
// and 0x08 on the K860.
//
// The device's HID input pipe is racy: right after opening the path the first
// report(s) read back are often STALE/unrelated frames, not the reply to our
// query. Trusting the first report is what dropped devices — if a stray frame's
// byte 4 happened to be 0x00 we'd wrongly conclude "no ChangeHost feature".
// So we VALIDATE that the reply echoes our request header (11 FF 00 00) before
// accepting byte 4, and keep reading/retrying until a matching reply arrives.
func probeFeatureIndex(path string, enginePath string) (string, error) {
	payload := "0x11,0xFF,0x00,0x00,0x18,0x14,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00"
	const attempts = 8
	for i := 0; i < attempts; i++ {
		cmd := exec.Command("sudo", "-n", enginePath, "--open-path", path, "--length", "20", "--send-output", payload, "--read-input", "20")
		out, _ := cmd.CombinedOutput()
		if idx, ok := parseProbeReply(string(out)); ok {
			if idx == "0x00" {
				return "", nil // validated reply: device has no Easy-Switch feature
			}
			return idx, nil
		}
		// No valid reply yet (empty read or a stale/unmatched frame). Back off and retry.
		time.Sleep(time.Duration(60*(i+1)) * time.Millisecond)
	}
	return "", nil
}

// parseProbeReply extracts the feature index from a probe response, but only if
// the frame echoes our request header (11 FF 00 00). Returns (idx, true) on a
// matching reply — including "0x00" meaning the feature is genuinely absent —
// or ("", false) when no matching reply is present and the caller should retry.
func parseProbeReply(output string) (string, bool) {
	marker := "read 20 bytes:"
	rest := output
	for {
		i := strings.Index(rest, marker)
		if i == -1 {
			return "", false
		}
		hexPart := rest[i+len(marker):]
		rest = hexPart
		parts := strings.Fields(hexPart)
		if len(parts) < 5 {
			continue
		}
		// Validate the HID++ reply header echoes our request: 11 FF 00 00.
		if !strings.EqualFold(parts[0], "11") ||
			!strings.EqualFold(parts[1], "FF") ||
			!strings.EqualFold(parts[2], "00") ||
			!strings.EqualFold(parts[3], "00") {
			continue // stale/unrelated frame — keep looking
		}
		return "0x" + strings.ToUpper(parts[4]), true
	}
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
