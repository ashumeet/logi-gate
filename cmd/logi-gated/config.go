package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

var ConfigPath = func() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "/tmp/logigate-config.json"
	}
	return filepath.Join(h, "Library", "Application Support", "LogiGate", "config.json")
}()

// All corner + edge zones. Corners are checked before edges in detectZone.
var ValidTriggers = []string{
	"top_left", "top_right", "bottom_left", "bottom_right",
	"top_edge", "bottom_edge", "left_edge", "right_edge",
}

// Buckets are the display configurations LogiGate auto-detects, keyed by how
// many external displays are attached. Every real setup lands in exactly one.
const (
	BucketNone  = "none"  // 0 external displays (laptop alone)
	BucketOne   = "one"   // exactly 1 external display
	BucketMulti = "multi" // 2 or more external displays
)

var Buckets = []string{BucketNone, BucketOne, BucketMulti}

// BucketFor maps an external-display count to its bucket.
func BucketFor(externalCount int) string {
	switch {
	case externalCount == 0:
		return BucketNone
	case externalCount == 1:
		return BucketOne
	default:
		return BucketMulti
	}
}

// Trigger binds one corner/edge zone to a target channel. Zone and Channel are
// independent — either can be chosen first, in any order, and both persist.
// Off is an explicit toggle: when true the trigger doesn't fire even though its
// zone/channel remain set, so the user can flip it off and back on without
// losing the selection.
type Trigger struct {
	Zone    string `json:"zone"`
	Channel int    `json:"channel"`
	Off     bool   `json:"off"`
}

// complete reports whether both zone and channel are chosen (regardless of Off).
func (t Trigger) complete() bool {
	return isValidTrigger(t.Zone) && t.Channel >= 1 && t.Channel <= 3
}

// valid (fires) requires a complete binding that is not toggled off.
func (t Trigger) valid() bool {
	return t.complete() && !t.Off
}

// Setup is one display configuration: up to two trigger slots. Slot 0 is
// "Trigger 1". A setup is "armed" purely as a function of its triggers — if any
// slot is set it fires, otherwise it's inert. There is no separate armed flag;
// the master Enable toggle is the only global override.
type Setup struct {
	Triggers [2]Trigger `json:"triggers"`
}

// Armed reports whether this setup has at least one usable trigger.
func (s *Setup) Armed() bool {
	for _, t := range s.Triggers {
		if t.valid() {
			return true
		}
	}
	return false
}

type Config struct {
	Enabled bool              `json:"enabled"`
	DwellMs int               `json:"dwell_ms"`
	CoolMs  int               `json:"cooldown_ms"`
	Setups  map[string]*Setup `json:"setups"`

	// Legacy single-trigger fields (pre-Setups). Read once for migration, then
	// cleared. Kept in the struct so old configs unmarshal cleanly.
	LegacyTrigger string `json:"trigger,omitempty"`
	LegacyChannel int    `json:"channel,omitempty"`

	mu   sync.Mutex               // guards mutation of the fields above
	snap atomic.Pointer[Snapshot] // lock-free hot-path view, rebuilt on mutation
}

func DefaultConfig() *Config {
	c := &Config{
		Enabled: true,
		DwellMs: 200,
		CoolMs:  1000,
		// Every setup starts empty (no triggers → inert). The user configures
		// each one independently in the menu; shipping identical defaults made
		// distinct setups look like clones, so we ship none.
		Setups: map[string]*Setup{
			BucketNone:  {},
			BucketOne:   {},
			BucketMulti: {},
		},
	}
	return c
}

func LoadConfig() *Config {
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		c := DefaultConfig()
		c.Save()
		c.publish()
		return c
	}
	c := &Config{}
	if err := json.Unmarshal(data, c); err != nil {
		d := DefaultConfig()
		d.Save()
		d.publish()
		return d
	}
	// Only rewrite the file when migration/repair actually changed something.
	if c.normalize() {
		c.Save()
	}
	c.publish()
	return c
}

// normalize migrates legacy configs and repairs invalid state in place. Returns
// true if it mutated the config (so the caller persists only when needed).
func (c *Config) normalize() bool {
	before, _ := json.Marshal(c)

	if c.DwellMs == 0 {
		c.DwellMs = 200
	}
	if c.CoolMs == 0 {
		c.CoolMs = 1000
	}

	if c.Setups == nil {
		c.Setups = map[string]*Setup{}
	}

	// Ensure every bucket exists (all start empty/inert).
	for _, b := range Buckets {
		if c.Setups[b] == nil {
			c.Setups[b] = &Setup{}
		}
	}

	// Migrate a legacy single trigger→channel. It fired only with one external
	// display, so it maps onto the "one" bucket's first trigger slot.
	if isValidTrigger(c.LegacyTrigger) && c.LegacyChannel >= 1 && c.LegacyChannel <= 3 {
		c.Setups[BucketOne].Triggers[0] = Trigger{Zone: c.LegacyTrigger, Channel: c.LegacyChannel}
	}
	c.LegacyTrigger = ""
	c.LegacyChannel = 0

	// Repair each setup's triggers. Preserve partial (zone-only or channel-only)
	// and toggled-off selections — the user builds a trigger incrementally and
	// can flip it off without losing it. Only sanitize a genuinely-unknown zone
	// (drop it, keeping the channel), and de-duplicate zones across the two
	// slots by comparing only COMPLETE bindings.
	for _, b := range Buckets {
		s := c.Setups[b]
		seen := map[string]bool{}
		for i := range s.Triggers {
			t := &s.Triggers[i]
			if t.Zone != "" && !isValidTrigger(t.Zone) {
				t.Zone = "" // unknown zone name — clear it, keep the channel
			}
			if t.complete() {
				if seen[t.Zone] {
					t.Zone = "" // same zone already bound in the other slot
				} else {
					seen[t.Zone] = true
				}
			}
		}
	}

	after, _ := json.Marshal(c)
	return string(before) != string(after)
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
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked()
}

// saveLocked persists the config; caller must hold c.mu.
func (c *Config) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(ConfigPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath, data, 0644)
}

// SetupView is an immutable per-bucket view for the event tap's hot path.
type SetupView struct {
	Zones map[string]int // zone → channel, only valid triggers
}

// Armed: a setup fires iff it has at least one bound zone.
func (v SetupView) Armed() bool { return len(v.Zones) > 0 }

// Snapshot is a lock-free, immutable view of config. Rebuilt on every mutation
// and read on the mouse-move hot path via an atomic pointer (zero allocation).
type Snapshot struct {
	Enabled bool
	DwellMs int
	CoolMs  int
	Setups  map[string]SetupView
}

// publish rebuilds the cached snapshot from current fields. Caller need not
// hold the lock, but callers that mutate should hold c.mu while calling it.
func (c *Config) publish() {
	views := make(map[string]SetupView, len(c.Setups))
	for b, s := range c.Setups {
		zones := map[string]int{}
		for _, t := range s.Triggers {
			if t.valid() {
				zones[t.Zone] = t.Channel
			}
		}
		views[b] = SetupView{Zones: zones}
	}
	c.snap.Store(&Snapshot{
		Enabled: c.Enabled,
		DwellMs: c.DwellMs,
		CoolMs:  c.CoolMs,
		Setups:  views,
	})
}

// Get returns the cached snapshot — a lock-free pointer load, safe to call on
// every mouse event.
func (c *Config) Get() *Snapshot {
	return c.snap.Load()
}

// --- Mutators. Each mutates under the lock, republishes the snapshot, and
// persists synchronously so on-disk state and the hot-path view never diverge.

func (c *Config) IsEnabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Enabled
}

// SetupTriggers returns a bucket's two trigger slots in order (slot 1, slot 2).
func (c *Config) SetupTriggers(bucket string) [2]Trigger {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.Setups[bucket]; ok {
		return s.Triggers
	}
	return [2]Trigger{}
}

func (c *Config) SetEnabled(v bool) {
	c.mu.Lock()
	c.Enabled = v
	c.publish()
	_ = c.saveLocked()
	c.mu.Unlock()
}

// SetTriggerZone sets the zone for a trigger slot (1 or 2) within a bucket.
// Choosing a zone also un-toggles Off (an explicit selection means "use it").
func (c *Config) SetTriggerZone(bucket string, slot int, zone string) bool {
	if !isValidTrigger(zone) {
		return false
	}
	return c.mutateTrigger(bucket, slot, func(t *Trigger) { t.Zone = zone; t.Off = false })
}

// SetTriggerChannel sets the channel for a trigger slot (1 or 2) within a bucket.
// Choosing a channel also un-toggles Off.
func (c *Config) SetTriggerChannel(bucket string, slot, channel int) bool {
	if channel < 1 || channel > 3 {
		return false
	}
	return c.mutateTrigger(bucket, slot, func(t *Trigger) { t.Channel = channel; t.Off = false })
}

// SetTriggerOff toggles a trigger's Off flag WITHOUT discarding its zone/channel,
// so the user can disable a fully-configured trigger and re-enable it later.
func (c *Config) SetTriggerOff(bucket string, slot int, off bool) bool {
	return c.mutateTrigger(bucket, slot, func(t *Trigger) { t.Off = off })
}

func (c *Config) mutateTrigger(bucket string, slot int, fn func(*Trigger)) bool {
	if slot < 1 || slot > 2 {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.Setups[bucket]
	if !ok {
		return false
	}
	fn(&s.Triggers[slot-1])
	c.publish()
	_ = c.saveLocked()
	return true
}
