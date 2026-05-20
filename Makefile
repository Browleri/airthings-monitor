APP := airthings-server
BIN_DIR := bin
CONFIG ?= config.example.toml
GO ?= go

CACHE_ROOT ?= /mnt/pihole-usb/airthings/cache
GOCACHE ?= $(CACHE_ROOT)/go-build
GOMODCACHE ?= $(CACHE_ROOT)/go-mod
GOFLAGS ?= -p 1
GOMAXPROCS ?= 2

GOENV := GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOMAXPROCS=$(GOMAXPROCS)

.PHONY: test build web-build run lint clean prepare-cache

prepare-cache:
	mkdir -p $(GOCACHE) $(GOMODCACHE)

test: prepare-cache
	$(GOENV) $(GO) test $(GOFLAGS) ./...

build: web-build
	mkdir -p $(BIN_DIR)
	$(GOENV) $(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(APP) ./cmd/airthings-server

web-build:
	cd web && npm install && npm run build

run: prepare-cache
	$(GOENV) $(GO) run $(GOFLAGS) ./cmd/airthings-server -config $(CONFIG)

lint: prepare-cache
	$(GOENV) $(GO) vet $(GOFLAGS) ./...
	cd web && npm install && npm run typecheck

clean:
	rm -rf $(BIN_DIR) web/dist