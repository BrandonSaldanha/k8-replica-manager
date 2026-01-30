SHELL := /bin/bash

BINARY_NAME := server
BIN_DIR := bin
CMD_DIR := ./cmd/server

# Defaults (override like: make CLUSTER_NAME=foo deploy)
CLUSTER_NAME ?= replica-manager
NAMESPACE    ?= replica-manager
RELEASE      ?= replica-manager
CHART_DIR    ?= ./charts/k8-replica-manager

IMAGE_REPO   ?= k8-replica-manager
IMAGE_TAG    ?= latest

.PHONY: build test run clean \
	kind-up kind-down kind-load \
	docker-build \
	certs \
	deploy undeploy \
	setup \
	integration-test

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)

test:
	go test ./...

run: build
	./$(BIN_DIR)/$(BINARY_NAME)

clean:
	rm -rf $(BIN_DIR)

# --- KIND ---

kind-up:
	@kind get clusters | grep -qx "$(CLUSTER_NAME)" || kind create cluster --name "$(CLUSTER_NAME)"
	@kubectl config use-context "kind-$(CLUSTER_NAME)" >/dev/null

kind-down:
	kind delete cluster --name "$(CLUSTER_NAME)"

# --- Docker image for Kubernetes deployment ---

docker-build:
	docker build -t "$(IMAGE_REPO):$(IMAGE_TAG)" .

kind-load: docker-build
	kind load docker-image "$(IMAGE_REPO):$(IMAGE_TAG)" --name "$(CLUSTER_NAME)"

# --- Local cert generation (for takehome/dev) ---

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

# --- Helm deploy ---

deploy: kind-up kind-load
	@test -f certs/ca.crt -a -f certs/server.crt -a -f certs/server.key || (echo "missing certs; run: make certs" && exit 1)
	helm upgrade --install "$(RELEASE)" "$(CHART_DIR)" \
	  --namespace "$(NAMESPACE)" \
	  --create-namespace \
	  --set image.repository="$(IMAGE_REPO)" \
	  --set image.tag="$(IMAGE_TAG)" \
	  --set image.pullPolicy=IfNotPresent \
	  --set tls.enabled=true \
	  --set tls.createSecret=true \
	  --set-file tls.serverCrt=certs/server.crt \
	  --set-file tls.serverKey=certs/server.key \
	  --set-file tls.clientCA=certs/ca.crt

undeploy:
	-helm uninstall "$(RELEASE)" -n "$(NAMESPACE)"
	-kubectl delete namespace "$(NAMESPACE)"

# Fresh-clone happy path: bring up cluster, build/load image, generate certs, deploy chart
setup: kind-up kind-load certs deploy
	@echo "setup complete: kind cluster=$(CLUSTER_NAME), namespace=$(NAMESPACE), release=$(RELEASE)"

integration-test:
	./scripts/integration_test.sh
