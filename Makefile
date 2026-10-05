BIN     := pironman
PREFIX  := /usr/local/bin
SERVICE := pironman

.DEFAULT_GOAL := help
.PHONY: help build test vet install update clean

help: ## List targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build for this machine
	go build -o $(BIN) ./cmd/pironman

test: ## Run all tests
	go test ./...

vet: ## Run go vet
	go vet ./...

install: build ## First-time setup on the Pi: install, enable and start the service
	sudo install -m 0755 $(BIN) $(PREFIX)/$(BIN)
	sudo $(PREFIX)/$(BIN) daemon install
	sudo $(PREFIX)/$(BIN) daemon enable
	sudo $(PREFIX)/$(BIN) daemon start

update: build ## Update on the Pi: replace the binary and restart the service
	sudo install -m 0755 $(BIN) $(PREFIX)/$(BIN)
	sudo systemctl restart $(SERVICE)
	$(PREFIX)/$(BIN) version

clean: ## Remove the built binary
	rm -f $(BIN)
