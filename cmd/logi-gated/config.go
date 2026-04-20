package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var ConfigPath = func() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "/tmp/logigate-config.json"
	}
	return filepath.Join(h, "Library", "Application Support", "LogiGate", "config.json")
}()

type Config struct {
	Enabled    bool   `json:"enabled"`
	DwellMs    int    `json:"dwell_ms"`
	CooldownMs int    `json:"cooldown_ms"`
	Trigger    string `json:"trigger"`
	Channel    int    `json:"channel"`

	mu sync.RWMutex
}

var ValidTriggers = []string{"bottom_left", "bottom_right", "left_edge", "right_edge"}

func DefaultConfig() *Config {
	return &Config{
		Enabled:    true,
		DwellMs:    200,
		CooldownMs: 1000,
		Trigger:    "bottom_left",
		Channel:    1,
	}
}

func LoadConfig() *Config {
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		c := DefaultConfig()
		c.Save()
		return c
	}
	c := DefaultConfig()
	if err := json.Unmarshal(data, c); err != nil {
		return DefaultConfig()
	}
	if !isValidTrigger(c.Trigger) {
		c.Trigger = "bottom_left"
	}
	if c.Channel < 1 || c.Channel > 3 {
		c.Channel = 1
	}
	return c
}

func isValidTrigger(name string) bool {
	for _, v := range ValidTriggers {
		if v == name {
			return true
		}
	}
	return false
}

func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if err := os.MkdirAll(filepath.Dir(ConfigPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath, data, 0644)
}

func (c *Config) Get() (enabled bool, dwellMs, cooldownMs int, trigger string, channel int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Enabled, c.DwellMs, c.CooldownMs, c.Trigger, c.Channel
}

func (c *Config) SetEnabled(v bool) {
	c.mu.Lock()
	c.Enabled = v
	c.mu.Unlock()
	_ = c.Save()
}

func (c *Config) SetTrigger(name string) bool {
	if !isValidTrigger(name) {
		return false
	}
	c.mu.Lock()
	c.Trigger = name
	c.mu.Unlock()
	_ = c.Save()
	return true
}

func (c *Config) SetChannel(ch int) bool {
	if ch < 1 || ch > 3 {
		return false
	}
	c.mu.Lock()
	c.Channel = ch
	c.mu.Unlock()
	_ = c.Save()
	return true
}
