# LogiGate

**Professional Logitech Hardware Synchronizer for macOS**

LogiGate is a surgical, high-performance CLI utility designed to synchronize multiple Logitech "Easy-Switch" devices (such as the MX Master 3S and ERGO K860) with sub-millisecond latency. It provides a native alternative to Logitech Flow that works across air-gapped machines, VPNs, and corporate firewalls.

**Note:** Currently tested on macOS only. The core hardware protocol should be portable to other operating systems with minimal modifications.

## Core Features

- **Precision Protocol:** Uses empirically validated HID++ 2.0 payloads for guaranteed physical switching.
- **Surgical Path Targeting:** Bypasses macOS exclusive driver locks by targeting specific hardware nodes (`DevSrvsID`).
- **Zero-Password Execution:** Optimized for background automation via a targeted `sudoers` whitelist.
- **Single Binary Utility:** All logic and hardware engines are bundled into a unified Go binary.

---

## 🚀 Installation

Clone the project and build it:

```bash
# Clone the repository
git clone https://github.com/ashumeet/logi-gate
cd logi-gate

# Build, sign, and install to /usr/local/bin
# Requires Go to be installed
make
```

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

### LogiOptions+ Button Integration
For seamless hardware button triggering:

1. **Create Automator Service:**
   - Open Automator → New Document → Quick Action
   - Set "Workflow receives" to "no input" in "all applications"
   - Add "Run Shell Script" action with: `/usr/local/bin/logi-gate 1`
   - Save as "Logi Gate" (or similar name)

2. **Assign Keyboard Shortcut:**
   - System Settings → Keyboard → Shortcuts → Services
   - Find your "Logi Gate" service
   - Assign a unique shortcut (e.g., Ctrl+Alt+Cmd+L)

3. **Program LogiOptions+ Button:**
   - Open LogiOptions+
   - Select your device (MX Master 3S, etc.)
   - Find the desired button (e.g., "Calculator" button next to 1,2,3)
   - Assign it to trigger your keyboard shortcut

4. **Repeat on All Machines:**
   - Set up the same configuration on each computer
   - Now pressing the hardware button instantly switches all devices to that machine

**Note:** The first time you run the service, macOS will prompt for **Input Monitoring** permission. Click "Allow" when prompted. If you accidentally dismiss the dialog, manually add `/usr/local/bin/logi-gate` in **System Settings > Privacy & Security > Input Monitoring**.

---

## 📖 How it Works

LogiGate utilizes an embedded `hidapitester` engine to speak the **Logitech HID++ 2.0 Protocol**. Unlike standard software, LogiGate bypasses the primary mouse/keyboard interfaces (which are locked by macOS) and communicates directly with the **Easy-Switch Control Node** (`usagePage: 0xFF43`) using surgical path targeting.

By sending a validated `0x11, 0x01, [Idx], 0x1E` payload, it forces the hardware to perform a physical handover to the specified host channel.

## 🤝 Contributing

LogiGate is open-source. Hardware protocol mappings and feature indices are documented in `HARDWARE_PROTOCOL.md`.
