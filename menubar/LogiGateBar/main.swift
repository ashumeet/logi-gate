import AppKit
import Foundation

let SOCKET_PATH = "/var/run/logigate.sock"
let TRIGGERS = ["bottom_left", "bottom_right", "left_edge", "right_edge"]
let TRIGGER_LABELS: [String: String] = [
    "bottom_left": "Bottom-Left Corner",
    "bottom_right": "Bottom-Right Corner",
    "left_edge": "Left Edge",
    "right_edge": "Right Edge",
]

struct Status {
    var enabled: Bool = false
    var qualified: Bool = false
    var trigger: String = "bottom_left"
    var channel: Int = 1
    var active: Bool { enabled && qualified }
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
    s.qualified = (obj["qualified"] as? Bool) ?? false
    if let t = obj["trigger"] as? String { s.trigger = t }
    if let c = obj["channel"] as? Int { s.channel = c }
    return s
}

class AppDelegate: NSObject, NSApplicationDelegate {
    var statusItem: NSStatusItem!
    var status = Status()

    func applicationDidFinishLaunching(_ n: Notification) {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        if let btn = statusItem.button {
            btn.target = self
            btn.action = #selector(onClick(_:))
            btn.sendAction(on: [.leftMouseUp, .rightMouseUp])
        }
        refresh()
        Timer.scheduledTimer(withTimeInterval: 3.0, repeats: true) { _ in self.refresh() }
    }

    func refresh() {
        status = fetchStatus()
        updateIcon()
    }

    func updateIcon() {
        guard let btn = statusItem.button else { return }
        let on = status.active
        let symbol = NSImage(systemSymbolName: "display",
                             accessibilityDescription: "LogiGate")
        symbol?.isTemplate = false
        btn.image = symbol
        btn.contentTintColor = on ? NSColor.systemBlue : NSColor.systemGray
        let tip: String
        if !status.qualified {
            tip = "LogiGate — off (needs exactly 1 external display)"
        } else if status.enabled {
            tip = "LogiGate — on"
        } else {
            tip = "LogiGate — off"
        }
        btn.toolTip = tip
    }

    @objc func onClick(_ sender: NSStatusBarButton) {
        guard let event = NSApp.currentEvent else { return }
        if event.type == .rightMouseUp {
            showMenu()
        } else {
            // Left click: toggle user preference. Auto-gate still applies.
            _ = sendCommand("TOGGLE")
            refresh()
        }
    }

    func showMenu() {
        let menu = NSMenu()

        let triggerSub = NSMenu()
        for t in TRIGGERS {
            let label = TRIGGER_LABELS[t] ?? t
            let item = NSMenuItem(title: label,
                                  action: #selector(setActiveTrigger(_:)),
                                  keyEquivalent: "")
            item.target = self
            item.representedObject = t
            if t == status.trigger { item.state = .on }
            triggerSub.addItem(item)
        }
        let triggerParent = NSMenuItem(title: "Trigger", action: nil, keyEquivalent: "")
        triggerParent.submenu = triggerSub
        menu.addItem(triggerParent)

        let channelSub = NSMenu()
        for ch in 1...3 {
            let item = NSMenuItem(title: "Channel \(ch)",
                                  action: #selector(setActiveChannel(_:)),
                                  keyEquivalent: "")
            item.target = self
            item.representedObject = ch
            if ch == status.channel { item.state = .on }
            channelSub.addItem(item)
        }
        let channelParent = NSMenuItem(title: "Channel", action: nil, keyEquivalent: "")
        channelParent.submenu = channelSub
        menu.addItem(channelParent)

        menu.addItem(.separator())
        for ch in 1...3 {
            let item = NSMenuItem(title: "Switch to Channel \(ch)",
                                  action: #selector(switchNow(_:)),
                                  keyEquivalent: "\(ch)")
            item.target = self
            item.representedObject = ch
            menu.addItem(item)
        }

        statusItem.menu = menu
        statusItem.button?.performClick(nil)
        statusItem.menu = nil
    }

    @objc func setActiveTrigger(_ sender: NSMenuItem) {
        guard let t = sender.representedObject as? String else { return }
        _ = sendCommand("SET trigger \(t)")
        refresh()
    }

    @objc func setActiveChannel(_ sender: NSMenuItem) {
        guard let ch = sender.representedObject as? Int else { return }
        _ = sendCommand("SET channel \(ch)")
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
