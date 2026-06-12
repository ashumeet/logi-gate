# LogiGate Makefile
#   make            → build & install everything (switcher + daemon + menubar)
#   make reinstall  → migrate from old system-daemon install, then install fresh
#   make nuke       → remove everything (interactive uninstall script)
#   make reload     → restart daemon + menubar (after rebuild)
#   make clean      → remove local build artifacts

BIN_PATH=/usr/local/bin/logi-gate
ENGINE_PATH=/usr/local/bin/logigate-engine
DAEMON_BIN=/usr/local/bin/logi-gated
BAR_BIN=/usr/local/bin/logi-gate-bar

# Stable code-signing identity. Auto-detected from the local keychain so this
# works on any machine; if none exists, `make` mints a self-signed one (see the
# signing-cert target). Override explicitly with:  make SIGN_ID="Your Identity"
# (name or SHA-1 from: security find-identity -v -p codesigning)
#
# REQUIRED: ad-hoc signing (codesign -s -) gives TCC only a cdhash-based
# Designated Requirement with no certificate anchor, so tccd cannot re-attribute
# the binary after a reboot and DROPS the Accessibility + Input Monitoring grants
# every boot. A cert-anchored identity yields a DR that survives reboot AND
# rebuild, so the grant persists permanently after a single re-approval.
SIGN_ID ?= $(shell security find-identity -v -p codesigning 2>/dev/null | grep -oE '[0-9A-F]{40}' | head -1)

DAEMON_PLIST=$(HOME)/Library/LaunchAgents/com.logigate.daemon.plist
BAR_PLIST=$(HOME)/Library/LaunchAgents/com.logigate.bar.plist

LEGACY_DAEMON_PLIST=/Library/LaunchDaemons/com.logigate.daemon.plist

.PHONY: all build install reinstall nuke clean reload signing-cert \
        migrate-legacy load-daemon unload-daemon load-bar unload-bar

all: install

# Ensure a stable code-signing identity exists before building. Falling back to
# ad-hoc silently would reintroduce the every-reboot permission reset. Uses an
# existing Apple Development / Developer ID cert if present; otherwise mints a
# stable self-signed one (100-year, never expires). First mint is interactive
# (login password once) — see scripts/make-signing-cert.sh.
signing-cert:
	@if [ -n "$(SIGN_ID)" ]; then \
		echo "→ Using code-signing identity: $(SIGN_ID)"; \
	else \
		echo "→ No code-signing identity found — creating a self-signed one..."; \
		./scripts/make-signing-cert.sh "LogiGate Local Signing"; \
	fi

build: signing-cert
	@echo "→ Building switcher CLI..."
	go build -o logi-gate ./cmd/logi-gate
	@echo "→ Building daemon (Cgo CGEventTap)..."
	CGO_ENABLED=1 go build -o logi-gated ./cmd/logi-gated
	@echo "→ Building menubar app (Swift)..."
	swiftc -O -o logi-gate-bar menubar/LogiGateBar/main.swift -framework AppKit
	cp cmd/logi-gate/bin/hidapitester ./logigate-engine
	@echo "→ Signing with stable identity (cert-anchored DR survives reboot)..."
	@id="$(SIGN_ID)"; \
	 [ -n "$$id" ] || id=$$(security find-identity -v -p codesigning 2>/dev/null | grep -oE '[0-9A-F]{40}' | head -1); \
	 if [ -z "$$id" ]; then \
	   echo "✗ No code-signing identity available. Run: make signing-cert"; \
	   exit 1; \
	 fi; \
	 echo "  identity: $$id"; \
	 codesign --force -s "$$id" -i com.logigate.cli    logi-gate; \
	 codesign --force -s "$$id" -i com.logigate.daemon logi-gated; \
	 codesign --force -s "$$id" -i com.logigate.bar    logi-gate-bar; \
	 codesign --force -s "$$id" -i com.logigate.engine ./logigate-engine

# Migrate off the old system LaunchDaemon (pre-4.x install) if present.
# The daemon used to run as root under /Library/LaunchDaemons; that context
# cannot see the user's displays via CoreGraphics and broke after every reboot.
migrate-legacy:
	@if [ -f $(LEGACY_DAEMON_PLIST) ] || sudo launchctl print system/com.logigate.daemon >/dev/null 2>&1; then \
		echo "→ Migrating: booting out legacy system daemon..."; \
		-sudo launchctl bootout system/com.logigate.daemon 2>/dev/null; \
		-sudo launchctl unload $(LEGACY_DAEMON_PLIST) 2>/dev/null; \
		sudo rm -f $(LEGACY_DAEMON_PLIST); \
		-sudo rm -f /var/run/logigate.sock; \
		-sudo rm -f /var/log/logi-gated.log; \
		-sudo rm -rf "/Library/Application Support/LogiGate"; \
		echo "→ Legacy system daemon removed."; \
	fi

install: build migrate-legacy
	@echo "→ Installing binaries..."
	sudo cp logi-gate $(BIN_PATH)            && sudo chmod +x $(BIN_PATH)
	sudo cp ./logigate-engine $(ENGINE_PATH) && sudo chmod +x $(ENGINE_PATH)
	sudo cp logi-gated $(DAEMON_BIN)         && sudo chmod +x $(DAEMON_BIN)
	sudo cp logi-gate-bar $(BAR_BIN)         && sudo chmod +x $(BAR_BIN)
	rm -f ./logigate-engine
	@echo "→ Sudoers rule (passwordless HID access)..."
	@echo "$(shell whoami) ALL=(ALL) NOPASSWD: $(ENGINE_PATH)" | sudo tee /etc/sudoers.d/logigate >/dev/null
	sudo chmod 0440 /etc/sudoers.d/logigate
	@echo "→ User config dir..."
	mkdir -p "$(HOME)/Library/Application Support/LogiGate"
	@echo "→ LaunchAgent plists..."
	mkdir -p $(HOME)/Library/LaunchAgents
	cp launchd/com.logigate.daemon.plist $(DAEMON_PLIST)
	cp launchd/com.logigate.bar.plist    $(BAR_PLIST)
	@$(MAKE) -s load-daemon load-bar
	@echo ""
	@echo "✓ Installed."
	@echo ""
	@echo "ONE-TIME SETUP (required once after switching to stable signing):"
	@echo "  System Settings → Privacy & Security"
	@echo "    Accessibility    → REMOVE any old logi-gated entry (−), then add"
	@echo "                       /usr/local/bin/logi-gated (+) → ON"
	@echo "    Input Monitoring → REMOVE any old logi-gated entry (−), then add"
	@echo "                       /usr/local/bin/logi-gated (+) → ON"
	@echo "  The OLD entry was keyed to the ad-hoc cdhash and will never match the"
	@echo "  newly-signed binary, so it MUST be removed once. After this single"
	@echo "  re-grant the permission persists across every reboot — permanently."
	@echo ""
	@echo "Then: make reload   (or just reboot to confirm it sticks)"

# Idempotent re-install: safe to run any time. Handles migration off the
# legacy root system daemon and replaces the running user agent in place.
reinstall: install
	@echo "✓ Reinstalled."

nuke:
	@./scripts/uninstall.sh

reload: load-daemon load-bar
	@echo "✓ Reloaded."

clean:
	rm -f logi-gate logi-gated logi-gate-bar logigate-engine

# -------- launchd helpers --------
# Both daemon and bar run as user LaunchAgents under gui/$UID.
# kickstart -k restarts an already-loaded service in place; load -w first-loads it.

load-daemon:
	@if launchctl print gui/$$UID/com.logigate.daemon >/dev/null 2>&1; then \
		echo "→ Restarting daemon (kickstart)..."; \
		launchctl kickstart -k gui/$$UID/com.logigate.daemon; \
	else \
		echo "→ Loading daemon (first time)..."; \
		launchctl load -w $(DAEMON_PLIST); \
	fi

unload-daemon:
	-launchctl bootout gui/$$UID/com.logigate.daemon 2>/dev/null || launchctl unload $(DAEMON_PLIST) 2>/dev/null || true

load-bar:
	@if launchctl print gui/$$UID/com.logigate.bar >/dev/null 2>&1; then \
		echo "→ Restarting bar (kickstart)..."; \
		launchctl kickstart -k gui/$$UID/com.logigate.bar; \
	else \
		echo "→ Loading bar (first time)..."; \
		launchctl load -w $(BAR_PLIST); \
	fi

unload-bar:
	-launchctl bootout gui/$$UID/com.logigate.bar 2>/dev/null || launchctl unload $(BAR_PLIST) 2>/dev/null || true
