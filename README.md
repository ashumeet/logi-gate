# LogiGate

**App-agnostic Logitech Easy-Switch orchestration for macOS.**

LogiGate switches all your paired Logitech devices (MX Master, ERGO K860, etc.)
to the same host channel in a single operation, using the validated HID++ 2.0
protocol. It works three ways:

1. **Cursor-corner trigger** — move the cursor to a screen corner or edge and
   all paired devices switch channels automatically (daemon + menubar).
2. **CLI one-shot** — `logi-gate 2` to force a switch.
3. **Automation** — bind the CLI to a hotkey via Automator + LogiOptions+.

It works across air-gapped machines, VPNs, and corporate firewalls — no cloud,
no Logitech Flow, no shared network required.

---

## Components

| Binary | Role | Runs as |
|---|---|---|
| `logi-gate` | CLI switcher (`logi-gate 1\|2\|3\|scan`). Embeds the HID engine. | user (on demand) |
| `logi-gated` | Daemon. CGEventTap watches cursor; fires a switch when the cursor dwells in the configured trigger zone. | user LaunchAgent |
| `logi-gate-bar` | Menubar app. Toggle on/off, pick trigger zone + target channel, manual switch. | user LaunchAgent |
| `logigate-engine` | Statically installed `hidapitester`. Requires root for HID access; invoked via a passwordless sudoers rule. | root (via `sudo -n`) |

---

## Architecture notes

- **Both `logi-gated` and `logi-gate-bar` run as user LaunchAgents under
  `gui/$UID/`.** The daemon needs to be in a GUI session so
  `CGGetActiveDisplayList` and `CGDisplayRegisterReconfigurationCallback` work
  against WindowServer. A root system daemon under `/Library/LaunchDaemons/`
  cannot see displays and its event tap breaks across reboots.
- **Display gating:** triggers only fire when exactly **one external display**
  is connected. With zero externals (laptop only) or multiple externals the
  menubar icon greys out and the daemon drops cursor events.
- **Socket IPC:** daemon exposes a UNIX socket at `/tmp/logigate-$UID.sock`
  (commands: `STATUS`, `TOGGLE`, `SET trigger|channel`, `SWITCH <n>`, `SCAN`).
- **Config:** `~/Library/Application Support/LogiGate/config.json` — enabled,
  dwell_ms, cooldown_ms, trigger, channel.
- **Log:** `/tmp/logi-gated.log` (and `/tmp/logi-gate-bar.log`).

---

## Install

Requires Go (for `logi-gate` and `logi-gated`) and Xcode command-line tools
(for `swiftc`, used to build the menubar).

```bash
git clone https://github.com/ashumeet/logi-gate
cd logi-gate
make
```

`make` builds, signs, and installs everything:

- Binaries → `/usr/local/bin/` (requires sudo once)
- Sudoers rule → `/etc/sudoers.d/logigate` (NOPASSWD for the engine)
- Plists → `~/Library/LaunchAgents/com.logigate.daemon.plist` and
  `~/Library/LaunchAgents/com.logigate.bar.plist`
- Loads both agents under `gui/$UID/` immediately

### One-time TCC grants

After the first install, grant the daemon two macOS privileges so the event
tap can see cursor movement:

**System Settings → Privacy & Security:**

- **Input Monitoring** → add `/usr/local/bin/logi-gated` → toggle **ON**
- **Accessibility** → add `/usr/local/bin/logi-gated` → toggle **ON**

The daemon polls these grants every 2 seconds and self-exits the moment
Input Monitoring flips on; launchd respawns it with a live tap. No manual
reload needed.

### Reinstall (or upgrading from the old root-daemon install)

```bash
make reinstall
```

`reinstall` runs `migrate-legacy` first, which:

- Boots out any running `system/com.logigate.daemon` (the old root daemon)
- Removes `/Library/LaunchDaemons/com.logigate.daemon.plist`
- Cleans up `/var/run/logigate.sock`, `/var/log/logi-gated.log`, and
  `/Library/Application Support/LogiGate`

then installs the current user-agent layout.

If you previously granted TCC to `logi-gated` as a root daemon, **remove and
re-add it** in Input Monitoring + Accessibility after `make reinstall` — the
user-agent principal is a different TCC grantee.

### Uninstall

```bash
make nuke
```

Removes every binary, plist, sudoers rule, config, socket, and log from both
the current and legacy install layouts. TCC entries must be cleared manually
in System Settings.

### Rebuild after code changes

```bash
make reload
```

Kickstarts both agents in place (preserves TCC grants).

---

## Usage

### Menubar (primary UX)

Left-click the menubar icon to toggle the trigger on/off. Right-click for:

- **Trigger** submenu — pick the activation zone (Bottom-Left, Bottom-Right,
  Left Edge, Right Edge).
- **Channel** submenu — pick which channel the trigger switches to (1, 2, 3).
- **Switch to Channel N** — manual one-click switch, ignores trigger state.

Icon states:

- **Blue `display`** — enabled and display-qualified (1 external connected).
- **Grey `display`** — disabled by user toggle.
- **Faded `display.slash`** — gated out because display count != 1 external.

### CLI

```bash
logi-gate scan        # list discovered Logitech devices + feature indices
logi-gate 1           # force-switch all devices to channel 1
logi-gate 2           # channel 2
logi-gate 3           # channel 3
```

### Automation (hotkey + LogiOptions+)

For one-keypress switching using a Logitech button:

1. **Automator Quick Action** — New Document → Quick Action, "receives no
   input" in "all applications", add a Run Shell Script action with
   `/usr/local/bin/logi-gate 1`. Save as e.g. "LogiGate 1".
2. **Keyboard shortcut** — System Settings → Keyboard → Keyboard Shortcuts →
   Services → find "LogiGate 1" → assign a shortcut (e.g. Ctrl+Alt+Cmd+1).
3. **LogiOptions+** — open Logi Options+, pick your device, bind a hardware
   button to that keyboard shortcut.
4. Repeat on every machine. Now one hardware button switches every paired
   Logitech device across every host.

---

## Troubleshooting

**Icon is greyed out / triggers don't fire.**
Check `echo STATUS | nc -U /tmp/logigate-$UID.sock`. If `qualified=false`, you
don't have exactly 1 external display attached — that's intentional gating.

**Daemon installed but cursor corners do nothing.**
Tail `/tmp/logi-gated.log`. If you see
`Input Monitoring at startup: granted=false`, grant TCC (see above). If the
log stops at "event tap: first event received" but triggers don't fire,
check `active_trigger` in STATUS matches the corner you're hitting.

**`launchctl print gui/$UID/com.logigate.daemon` says "Could not find service"**.
The agent isn't loaded. Run `make reload` or `launchctl load -w
~/Library/LaunchAgents/com.logigate.daemon.plist`.

**Old install from before the LaunchAgent migration.**
Run `make reinstall` — it handles the cleanup automatically.

---

## How it works

LogiGate embeds `hidapitester` and speaks the **Logitech HID++ 2.0 Protocol**
directly to the **Easy-Switch Control Node** (`usagePage: 0xFF43`), bypassing
the primary mouse/keyboard interfaces that macOS holds exclusive locks on. It
targets a specific hardware path (`DevSrvsID`) rather than the VID/PID pair,
which is what makes it work while LogiOptions+ is also running.

The validated payload is `0x11 0x01 [FeatureIdx] 0x1E [Channel] 00 ... (20B)`.
See `HARDWARE_PROTOCOL.md` for the byte-level spec and per-device feature
indices.

---

## Contributing

Hardware protocol mappings and feature indices are documented in
`HARDWARE_PROTOCOL.md`. Feature-index probes for unknown Logitech devices
use the `probeFeatureIndex` path in `cmd/logi-gate/manager.go`; hardcoded
fast-path IDs (B034, B364) live in the same file.
