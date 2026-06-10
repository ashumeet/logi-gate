#!/usr/bin/env bash
# One-time migration from ad-hoc signing to a stable cert-anchored identity.
#
# WHY: Ad-hoc signed binaries (codesign -s -) give TCC a Designated Requirement
# that is only a cdhash with no certificate anchor. On reboot, tccd restarts and
# cannot re-attribute the binary to its stored grant, so the Accessibility and
# Input Monitoring permissions are dropped EVERY boot. Signing with a stable
# certificate produces a cert-anchored DR that survives reboot AND rebuild, so
# the grant only ever has to be approved once.
#
# Run this AFTER `make` has rebuilt + reinstalled the cert-signed binaries.
# It fully reloads both user agents so the freshly-signed daemon is the live
# client when you re-approve the permission. Removing the stale System Settings
# entry is manual (tccutil cannot target a path-based, non-bundle client without
# wiping the service for every app on the machine).
set -euo pipefail

UID_NUM="$(id -u)"
DAEMON=com.logigate.daemon
BAR=com.logigate.bar
DAEMON_BIN=/usr/local/bin/logi-gated
DAEMON_PLIST="$HOME/Library/LaunchAgents/$DAEMON.plist"
BAR_PLIST="$HOME/Library/LaunchAgents/$BAR.plist"

echo "→ Verifying the installed daemon is cert-signed (not ad-hoc)..."
if codesign -dvv "$DAEMON_BIN" 2>&1 | grep -q 'Signature=adhoc'; then
  echo "✗ $DAEMON_BIN is still ad-hoc signed. Run 'make' first, then re-run this."
  exit 1
fi
echo "  Designated Requirement now:"
codesign -d -r- "$DAEMON_BIN" 2>&1 | sed 's/^/    /'
echo "  (Must be 'identifier ... and certificate leaf ...', NOT 'cdhash ...'.)"

echo "→ Booting out user agents so they relaunch against the new signature..."
launchctl bootout "gui/$UID_NUM/$DAEMON" 2>/dev/null || true
launchctl bootout "gui/$UID_NUM/$BAR"    2>/dev/null || true

echo "→ Re-bootstrapping user agents..."
launchctl bootstrap "gui/$UID_NUM" "$DAEMON_PLIST"
launchctl bootstrap "gui/$UID_NUM" "$BAR_PLIST"
launchctl kickstart -k "gui/$UID_NUM/$DAEMON" 2>/dev/null || true
launchctl kickstart -k "gui/$UID_NUM/$BAR"    2>/dev/null || true

cat <<EOF

✓ Agents reloaded with the cert-signed daemon.

FINAL ONE-TIME STEP — do this once, then never again:

  System Settings → Privacy & Security
    Accessibility:
      1. If 'logi-gated' is listed, select it and click − to REMOVE it
         (that entry is the stale ad-hoc grant and will never match the new binary).
      2. Click +, press Cmd+Shift+G, enter: $DAEMON_BIN , add it, toggle ON.
    Input Monitoring:
      Repeat the same remove-then-add for $DAEMON_BIN , toggle ON.

  Then: launchctl kickstart -k gui/$UID_NUM/$DAEMON

Reboot once to confirm. After this, the permission persists across every reboot.
EOF
