BIN     := pironman
PREFIX  := /usr/local/bin
SERVICE := pironman
HOST    ?=

.DEFAULT_GOAL := help
.PHONY: help build build-pi test vet install update deploy clean require-host

help: ## List targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build for this machine
	go build -o $(BIN) ./cmd/pironman

build-pi: ## Cross-compile for the Pi (linux/arm64)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o $(BIN) ./cmd/pironman

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

deploy: require-host build-pi ## Update a Pi from another machine: make deploy HOST=<ssh-host>
	scp $(BIN) $(HOST):/tmp/$(BIN)
	ssh -t $(HOST) 'sudo install -m 0755 /tmp/$(BIN) $(PREFIX)/$(BIN) && rm /tmp/$(BIN) && sudo systemctl restart $(SERVICE) && $(PREFIX)/$(BIN) version'

clean: ## Remove the built binary
	rm -f $(BIN)

require-host:
	@test -n "$(HOST)" || { echo "deploy: set HOST, e.g. make deploy HOST=pi" >&2; exit 1; }
