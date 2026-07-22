import AppKit
import CoreGraphics
import Foundation

func countExternalDisplays() -> Int {
    var count: UInt32 = 0
    CGGetActiveDisplayList(0, nil, &count)
    if count == 0 { return 0 }
    var ids = [CGDirectDisplayID](repeating: 0, count: Int(count))
    CGGetActiveDisplayList(count, &ids, &count)
    var ext = 0
    for id in ids.prefix(Int(count)) where CGDisplayIsBuiltin(id) == 0 {
        ext += 1
    }
    return ext
}

let SOCKET_PATH: String = {
    return "/tmp/logigate-\(getuid()).sock"
}()
let TRIGGERS = ["bottom_left", "bottom_right", "left_edge", "right_edge"]
let TRIGGER_LABELS: [String: String] = [
    "bottom_left": "Bottom-Left Corner",
    "bottom_right": "Bottom-Right Corner",
    "left_edge": "Left Edge",
    "right_edge": "Right Edge",
]

// One trigger slot: a zone bound to a channel (channel 0 / empty zone = unused).
struct Trigger {
    var zone: String = ""
    var channel: Int = 0
    var isSet: Bool { !zone.isEmpty && channel >= 1 && channel <= 3 }
}

// A Setup = one display configuration (bucket) with up to 2 triggers. It is
// "armed" purely as a function of its triggers — if any slot is set it fires.
// The master Enable button is the only global override.
struct Setup {
    var triggers: [Trigger] = [Trigger(), Trigger()]
    var armed: Bool { triggers.contains { $0.isSet } }
}

let BUCKET_LABELS: [String: String] = [
    "none": "Laptop only (0 external)",
    "one": "1 external display",
    "multi": "2+ external displays",
]
let BUCKET_ORDER = ["none", "one", "multi"]

struct Status {
    var enabled: Bool = false
    var externalCount: Int = 0
    var activeBucket: String = "one"
    var setups: [String: Setup] = [:]

    var activeSetup: Setup? { setups[activeBucket] }
    var armedHere: Bool { activeSetup?.armed ?? false }
    var active: Bool { enabled && armedHere }
}

func sendCommand(_ cmd: String) -> String? {
    let fd = socket(AF_UNIX, SOCK_STREAM, 0)
    if fd < 0 { return nil }
    defer { close(fd) }

    var addr = sockaddr_un()
    addr.sun_family = sa_family_t(AF_UNIX)
    let pathBytes = SOCKET_PATH.utf8CString
    withUnsafeMutablePointer(to: &addr.sun_path) { ptr in
        ptr.withMemoryRebound(to: CChar.self, capacity: 104) { cptr in
            for (i, b) in pathBytes.enumerated() where i < 104 {
                cptr[i] = b
            }
        }
    }
    let size = socklen_t(MemoryLayout<sockaddr_un>.size)
    let rc = withUnsafePointer(to: &addr) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
            Darwin.connect(fd, $0, size)
        }
    }
    if rc != 0 { return nil }

    let line = cmd + "\n"
    _ = line.withCString { ptr -> Int in
        Darwin.send(fd, ptr, strlen(ptr), 0)
    }

    var buf = [UInt8](repeating: 0, count: 4096)
    let n = Darwin.recv(fd, &buf, buf.count, 0)
    if n <= 0 { return "" }
    return String(bytes: buf[0..<n], encoding: .utf8)
}

func fetchStatus() -> Status {
    var s = Status()
    guard let resp = sendCommand("STATUS"),
          let data = resp.data(using: .utf8),
          let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
    else { return s }
    s.enabled = (obj["enabled"] as? Bool) ?? false
    s.externalCount = (obj["external_count"] as? Int) ?? 0
    if let ab = obj["active_bucket"] as? String { s.activeBucket = ab }
    if let setups = obj["setups"] as? [String: Any] {
        for (bucket, raw) in setups {
            guard let sd = raw as? [String: Any] else { continue }
            var setup = Setup()
            if let trigs = sd["triggers"] as? [[String: Any]] {
                for (i, td) in trigs.prefix(2).enumerated() {
                    setup.triggers[i] = Trigger(
                        zone: (td["zone"] as? String) ?? "",
                        channel: (td["channel"] as? Int) ?? 0)
                }
            }
            s.setups[bucket] = setup
        }
    }
    return s
}

class AppDelegate: NSObject, NSApplicationDelegate, NSMenuDelegate {
    var statusItem: NSStatusItem!
    var status = Status()

    func applicationDidFinishLaunching(_ n: Notification) {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        // macOS 27: a status-bar button no longer delivers right-click events to
        // its action (every click arrives as .leftMouseUp), so the old right-click
        // menu could never open. Use the native statusItem.menu — AppKit opens it
        // reliably on a normal click. The on/off toggle now lives in the menu.
        let menu = NSMenu()
        menu.delegate = self
        statusItem.menu = menu
        refresh()
        Timer.scheduledTimer(withTimeInterval: 3.0, repeats: true) { _ in self.refresh() }
        NotificationCenter.default.addObserver(
            forName: NSApplication.didChangeScreenParametersNotification,
            object: nil, queue: .main
        ) { [weak self] _ in self?.refresh() }
    }

    func refresh() {
        status = fetchStatus()
        // Trust local CoreGraphics for the display count — daemon state can lag.
        // Re-derive the active bucket locally so the icon reflects reality.
        status.externalCount = countExternalDisplays()
        status.activeBucket = bucketFor(status.externalCount)
        updateIcon()
    }

    // Mirror the daemon's BucketFor: 0 → none, 1 → one, else → multi.
    func bucketFor(_ count: Int) -> String {
        return count == 0 ? "none" : (count == 1 ? "one" : "multi")
    }

    func updateIcon() {
        guard let btn = statusItem.button else { return }
        let on = status.active
        let disabled = !status.armedHere
        let color: NSColor
        if disabled {
            color = NSColor.tertiaryLabelColor
        } else if on {
            color = .systemBlue
        } else {
            color = .systemGray
        }
        let symbolName = disabled ? "display.slash" : "display"
        let base = NSImage(systemSymbolName: symbolName,
                           accessibilityDescription: "LogiGate")
            ?? NSImage(systemSymbolName: "display", accessibilityDescription: "LogiGate")
        let config = NSImage.SymbolConfiguration(pointSize: 16, weight: .regular)
            .applying(NSImage.SymbolConfiguration(paletteColors: [color]))
        let tinted = base?.withSymbolConfiguration(config)
        tinted?.isTemplate = false
        btn.image = tinted
        btn.alphaValue = disabled ? 0.45 : 1.0
        let tip: String
        let setupName = BUCKET_LABELS[status.activeBucket] ?? status.activeBucket
        if !status.armedHere {
            tip = "LogiGate — off (\(setupName) not armed)"
        } else if status.enabled {
            tip = "LogiGate — on (\(setupName))"
        } else {
            tip = "LogiGate — off"
        }
        btn.toolTip = tip
    }

    // Rebuilt with fresh state every time the menu is about to open. Layout A:
    // the active setup is expanded inline; the other setups live in a submenu.
    func menuNeedsUpdate(_ menu: NSMenu) {
        status = fetchStatus()
        status.externalCount = countExternalDisplays()
        status.activeBucket = bucketFor(status.externalCount)
        menu.removeAllItems()

        // Master enable/disable.
        let enabledItem = NSMenuItem(title: status.enabled ? "Enabled" : "Disabled",
                                     action: #selector(toggleEnabled),
                                     keyEquivalent: "")
        enabledItem.target = self
        enabledItem.state = status.enabled ? .on : .off
        menu.addItem(enabledItem)

        // "Now:" status line — which setup is active and whether it's armed.
        let activeName = BUCKET_LABELS[status.activeBucket] ?? status.activeBucket
        let nowLine = NSMenuItem(
            title: "Now: \(status.externalCount) external → \(activeName)\(status.armedHere ? " · armed" : " · off")",
            action: nil, keyEquivalent: "")
        nowLine.isEnabled = false
        menu.addItem(nowLine)
        menu.addItem(.separator())

        // THIS SETUP — expanded inline.
        let header = NSMenuItem(title: "THIS SETUP  (\(activeName))", action: nil, keyEquivalent: "")
        header.isEnabled = false
        menu.addItem(header)
        appendSetupBody(to: menu, bucket: status.activeBucket, indent: true)
        menu.addItem(.separator())

        // Other setups — each in its own submenu.
        let othersParent = NSMenuItem(title: "Other setups", action: nil, keyEquivalent: "")
        let othersMenu = NSMenu()
        for bucket in BUCKET_ORDER where bucket != status.activeBucket {
            let name = BUCKET_LABELS[bucket] ?? bucket
            let armed = status.setups[bucket]?.armed ?? false
            let sub = NSMenu()
            appendSetupBody(to: sub, bucket: bucket, indent: false)
            let parent = NSMenuItem(title: "\(name) · \(armed ? "armed" : "off")",
                                    action: nil, keyEquivalent: "")
            parent.submenu = sub
            othersMenu.addItem(parent)
        }
        othersParent.submenu = othersMenu
        menu.addItem(othersParent)
        menu.addItem(.separator())

        // Manual switch — ignores gates/triggers.
        let switchParent = NSMenuItem(title: "Switch now", action: nil, keyEquivalent: "")
        let switchMenu = NSMenu()
        for ch in 1...3 {
            let item = NSMenuItem(title: "Channel \(ch)", action: #selector(switchNow(_:)), keyEquivalent: "")
            item.target = self
            item.representedObject = ch
            switchMenu.addItem(item)
        }
        switchParent.submenu = switchMenu
        menu.addItem(switchParent)
    }

    // Renders one setup: two trigger-slot submenus. The setup is armed
    // automatically whenever at least one slot is set — no separate toggle.
    func appendSetupBody(to menu: NSMenu, bucket: String, indent: Bool) {
        let pad = indent ? "  " : ""
        let setup = status.setups[bucket] ?? Setup()

        for slot in 1...2 {
            let trig = setup.triggers[slot - 1]
            let sub = NSMenu()

            // Zone picker (including Off).
            let offItem = NSMenuItem(title: "Off", action: #selector(setTriggerZone(_:)), keyEquivalent: "")
            offItem.target = self
            offItem.representedObject = ["bucket": bucket, "slot": slot, "zone": "off"]
            if !trig.isSet { offItem.state = .on }
            sub.addItem(offItem)
            sub.addItem(.separator())
            for z in TRIGGERS {
                let zi = NSMenuItem(title: TRIGGER_LABELS[z] ?? z, action: #selector(setTriggerZone(_:)), keyEquivalent: "")
                zi.target = self
                zi.representedObject = ["bucket": bucket, "slot": slot, "zone": z]
                if trig.zone == z { zi.state = .on }
                sub.addItem(zi)
            }
            sub.addItem(.separator())
            // Channel picker.
            let chHeader = NSMenuItem(title: "Switch to channel:", action: nil, keyEquivalent: "")
            chHeader.isEnabled = false
            sub.addItem(chHeader)
            for ch in 1...3 {
                let ci = NSMenuItem(title: "Channel \(ch)", action: #selector(setTriggerChannel(_:)), keyEquivalent: "")
                ci.target = self
                ci.representedObject = ["bucket": bucket, "slot": slot, "channel": ch]
                if trig.channel == ch { ci.state = .on }
                sub.addItem(ci)
            }

            let desc: String
            if trig.isSet {
                desc = "\(TRIGGER_LABELS[trig.zone] ?? trig.zone) → Ch \(trig.channel)"
            } else {
                desc = "Off"
            }
            let parent = NSMenuItem(title: "\(pad)Trigger \(slot):  \(desc)", action: nil, keyEquivalent: "")
            parent.submenu = sub
            menu.addItem(parent)
        }
    }

    @objc func toggleEnabled() {
        _ = sendCommand("TOGGLE")
        refresh()
    }

    @objc func setTriggerZone(_ sender: NSMenuItem) {
        guard let info = sender.representedObject as? [String: Any],
              let bucket = info["bucket"] as? String,
              let slot = info["slot"] as? Int,
              let zone = info["zone"] as? String else { return }
        _ = sendCommand("SET trigger \(bucket) \(slot) \(zone)")
        refresh()
    }

    @objc func setTriggerChannel(_ sender: NSMenuItem) {
        guard let info = sender.representedObject as? [String: Any],
              let bucket = info["bucket"] as? String,
              let slot = info["slot"] as? Int,
              let ch = info["channel"] as? Int else { return }
        _ = sendCommand("SET channel \(bucket) \(slot) \(ch)")
        refresh()
    }

    @objc func switchNow(_ sender: NSMenuItem) {
        guard let ch = sender.representedObject as? Int else { return }
        _ = sendCommand("SWITCH \(ch)")
    }
}

let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let delegate = AppDelegate()
app.delegate = delegate
app.run()
