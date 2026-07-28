package main

/*
#cgo CFLAGS: -x objective-c -Wno-deprecated-declarations
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics

#include <ApplicationServices/ApplicationServices.h>

// Returns up to 16 active display IDs via `ids`, count in `outCount`.
static void listDisplays(uint32_t* ids, uint32_t* outCount) {
    uint32_t count = 0;
    CGGetActiveDisplayList(16, ids, &count);
    *outCount = count;
}

static int isBuiltin(uint32_t id) { return CGDisplayIsBuiltin(id) ? 1 : 0; }

static void displayBounds(uint32_t id, double* x, double* y, double* w, double* h) {
    CGRect b = CGDisplayBounds(id);
    *x = b.origin.x; *y = b.origin.y;
    *w = b.size.width; *h = b.size.height;
}

extern void goDisplayChanged(void);

static void reconfigCallback(CGDirectDisplayID display, CGDisplayChangeSummaryFlags flags, void* userInfo) {
    goDisplayChanged();
}

static void startDisplayWatcher() {
    CGDisplayRegisterReconfigurationCallback(reconfigCallback, NULL);
}
*/
import "C"

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// Rect is a display's bounds in global CG coordinates.
type Rect struct {
	X, Y, W, H float64
}

type DisplayState struct {
	ExternalCount int    // number of external (non-builtin) displays — selects the setup bucket
	Displays      []Rect // bounds of ALL active displays (builtin + external)
}

// DisplayUnderCursor returns the display rectangle containing the global point,
// and whether one was found. It considers ALL active displays (builtin AND
// external) so corner/edge detection works on the laptop's own screen when
// there are no externals (the "none"/0-external setup), and on whichever
// monitor the cursor is on when there are.
func (s DisplayState) DisplayUnderCursor(gx, gy float64) (Rect, bool) {
	for _, r := range s.Displays {
		if gx >= r.X && gy >= r.Y && gx <= r.X+r.W && gy <= r.Y+r.H {
			return r, true
		}
	}
	return Rect{}, false
}

var (
	displayMu    sync.RWMutex
	displayState DisplayState
	changeTick   int64
)

func GetDisplayState() DisplayState {
	displayMu.RLock()
	defer displayMu.RUnlock()
	return displayState
}

func recomputeDisplay() {
	var ids [16]C.uint32_t
	var count C.uint32_t
	C.listDisplays(&ids[0], &count)

	rects := make([]Rect, 0, int(count))
	externalCount := 0
	for i := 0; i < int(count); i++ {
		if C.isBuiltin(ids[i]) == 0 {
			externalCount++
		}
		// Store bounds for EVERY active display (builtin + external) so corner
		// detection works on the laptop screen too. The bucket is still chosen
		// by externalCount; only the cursor hit-test spans all displays.
		var x, y, w, h C.double
		C.displayBounds(ids[i], &x, &y, &w, &h)
		rects = append(rects, Rect{X: float64(x), Y: float64(y), W: float64(w), H: float64(h)})
	}

	st := DisplayState{
		ExternalCount: externalCount,
		Displays:      rects,
	}

	displayMu.Lock()
	prev := displayState.ExternalCount
	displayState = st
	displayMu.Unlock()

	if prev != st.ExternalCount {
		log.Printf("display state: externals=%d bucket=%s", st.ExternalCount, BucketFor(st.ExternalCount))
	}
}

//export goDisplayChanged
func goDisplayChanged() {
	// Reconfig callback fires twice (before + after). Debounce on a short goroutine.
	tick := atomic.AddInt64(&changeTick, 1)
	go func(myTick int64) {
		time.Sleep(500 * time.Millisecond)
		if atomic.LoadInt64(&changeTick) != myTick {
			return
		}
		recomputeDisplay()
	}(tick)
}

func StartDisplayWatcher() {
	recomputeDisplay()
	C.startDisplayWatcher()
	// Fallback for a missed reconfiguration callback. The daemon launches at
	// boot via RunAtLoad, often BEFORE the displays finish enumerating, so the
	// first recomputeDisplay sees 0 externals and computes Qualified=false. The
	// CGDisplayRegisterReconfigurationCallback is meant to correct this once the
	// displays settle, but during boot (and the permission-respawn churn) that
	// event can fire before registration completes or be lost — leaving the
	// daemon stuck at Qualified=false until a manual restart, so the corner
	// trigger never fires. Re-evaluate on a low-frequency ticker; recomputeDisplay
	// only updates/logs on change, so this is quiet and self-heals the boot race.
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for range t.C {
			recomputeDisplay()
		}
	}()
	_ = unsafe.Pointer(nil)
}
