# LogiGate

**Universal Logitech Hardware Synchronizer for macOS**

LogiGate is a professional, high-performance utility designed to synchronize multiple Logitech "Easy-Switch" devices (such as the MX Master 3S and ERGO K860) with a single command or hotkey. It provides a native, low-latency alternative to Logitech Flow that works across air-gapped machines, VPNs, and corporate firewalls.

## Core Features

- **Dynamic Discovery:** Automatically identifies all connected Logitech HID++ 2.0/3.0 devices and their internal feature indices.
- **Zero-Dependency Delivery:** All logic and hardware engines are bundled into a single, unified Go binary.
- **Native Hotkey Daemon:** Hook directly into the macOS event loop for instant, background synchronization.
- **Surgical Execution:** Communicates directly with device firmware for sub-millisecond switching performance.

---

## 🚀 Installation

Follow these steps to build and install LogiGate manually on your Mac.

### 1. Build from Source
Ensure you have [Go](https://go.dev/) installed, then run:
```bash
# Build the unified binary
go build -o logi-gate cmd/logi-gate/*.go

# Move to your system path
sudo cp logi-gate /usr/local/bin/
sudo chmod +x /usr/local/bin/logi-gate
```

### 2. Configure Hardware Permissions
LogiGate requires root access to communicate with HID hardware. To enable instant switching without password prompts, add a whitelist rule to your system:
```bash
# Grant password-free access to the utility
echo "$(whoami) ALL=(ALL) NOPASSWD: /usr/local/bin/logi-gate" | sudo tee /etc/sudoers.d/logigate
sudo chmod 0440 /etc/sudoers.d/logigate
```

---

## ⌨️ Usage

### Global Hotkeys
The LogiGate daemon listens for the following native shortcuts (Control + Option + Command):

- **Switch all to Mac 1:** `Ctrl + Opt + Cmd + J`
- **Switch all to Mac 2:** `Ctrl + Opt + Cmd + K`
- **Switch all to Mac 3:** `Ctrl + Opt + Cmd + L`

**First-Time Setup:** Run `logi-gate daemon` in your terminal. On the first hotkey press, macOS will prompt you for **Accessibility Permissions**. Grant the permission in *System Settings* to enable the listener.

### CLI Commands
- `logi-gate scan`: Lists all discovered, sync-ready Logitech devices.
- `logi-gate switch [1-3]`: Manually force all devices to a specific channel.
- `logi-gate daemon`: Starts the background hotkey listener.

---

## 📖 How it Works

LogiGate utilizes an embedded `hidapitester` engine to speak the **Logitech HID++ Protocol**. Unlike official software that relies on network discovery, LogiGate probes the local HID stack for Vendor ID `0x046D` and identifies the **Easy-Switch** feature (`0x1E00`) on each device firmware. 

By mapping a single button on your mouse to a keyboard shortcut in **Logi Options+**, LogiGate catches the intent and sends a surgical "Follow Me" command to all other devices simultaneously.

## 🤝 Contributing

LogiGate is open-source. Contributions, feature requests, and hardware protocol mappings are welcome!
