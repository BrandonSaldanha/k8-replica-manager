SHELL := /bin/bash

BINARY_NAME := server
BIN_DIR := bin
CMD_DIR := ./cmd/server

.PHONY: build test run clean kind-up kind-down setup

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)

test:
	go test ./...

run: build
	./$(BIN_DIR)/$(BINARY_NAME)

clean:
	rm -rf $(BIN_DIR)

kind-up:
	kind create cluster --name replica-manager

kind-down:
	kind delete cluster --name replica-manager

setup: kind-up
	@echo "setup complete (future: generate TLS certs, load images, deploy helm)"
