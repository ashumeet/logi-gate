package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// indexCache persists resolved ChangeHost feature indices keyed by device PID.
// The index is a property of the device model/firmware, so it is stable across
// reboots and HID path changes and is shared by identical units (e.g. two of
// the same mouse). Caching it lets a switch skip the ~470ms-per-device probe —
// the probe only runs on a cache miss (a device model never seen before).
//
// Format: {"B034":"0x0A","B359":"0x08"}

var CachePath = func() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "/tmp/logigate-index-cache.json"
	}
	return filepath.Join(h, "Library", "Application Support", "LogiGate", "index-cache.json")
}()

type indexCache struct {
	mu      sync.Mutex
	entries map[string]string // PID → SwitchIdx (e.g. "0x0A")
	dirty   bool
}

func loadIndexCache() *indexCache {
	c := &indexCache{entries: map[string]string{}}
	data, err := os.ReadFile(CachePath)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c.entries)
	if c.entries == nil {
		c.entries = map[string]string{}
	}
	return c
}

func (c *indexCache) get(pid string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	idx, ok := c.entries[pid]
	return idx, ok
}

func (c *indexCache) put(pid, idx string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries[pid] != idx {
		c.entries[pid] = idx
		c.dirty = true
	}
}

// save writes the cache to disk only if it changed since load.
func (c *indexCache) save() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return
	}
	if err := os.MkdirAll(filepath.Dir(CachePath), 0755); err != nil {
		return
	}
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return
	}
	if os.WriteFile(CachePath, data, 0644) == nil {
		c.dirty = false
	}
}
