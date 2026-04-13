#!/bin/bash
# LogiGate — full uninstall.
# Removes daemon, menubar, switcher CLI, engine, plists, sudoers rule, config, logs.
# Requires sudo.

set -u

echo "LogiGate uninstall"
echo "=================="
echo "This will remove ALL LogiGate components from the system."
read -p "Continue? [y/N] " confirm
case "$confirm" in
    y|Y|yes|YES) ;;
    *) echo "Aborted."; exit 0 ;;
esac

echo ""
echo "→ Stopping daemons..."
launchctl unload -w "$HOME/Library/LaunchAgents/com.logigate.bar.plist" 2>/dev/null || true
sudo launchctl unload -w /Library/LaunchDaemons/com.logigate.daemon.plist 2>/dev/null || true

echo "→ Killing any stragglers..."
sudo pkill -9 logi-gated 2>/dev/null || true
pkill -9 logi-gate-bar 2>/dev/null || true
sudo pkill -9 logigate-engine 2>/dev/null || true

echo "→ Removing launchd plists..."
sudo rm -f /Library/LaunchDaemons/com.logigate.daemon.plist
rm -f "$HOME/Library/LaunchAgents/com.logigate.bar.plist"

echo "→ Removing binaries..."
sudo rm -f /usr/local/bin/logi-gated
sudo rm -f /usr/local/bin/logi-gate-bar
sudo rm -f /usr/local/bin/logi-gate
sudo rm -f /usr/local/bin/logigate-engine

echo "→ Removing sudoers rule..."
sudo rm -f /etc/sudoers.d/logigate

echo "→ Removing config + socket + log..."
sudo rm -rf "/Library/Application Support/LogiGate"
sudo rm -f /var/run/logigate.sock
sudo rm -f /var/log/logi-gated.log

echo "→ Removing legacy/leftover artifacts..."
sudo rm -f /usr/local/bin/logigate-trigger.sh
sudo rm -f /usr/local/bin/switch_logi
rm -f "$HOME/Library/LaunchAgents/com.ashu.logiswitch.plist"
rm -rf /tmp/logigate.* /tmp/logi_switch_signal /tmp/logigate-engine 2>/dev/null || true

echo ""
echo "✓ Uninstalled."
echo ""
echo "MANUAL STEP (macOS won't let scripts touch TCC):"
echo "  System Settings → Privacy & Security → Input Monitoring + Accessibility"
echo "  Remove 'logi-gated' (and any other LogiGate entries) from both lists."
