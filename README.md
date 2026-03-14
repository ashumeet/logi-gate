# LogiGate

**Professional Logitech Hardware Synchronizer for macOS**

LogiGate is a surgical, high-performance CLI utility designed to synchronize multiple Logitech "Easy-Switch" devices (such as the MX Master 3S and ERGO K860) with sub-millisecond latency. It provides a native alternative to Logitech Flow that works across air-gapped machines, VPNs, and corporate firewalls.

## Core Features

- **Precision Protocol:** Uses empirically validated HID++ 2.0 payloads for guaranteed physical switching.
- **Surgical Path Targeting:** Bypasses macOS exclusive driver locks by targeting specific hardware nodes (`DevSrvsID`).
- **Zero-Password Execution:** Optimized for background automation via a targeted `sudoers` whitelist.
- **Single Binary Utility:** All logic and hardware engines are bundled into a unified Go binary.

---

## 🚀 Installation

Follow these steps to deploy LogiGate as a system-wide CLI utility.

### 1. Build and Install
Ensure you have [Go](https://go.dev/) installed, then use the provided Makefile to handle compilation, ad-hoc signing, and system placement:

```bash
# Build, sign, and install to /usr/local/bin
make
```

### 2. Configure Permissions
LogiGate requires **Accessibility** permission to interact with the OS and **Root Access** (handled via `sudoers`) to write to HID hardware.

1.  Open **System Settings > Privacy & Security > Accessibility**.
2.  Add `/usr/local/bin/logi-gate` manually and ensure it is toggled **ON**.

---

## ⌨️ Usage

### CLI Commands
The utility is designed for direct terminal use or integration into automation scripts.

- `logi-gate scan`: Lists all discovered, sync-ready Logitech devices and their unique hardware paths.
- `logi-gate [1|2|3]`: Instantaneously force all compatible devices to Channel 1, 2, or 3.

### Automation Trigger
To trigger LogiGate without a terminal, create a **Quick Action (Service)** in Automator that runs a shell script:
```bash
# Example Automator script for a hotkey
/usr/local/bin/logi-gate 1
```

---

## 📖 How it Works

LogiGate utilizes an embedded `hidapitester` engine to speak the **Logitech HID++ 2.0 Protocol**. Unlike standard software, LogiGate bypasses the primary mouse/keyboard interfaces (which are locked by macOS) and communicates directly with the **Easy-Switch Control Node** (`usagePage: 0xFF43`) using surgical path targeting.

By sending a validated `0x11, 0x01, [Idx], 0x1E` payload, it forces the hardware to perform a physical handover to the specified host channel.

## 🤝 Contributing

LogiGate is open-source. Hardware protocol mappings and feature indices are documented in `HARDWARE_PROTOCOL.md`.
