APP := airthings-server
BIN_DIR := bin
CONFIG ?= config.example.toml
GO ?= go
GOCACHE ?= /tmp/airthings-go-build-cache
GOMODCACHE ?= /tmp/go-mod
GOENV := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE)

.PHONY: test build web-build run lint clean

test:
	$(GOENV) $(GO) test ./...

build: web-build
	mkdir -p $(BIN_DIR)
	$(GOENV) $(GO) build -o $(BIN_DIR)/$(APP) ./cmd/airthings-server

web-build:
	cd web && npm install && npm run build

run:
	$(GOENV) $(GO) run ./cmd/airthings-server -config $(CONFIG)

lint:
	$(GOENV) $(GO) vet ./...
	cd web && npm install && npm run typecheck

clean:
	rm -rf $(BIN_DIR) web/dist
