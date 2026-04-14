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

type DisplayState struct {
	Qualified bool    // exactly one external display is connected
	X, Y      float64 // external display origin in global CG coords
	W, H      float64 // external display size
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

	externals := []C.uint32_t{}
	for i := 0; i < int(count); i++ {
		if C.isBuiltin(ids[i]) == 0 {
			externals = append(externals, ids[i])
		}
	}

	var st DisplayState
	if len(externals) == 1 {
		var x, y, w, h C.double
		C.displayBounds(externals[0], &x, &y, &w, &h)
		st = DisplayState{
			Qualified: true,
			X:         float64(x), Y: float64(y),
			W: float64(w), H: float64(h),
		}
	}

	displayMu.Lock()
	prev := displayState
	displayState = st
	displayMu.Unlock()

	if prev != st {
		log.Printf("display state: qualified=%v externals=%d bounds=(%.0f,%.0f %.0fx%.0f)",
			st.Qualified, len(externals), st.X, st.Y, st.W, st.H)
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
	_ = unsafe.Pointer(nil)
	// Safety-net poll — callback can miss events depending on runloop state.
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for range t.C {
			recomputeDisplay()
		}
	}()
}
