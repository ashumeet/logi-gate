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

	enabled, dwellMs, cooldownMs, activeTrigger, channel := t.cfg.Get()
	disp := GetDisplayState()

	if !enabled || !disp.Qualified {
		t.mu.Lock()
		t.inZone = ""
		t.mu.Unlock()
		return
	}

	// Translate global CG coords → external display local coords.
	lx := gx - disp.X
	ly := gy - disp.Y
	// Outside the external display → no zone.
	if lx < 0 || ly < 0 || lx > disp.W || ly > disp.H {
		t.mu.Lock()
		t.inZone = ""
		t.mu.Unlock()
		return
	}

	rawZone := detectZone(lx, ly, disp.W, disp.H)
	// Only the single active trigger fires.
	zone := ""
	if rawZone == activeTrigger {
		zone = rawZone
	}
	now := time.Now()

	t.mu.Lock()
	if zone == t.inZone {
		t.mu.Unlock()
		return
	}
	// Zone transition.
	if zone != "" {
		log.Printf("zone enter: %s (local %.0f,%.0f in %.0fx%.0f)", zone, gx-disp.X, gy-disp.Y, disp.W, disp.H)
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
