SHELL := /bin/bash

BINARY_NAME := server
BIN_DIR := bin
CMD_DIR := ./cmd/server

.PHONY: build test run clean kind-up kind-down setup certs integration-test

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

integration-test:
	./scripts/integration_tls.sh

certs:
	mkdir -p certs
	openssl genrsa -out certs/ca.key 4096
	openssl req -x509 -new -nodes -key certs/ca.key -sha256 -days 3650 \
	  -subj "/CN=replica-manager-ca" -out certs/ca.crt

	openssl genrsa -out certs/server.key 2048
	openssl req -new -key certs/server.key -subj "/CN=localhost" -out certs/server.csr
	printf "subjectAltName=DNS:localhost,IP:127.0.0.1\n" > certs/server.ext
	openssl x509 -req -in certs/server.csr -CA certs/ca.crt -CAkey certs/ca.key -CAcreateserial \
	  -out certs/server.crt -days 365 -sha256 -extfile certs/server.ext

	openssl genrsa -out certs/client.key 2048
	openssl req -new -key certs/client.key -subj "/CN=replica-manager-client" -out certs/client.csr
	openssl x509 -req -in certs/client.csr -CA certs/ca.crt -CAkey certs/ca.key -CAcreateserial \
	  -out certs/client.crt -days 365 -sha256