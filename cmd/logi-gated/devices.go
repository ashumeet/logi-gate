package main

/*
#cgo CFLAGS: -x objective-c -Wno-deprecated-declarations
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation

#include <IOKit/hid/IOHIDManager.h>

extern void goDevicesChanged(void);

static void matchCb(void* ctx, IOReturn r, void* sender, IOHIDDeviceRef d) { goDevicesChanged(); }
static void removeCb(void* ctx, IOReturn r, void* sender, IOHIDDeviceRef d) { goDevicesChanged(); }

// startHIDWatcher registers add/remove callbacks for Logitech (vendor 0x046D)
// HID devices. WindowServer/IOKit pushes an event on hot-plug — no polling.
static void startHIDWatcher() {
    IOHIDManagerRef mgr = IOHIDManagerCreate(kCFAllocatorDefault, kIOHIDOptionsTypeNone);

    int vendor = 0x046D;
    CFNumberRef vid = CFNumberCreate(kCFAllocatorDefault, kCFNumberIntType, &vendor);
    CFStringRef key = CFSTR("VendorID");
    CFDictionaryRef match = CFDictionaryCreate(kCFAllocatorDefault,
        (const void**)&key, (const void**)&vid, 1,
        &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
    IOHIDManagerSetDeviceMatching(mgr, match);
    CFRelease(match);
    CFRelease(vid);

    IOHIDManagerRegisterDeviceMatchingCallback(mgr, matchCb, NULL);
    IOHIDManagerRegisterDeviceRemovalCallback(mgr, removeCb, NULL);
    IOHIDManagerScheduleWithRunLoop(mgr, CFRunLoopGetMain(), kCFRunLoopDefaultMode);
    IOHIDManagerOpen(mgr, kIOHIDOptionsTypeNone);
}
*/
import "C"

import (
	"bufio"
	"log"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Device presence tracking. The trigger is pointless when there are no
// switchable Logitech devices connected, so the daemon greys out and drops
// events in that case — just like the display gate.
//
// Presence is driven by IOKit HID add/remove callbacks (push, no polling).
// Counting itself uses only `logi-gate scan`, which is fast now that feature
// indices are cached (enumeration ~50ms; the slow probe runs only on a cache
// miss). We still keep last-known-good on a failed scan to avoid a false
// "disabled" flicker from a transient error.

var (
	deviceCount   int64 = -1 // -1 = never successfully scanned yet
	deviceScanMu  sync.Mutex
	deviceScanned int32 // 0 until the first successful scan completes
	scanTick      int64
)

// DeviceCount returns the last-known count of switchable devices. Returns -1
// before the first successful scan (callers treat <0 as "unknown, don't gate").
func DeviceCount() int {
	return int(atomic.LoadInt64(&deviceCount))
}

// DevicesReady reports whether at least one successful scan has completed.
func DevicesReady() bool {
	return atomic.LoadInt32(&deviceScanned) == 1
}

// countScannedDevices runs `logi-gate scan` and counts the reported devices.
// Returns (count, true) only when the command ran; (0, false) on exec failure
// so the caller can keep the last-known-good value instead of dropping to 0.
func countScannedDevices() (int, bool) {
	out, err := exec.Command(LogiGateBin, "scan").CombinedOutput()
	if err != nil {
		return 0, false
	}
	n := 0
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		// scan prints one "- <name> (Idx: 0x..)" line per discovered device.
		if strings.HasPrefix(strings.TrimSpace(sc.Text()), "- ") {
			n++
		}
	}
	return n, true
}

// refreshDeviceCount performs one scan and updates the cache on success.
func refreshDeviceCount() {
	deviceScanMu.Lock()
	defer deviceScanMu.Unlock()
	n, ok := countScannedDevices()
	if !ok {
		return // keep last-known-good
	}
	prev := atomic.LoadInt64(&deviceCount)
	atomic.StoreInt64(&deviceCount, int64(n))
	atomic.StoreInt32(&deviceScanned, 1)
	if prev != int64(n) {
		log.Printf("device count: %d switchable device(s) connected", n)
	}
}

//export goDevicesChanged
func goDevicesChanged() {
	// IOKit fires add/remove callbacks in bursts as each interface of a device
	// enumerates. Debounce so we scan once after the burst settles.
	tick := atomic.AddInt64(&scanTick, 1)
	go func(myTick int64) {
		time.Sleep(800 * time.Millisecond)
		if atomic.LoadInt64(&scanTick) != myTick {
			return
		}
		refreshDeviceCount()
	}(tick)
}

// StartDeviceWatcher does one scan at startup, then relies on IOKit HID
// add/remove callbacks to re-scan on hot-plug — no periodic polling.
func StartDeviceWatcher() {
	go refreshDeviceCount()
	C.startHIDWatcher()
}
