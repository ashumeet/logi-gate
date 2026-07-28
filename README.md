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
- **Setup gating:** the active setup is chosen by external-display count
  (0/1/2+ buckets); a setup fires only if it has a trigger. See Usage below.
- **Device gating:** if no switchable Logitech devices are connected there's
  nothing to switch, so the icon greys out and the daemon drops cursor events.
  Presence is driven by IOKit HID add/remove callbacks (push, **no polling**).
- **Feature-index cache:** the per-device ChangeHost index (needed to switch)
  is resolved once via HID probe and cached by PID in `index-cache.json`. A warm
  switch skips the ~470ms-per-device probe, dropping switch latency from ~2s to
  ~100ms. The probe only runs on a cache miss (a device model never seen).
- **Socket IPC:** daemon exposes a UNIX socket at `/tmp/logigate-$UID.sock`
  (commands: `STATUS`, `TOGGLE`,
  `SET trigger <bucket> <1|2> <zone|off>`, `SET channel <bucket> <1|2> <1|2|3>`,
  `SWITCH <n>`, `SCAN`, where `bucket` ∈ `none|one|multi`). A setup is armed iff
  it has a trigger; STATUS reports the derived `armed` per bucket.
- **Config:** `~/Library/Application Support/LogiGate/config.json` — enabled,
  dwell_ms, cooldown_ms, and `setups` (a per-bucket map, each with two
  `triggers`). Legacy `trigger`/`channel` configs migrate automatically.
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

LogiGate is organized around **Setups** — display configurations it
auto-detects by external-monitor count. There are three, and every real
arrangement lands in exactly one:

- **Laptop only** (0 external)
- **1 external display**
- **2+ external displays**

Each Setup has up to **two triggers** (a corner/edge zone → a target channel).
A Setup is **armed automatically whenever it has at least one trigger set** —
there is no separate on/off per setup. So a bare laptop with no triggers stays
inert, while the same laptop docked with a monitor (a setup you've given a
trigger) arms itself, with no manual switching. The **Enable** button is the
single master override on top of that.

Click the menubar icon for the menu (Layout A — the active setup expands inline):

- **Enabled/Disabled** — master toggle (turns everything off regardless of triggers).
- **Now: N external → …** — the currently-active setup and whether it's armed.
- **THIS SETUP** — the active setup, expanded: *Trigger 1* and *Trigger 2*
  submenus. Each trigger has a zone (four corners + four edges) and a target
  channel, chosen **independently in any order**; the trigger goes live once
  both are set. *Off* toggles a configured trigger on/off **without discarding**
  its zone/channel, so you can disable and re-enable without re-picking.
- **Other setups ▸** — the other two setups, each in its own submenu with the
  same controls, so you can pre-configure them.
- **Switch now ▸** — manual one-click switch to a channel, ignores triggers.

Since you're always *already* on one of your three hosts, one or two triggers
per setup is enough to reach the other two. On multi-monitor setups the
corner/edge is detected on whichever display the cursor is currently in.

Icon states:

- **Blue `display`** — enabled and the active setup is armed.
- **Grey `display`** — disabled by the master toggle.
- **Faded `display.slash`** — the active setup isn't armed.

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

Hardware protocol mappings are documented in `HARDWARE_PROTOCOL.md`. Every
Logitech device's ChangeHost feature index is resolved dynamically by
`probeFeatureIndex` in `cmd/logi-gate/manager.go` — there is no hardcoded
PID→index table. The probe queries HID++ feature `0x1814`, validates the reply
header (`11 FF 00 00`) to skip the stale frames the device emits right after
open, and retries with backoff. This works for any Logitech unit, so multiple
keyboard/mouse sets across machines are handled without code changes.
