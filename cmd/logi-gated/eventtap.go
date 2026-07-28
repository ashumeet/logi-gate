package main

/*
#cgo CFLAGS: -x objective-c -Wno-deprecated-declarations
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics -framework Foundation

#include <ApplicationServices/ApplicationServices.h>

extern void goHandleMove(double x, double y);

static CFMachPortRef gTap = NULL;

static CGEventRef tapCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void* refcon) {
    if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
        if (gTap) CGEventTapEnable(gTap, true);
        return event;
    }
    CGPoint p = CGEventGetLocation(event);
    goHandleMove(p.x, p.y);
    return event;
}

static int startTap() {
    CGEventMask mask = CGEventMaskBit(kCGEventMouseMoved)
                     | CGEventMaskBit(kCGEventLeftMouseDragged)
                     | CGEventMaskBit(kCGEventRightMouseDragged);
    gTap = CGEventTapCreate(
        kCGSessionEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionListenOnly,
        mask,
        tapCallback,
        NULL
    );
    if (!gTap) { return 0; }
    CFRunLoopSourceRef src = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, gTap, 0);
    CFRunLoopAddSource(CFRunLoopGetCurrent(), src, kCFRunLoopCommonModes);
    CGEventTapEnable(gTap, true);
    CFRunLoopRun();
    return 1;
}
*/
import "C"

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var firstEvent sync.Once
var eventCount int64

type Tap struct {
	cfg *Config

	mu       sync.Mutex
	inZone   string
	enterAt  time.Time
	lastFire time.Time
	spent    bool // true after firing; cleared when cursor leaves the zone
	timer    *time.Timer
}

var activeTap *Tap

//export goHandleMove
func goHandleMove(x, y C.double) {
	if activeTap == nil {
		return
	}
	activeTap.onMove(float64(x), float64(y))
}

func (t *Tap) onMove(gx, gy float64) {
	firstEvent.Do(func() { log.Printf("event tap: first event received (%.0f, %.0f)", gx, gy) })
	n := atomic.AddInt64(&eventCount, 1)
	if n%2000 == 0 {
		log.Printf("event tap: %d events seen, last=(%.0f,%.0f)", n, gx, gy)
	}

	snap := t.cfg.Get()
	disp := GetDisplayState()
	if snap == nil {
		return
	}

	// Auto-select the setup for the current display configuration.
	setup, ok := snap.Setups[BucketFor(disp.ExternalCount)]
	// Do nothing if: master toggle off, this setup has no triggers, or there are
	// no switchable devices connected (nothing to switch → gate is pointless).
	// DeviceCount()==0 gates only after a successful scan; <0 means "unknown".
	if !snap.Enabled || !ok || !setup.Armed() || DeviceCount() == 0 {
		t.mu.Lock()
		t.inZone = ""
		t.mu.Unlock()
		return
	}

	// Find the display the cursor is currently in (builtin or external). Corners
	// are detected per-monitor so triggers work on any screen — including the
	// laptop's own display when there are no externals (the "none" setup).
	rect, found := disp.DisplayUnderCursor(gx, gy)
	if !found {
		t.mu.Lock()
		t.inZone = ""
		t.mu.Unlock()
		return
	}
	lx := gx - rect.X
	ly := gy - rect.Y

	rawZone := detectZone(lx, ly, rect.W, rect.H)
	// A zone only counts if this setup binds it to a channel.
	zone := ""
	channel := 0
	if rawZone != "" {
		if ch, bound := setup.Zones[rawZone]; bound {
			zone = rawZone
			channel = ch
		}
	}
	dwellMs := snap.DwellMs
	cooldownMs := snap.CoolMs
	now := time.Now()

	t.mu.Lock()
	if zone == t.inZone {
		t.mu.Unlock()
		return
	}
	// Zone transition.
	if zone != "" {
		log.Printf("zone enter: %s -> channel %d (local %.0f,%.0f in %.0fx%.0f)", zone, channel, lx, ly, rect.W, rect.H)
	}
	t.inZone = zone
	t.enterAt = now
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	if zone == "" {
		// Left all zones → re-arm.
		t.spent = false
		t.mu.Unlock()
		return
	}
	if t.spent {
		t.mu.Unlock()
		return
	}
	if channel < 1 || channel > 3 {
		t.mu.Unlock()
		return
	}
	enteredZone := zone
	enteredChannel := channel
	fire := func() {
		t.mu.Lock()
		if t.inZone != enteredZone || t.spent {
			t.mu.Unlock()
			return
		}
		if time.Since(t.lastFire) < time.Duration(cooldownMs)*time.Millisecond {
			t.mu.Unlock()
			return
		}
		t.lastFire = time.Now()
		t.spent = true
		t.timer = nil
		t.mu.Unlock()
		log.Printf("trigger %s -> channel %d", enteredZone, enteredChannel)
		go Switch(enteredChannel)
	}
	if dwellMs <= 0 {
		t.mu.Unlock()
		fire()
		return
	}
	t.timer = time.AfterFunc(time.Duration(dwellMs)*time.Millisecond, fire)
	t.mu.Unlock()
}

func detectZone(x, y, w, h float64) string {
	const corner = 3.0
	const edge = 1.0
	if x <= corner && y >= h-corner {
		return "bottom_left"
	}
	if x >= w-corner && y >= h-corner {
		return "bottom_right"
	}
	if x <= edge {
		return "left_edge"
	}
	if x >= w-edge {
		return "right_edge"
	}
	return ""
}

func StartEventTap(cfg *Config) {
	activeTap = &Tap{cfg: cfg}
	if C.startTap() == 0 {
		log.Fatalf("CGEventTap failed — grant Input Monitoring + Accessibility to logi-gated in System Settings")
	}
}
