# LogiGate Makefile
#   make         → build & install everything (switcher + daemon + menubar)
#   make nuke    → remove everything (interactive uninstall script)
#   make reload  → restart daemon + menubar (after rebuild)
#   make clean   → remove local build artifacts

BIN_PATH=/usr/local/bin/logi-gate
ENGINE_PATH=/usr/local/bin/logigate-engine
DAEMON_BIN=/usr/local/bin/logi-gated
BAR_BIN=/usr/local/bin/logi-gate-bar

DAEMON_PLIST=/Library/LaunchDaemons/com.logigate.daemon.plist
BAR_PLIST=$(HOME)/Library/LaunchAgents/com.logigate.bar.plist

.PHONY: all build install nuke clean reload \
        load-daemon unload-daemon load-bar unload-bar

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

install: build
	@echo "→ Installing binaries..."
	sudo cp logi-gate $(BIN_PATH)            && sudo chmod +x $(BIN_PATH)
	sudo cp ./logigate-engine $(ENGINE_PATH) && sudo chmod +x $(ENGINE_PATH)
	sudo cp logi-gated $(DAEMON_BIN)         && sudo chmod +x $(DAEMON_BIN)
	sudo cp logi-gate-bar $(BAR_BIN)         && sudo chmod +x $(BAR_BIN)
	rm -f ./logigate-engine
	@echo "→ Sudoers rule (passwordless HID access)..."
	@echo "$(shell whoami) ALL=(ALL) NOPASSWD: $(ENGINE_PATH)" | sudo tee /etc/sudoers.d/logigate >/dev/null
	sudo chmod 0440 /etc/sudoers.d/logigate
	@echo "→ Config dir..."
	sudo mkdir -p "/Library/Application Support/LogiGate"
	sudo chmod 0775 "/Library/Application Support/LogiGate"
	@echo "→ Launchd plists..."
	sudo cp launchd/com.logigate.daemon.plist $(DAEMON_PLIST)
	sudo chown root:wheel $(DAEMON_PLIST) && sudo chmod 0644 $(DAEMON_PLIST)
	mkdir -p $(HOME)/Library/LaunchAgents
	cp launchd/com.logigate.bar.plist $(BAR_PLIST)
	@$(MAKE) -s load-daemon load-bar
	@echo ""
	@echo "✓ Installed."
	@echo ""
	@echo "ONE-TIME SETUP: System Settings → Privacy & Security"
	@echo "  Input Monitoring → add /usr/local/bin/logi-gated → ON"
	@echo "  Accessibility    → add /usr/local/bin/logi-gated → ON"
	@echo "Then: make reload"

nuke:
	@./scripts/uninstall.sh

reload: load-daemon load-bar
	@echo "✓ Reloaded."

clean:
	rm -f logi-gate logi-gated logi-gate-bar logigate-engine

# -------- launchd helpers --------
# kickstart -k restarts an already-loaded service in place; load -w first-loads it.
# We prefer kickstart so TCC grants survive across rebuilds without races.

load-daemon:
	@if sudo launchctl print system/com.logigate.daemon >/dev/null 2>&1; then \
		echo "→ Restarting daemon (kickstart)..."; \
		sudo launchctl kickstart -k system/com.logigate.daemon; \
	else \
		echo "→ Loading daemon (first time)..."; \
		sudo launchctl load -w $(DAEMON_PLIST); \
	fi

unload-daemon:
	-sudo launchctl bootout system/com.logigate.daemon 2>/dev/null || sudo launchctl unload $(DAEMON_PLIST) 2>/dev/null || true

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
