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

DAEMON_PLIST=$(HOME)/Library/LaunchAgents/com.logigate.daemon.plist
BAR_PLIST=$(HOME)/Library/LaunchAgents/com.logigate.bar.plist

LEGACY_DAEMON_PLIST=/Library/LaunchDaemons/com.logigate.daemon.plist

.PHONY: all build install reinstall nuke clean reload \
        migrate-legacy load-daemon unload-daemon load-bar unload-bar

all: install

build:
	@echo "→ Building switcher CLI..."
	go build -o logi-gate ./cmd/logi-gate
	@echo "→ Building daemon (Cgo CGEventTap)..."
	CGO_ENABLED=1 go build -o logi-gated ./cmd/logi-gated
	@echo "→ Building menubar app (Swift)..."
	swiftc -O -o logi-gate-bar menubar/LogiGateBar/main.swift -framework AppKit
	@echo "→ Signing..."
	codesign -s - --force logi-gate
	codesign -s - --force logi-gated
	codesign -s - --force logi-gate-bar
	cp cmd/logi-gate/bin/hidapitester ./logigate-engine
	codesign -s - --force ./logigate-engine

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
	@echo "ONE-TIME SETUP (required after first install OR after any migration):"
	@echo "  System Settings → Privacy & Security"
	@echo "    Input Monitoring → add /usr/local/bin/logi-gated → ON"
	@echo "    Accessibility    → add /usr/local/bin/logi-gated → ON"
	@echo "  (If 'logi-gated' is already in those lists from an older install,"
	@echo "   remove the existing entry first, then re-add — the launch context"
	@echo "   changed from root daemon to user agent.)"
	@echo ""
	@echo "Then: make reload"

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
