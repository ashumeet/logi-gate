# LogiGate Manual CLI Makefile

BIN_PATH=/usr/local/bin/logi-gate
ENGINE_PATH=/usr/local/bin/logigate-engine

.PHONY: all build install uninstall nuke clean

all: build install

build:
	@echo "Building Go Binary..."
	go build -o logi-gate ./cmd/logi-gate

install: build
	@echo "Installing binaries..."
	@echo "Ad-hoc signing locally..."
	codesign -s - --force logi-gate
	cp cmd/logi-gate/bin/hidapitester ./logigate-engine
	codesign -s - --force ./logigate-engine
	@echo "Moving to system paths..."
	sudo cp logi-gate $(BIN_PATH)
	sudo chmod +x $(BIN_PATH)
	sudo cp ./logigate-engine $(ENGINE_PATH)
	sudo chmod +x $(ENGINE_PATH)
	rm ./logigate-engine
	@echo "Whitelisting for passwordless hardware access..."
	@echo "$(shell whoami) ALL=(ALL) NOPASSWD: $(ENGINE_PATH)" | sudo tee /etc/sudoers.d/logigate
	sudo chmod 0440 /etc/sudoers.d/logigate
	@echo "SUCCESS: logi-gate is ready for manual terminal commands."

uninstall:
	@echo "Removing binaries and sudoers rule..."
	sudo rm -f $(BIN_PATH) $(ENGINE_PATH)
	sudo rm -f /etc/sudoers.d/logigate

nuke: uninstall
	@echo "Purging Launch Agent files..."
	rm -f ~/Library/LaunchAgents/com.logigate.*.plist
	-sudo pkill -9 logi-gate || true
	-sudo pkill -9 logigate-engine || true

clean:
	rm -f logi-gate
