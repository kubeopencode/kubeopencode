# Copyright Contributors to the KubeOpenCode project
SHELL := /bin/bash

all: build
.PHONY: all

# Version information
VERSION ?= 0.0.25
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Image URL to use for building/pushing image targets
IMG_REGISTRY ?= ghcr.io
IMG_ORG ?= kubeopencode
IMG_NAME ?= kubeopencode
IMG ?= $(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME):$(VERSION)

# PLATFORMS defines the target platforms for multi-arch build
PLATFORMS ?= linux/arm64,linux/amd64

# Go packages
GO_PACKAGES := $(addsuffix ...,$(addprefix ./,$(filter-out vendor/ test/ hack/ client/,$(wildcard */))))
GO_BUILD_PACKAGES := $(GO_PACKAGES)
GO_BUILD_PACKAGES_EXPANDED := $(GO_BUILD_PACKAGES)
GO_LD_FLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildDate=$(BUILD_DATE)

# Local bin directory for tools
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# controller-gen setup
CONTROLLER_GEN_VERSION := v0.16.5
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen

# golangci-lint (version auto-detected by ci/lint/run-lint.sh based on Go version)
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

# Ensure GOPATH is set
check-env:
ifeq ($(GOPATH),)
	$(warning "environment variable GOPATH is empty, auto set from go env GOPATH")
export GOPATH=$(shell go env GOPATH)
endif
.PHONY: check-env

# Download controller-gen locally if not present
.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen if not present
$(CONTROLLER_GEN): $(LOCALBIN)
	@test -s $(CONTROLLER_GEN) || \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

# golangci-lint is installed automatically by ci/lint/run-lint.sh

# Vendor dependencies
vendor:
	go mod tidy
	go mod vendor
.PHONY: vendor

# Update scripts
update-scripts:
	hack/update-deepcopy.sh
	hack/update-codegen.sh
.PHONY: update-scripts

# Update all generated code
update: check-env vendor update-scripts update-crds
.PHONY: update

# Generate CRDs
update-crds: controller-gen
	$(CONTROLLER_GEN) crd:crdVersions=v1 \
		paths="./api/v1alpha1" \
		output:crd:dir=./deploy/crds
	@echo "Copying CRDs to Helm chart..."
	@mkdir -p ./charts/kubeopencode/crds
	@cp -f ./deploy/crds/*.yaml ./charts/kubeopencode/crds/
	@echo "CRDs updated successfully in both locations"
.PHONY: update-crds

# Build unified kubeopencode binary
build:
	go build -ldflags '$(GO_LD_FLAGS)' -o bin/kubeopencode ./cmd/kubeopencode
.PHONY: build

# Build KubeOpenCode CLI binary
build-cli:
	go build -ldflags '$(GO_LD_FLAGS)' -o bin/kubeoc ./cmd/kubeoc
.PHONY: build-cli

# Test runs unit tests only.
# Integration tests are excluded via build tags (//go:build integration).
# This follows the Kubernetes ecosystem convention (kubebuilder, controller-runtime)
# where tests remain alongside code but are separated by build tags.
# See: internal/controller/suite_test.go for detailed explanation.
test:
	go test -v ./internal/...
.PHONY: test

# Integration test runs envtest-based controller tests.
# Requires -tags=integration to include files with //go:build integration.
# envtest provides a local API server and etcd for testing without a full cluster.
# K8s version and setup-envtest version are auto-detected from go.mod.
integration-test:
	KUBEBUILDER_ASSETS="$$(LOCALBIN=$(LOCALBIN) ./ci/envtest/ensure-envtest.sh)" \
		go test -v -tags=integration ./internal/controller/... -coverprofile cover.out
.PHONY: integration-test

# Clean
clean:
	rm -rf bin/
	rm -rf vendor/
.PHONY: clean

# Verify
verify: check-env
	bash -x hack/verify-deepcopy.sh
	bash -x hack/verify-codegen.sh
.PHONY: verify

##@ Docker

# Build the docker image (includes UI build)
# Always tags as both :VERSION and :latest so Kind/local-dev clusters work without extra steps
docker-build: ui-build
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_DATE) \
		-t $(IMG) \
		-t $(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME):latest \
		.
.PHONY: docker-build

# Push the docker image
docker-push:
	docker push $(IMG)
.PHONY: docker-push

# Build and push docker image for multiple architectures (includes UI build)
docker-buildx: ui-build
	docker buildx create --use --name=kubeopencode-builder || true
	docker buildx build \
		--platform=$(PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_DATE) \
		--tag $(IMG) \
		--push \
		.
.PHONY: docker-buildx

##@ Helm

# Package helm chart
helm-package:
	helm package charts/kubeopencode -d dist/
.PHONY: helm-package

# Install helm chart
helm-install:
	helm install kubeopencode charts/kubeopencode \
		--namespace kubeopencode-system \
		--create-namespace \
		--set controller.image.repository=$(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME) \
		--set controller.image.tag=$(VERSION)
.PHONY: helm-install

# Upgrade helm chart
helm-upgrade:
	helm upgrade kubeopencode charts/kubeopencode \
		--namespace kubeopencode-system \
		--set controller.image.repository=$(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME) \
		--set controller.image.tag=$(VERSION)
.PHONY: helm-upgrade

# Uninstall helm chart
helm-uninstall:
	helm uninstall kubeopencode --namespace kubeopencode-system
.PHONY: helm-uninstall

# Template helm chart (dry-run)
helm-template:
	helm template kubeopencode charts/kubeopencode \
		--namespace kubeopencode-system \
		--set controller.image.repository=$(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME) \
		--set controller.image.tag=$(VERSION)
.PHONY: helm-template

# Chart registry settings
CHART_REGISTRY ?= oci://$(IMG_REGISTRY)/$(IMG_ORG)/helm-charts
CHART_NAME ?= kubeopencode

# Login to helm registry
helm-login: ## Login to helm OCI registry
	helm registry login $(IMG_REGISTRY)
.PHONY: helm-login

# Push helm chart to OCI registry
helm-push: helm-package ## Push helm chart to OCI registry
	helm push dist/$(CHART_NAME)-*.tgz $(CHART_REGISTRY)
.PHONY: helm-push

##@ UI

# Check if npm/yarn is available
UI_PACKAGE_MANAGER := $(shell command -v pnpm 2> /dev/null || command -v yarn 2> /dev/null || command -v npm 2> /dev/null)

# Build React UI
ui-install: ## Install UI dependencies
	cd ui && $(UI_PACKAGE_MANAGER) install
.PHONY: ui-install

ui-build: ## Build React UI for production
	@echo "Building React UI..."
	@rm -rf ui/dist
	cd ui && $(UI_PACKAGE_MANAGER) run build
	@echo "UI build complete"
.PHONY: ui-build

ui-test: ## Run UI unit tests
	cd ui && $(UI_PACKAGE_MANAGER) test
.PHONY: ui-test

ui-test-coverage: ## Run UI tests with coverage report
	cd ui && $(UI_PACKAGE_MANAGER) run test:coverage
.PHONY: ui-test-coverage

ui-dev: ## Run UI development server
	cd ui && $(UI_PACKAGE_MANAGER) run dev
.PHONY: ui-dev

ui-dev-mock: ## Run UI dev server with mock API (no backend needed)
	cd ui && $(UI_PACKAGE_MANAGER) run dev:mock
.PHONY: ui-dev-mock

ui-clean: ## Clean UI build artifacts
	rm -rf ui/dist
	rm -rf ui/node_modules
.PHONY: ui-clean

##@ Development

# Run controller locally
run:
	go run ./cmd/kubeopencode controller
.PHONY: run

# Run server locally
run-server: ## Run UI server locally
	go run ./cmd/kubeopencode server
.PHONY: run-server

# Run webhook server locally
run-webhook:
	go run ./cmd/kubeopencode webhook
.PHONY: run-webhook

# Format code
fmt:
	go fmt ./...
.PHONY: fmt

# Lint code (auto-detects and installs compatible golangci-lint version)
lint:
	@LOCALBIN=$(LOCALBIN) ./ci/lint/run-lint.sh
.PHONY: lint

##@ E2E Testing

# Kind cluster name for e2e testing
E2E_CLUSTER_NAME ?= kubeopencode-e2e
E2E_IMG_TAG ?= dev

# Create kind cluster for e2e testing
# Uses e2e/kind-config.yaml to expose NodePort 30082 for webhook server
e2e-kind-create: ## Create kind cluster for e2e testing
	@if kind get clusters | grep -q "^$(E2E_CLUSTER_NAME)$$"; then \
		echo "Kind cluster '$(E2E_CLUSTER_NAME)' already exists"; \
	else \
		kind create cluster --name $(E2E_CLUSTER_NAME) --config e2e/kind-config.yaml; \
	fi
.PHONY: e2e-kind-create

# Delete kind cluster
e2e-kind-delete: ## Delete kind cluster
	kind delete cluster --name $(E2E_CLUSTER_NAME)
.PHONY: e2e-kind-delete

# Build docker image for e2e testing
e2e-docker-build: ## Build docker image for e2e testing
	docker build --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg BUILD_TIME=$(BUILD_DATE) -t $(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME):$(E2E_IMG_TAG) .
.PHONY: e2e-docker-build

# Load docker image into kind cluster
# Also tags and loads :latest for init containers (git-init, save-session) that use DefaultKubeOpenCodeImage
e2e-kind-load: ## Load docker image into kind cluster
	kind load docker-image $(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME):$(E2E_IMG_TAG) --name $(E2E_CLUSTER_NAME)
	@if [ "$(E2E_IMG_TAG)" != "latest" ]; then \
		docker tag $(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME):$(E2E_IMG_TAG) $(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME):latest; \
		kind load docker-image $(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME):latest --name $(E2E_CLUSTER_NAME); \
	fi
.PHONY: e2e-kind-load

# Verify image in kind cluster
e2e-verify-image: ## Verify image is loaded in kind cluster
	@echo "Verifying image in kind cluster..."
	@docker exec -i $(E2E_CLUSTER_NAME)-control-plane crictl images | grep $(IMG_NAME) || \
		(echo "Error: Image not found in kind cluster" && exit 1)
	@echo "Image verified successfully"
.PHONY: e2e-verify-image

# Deploy controller to kind cluster using Helm (CRDs are in crds/ directory)
# Using uninstall + install instead of upgrade to ensure CRDs are properly installed
# Webhook is exposed as NodePort for E2E testing
e2e-deploy: ## Deploy controller and CRDs to kind cluster using Helm
	@helm uninstall kubeopencode --namespace kubeopencode-system 2>/dev/null || true
	helm install kubeopencode charts/kubeopencode \
		--namespace kubeopencode-system \
		--create-namespace \
		--set controller.image.repository=$(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME) \
		--set controller.image.tag=$(E2E_IMG_TAG) \
		--set controller.image.pullPolicy=Never \
		--set server.image.repository=$(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME) \
		--set server.image.tag=$(E2E_IMG_TAG) \
		--set server.image.pullPolicy=Never \
		--set webhook.enabled=true \
		--set webhook.service.type=NodePort \
		--set webhook.service.nodePort=30082 \
		--wait
.PHONY: e2e-deploy

# Undeploy controller from kind cluster (CRDs will be removed by Helm)
e2e-undeploy: ## Undeploy controller and CRDs from kind cluster
	helm uninstall kubeopencode --namespace kubeopencode-system || true
	kubectl delete namespace kubeopencode-system --ignore-not-found=true
.PHONY: e2e-undeploy

# Setup e2e environment (create cluster, build images, load images, and deploy)
e2e-setup: e2e-kind-create e2e-docker-build e2e-agent-build e2e-kind-load e2e-agent-load e2e-verify-image e2e-deploy ## Setup complete e2e environment
	@echo "E2E environment setup complete"
.PHONY: e2e-setup

# Teardown e2e environment (undeploy controller and delete cluster)
e2e-teardown: e2e-undeploy e2e-kind-delete ## Teardown e2e environment
	@echo "E2E environment teardown complete"
.PHONY: e2e-teardown

# Rebuild and reload controller image (for iterative development)
e2e-reload: e2e-docker-build e2e-kind-load e2e-verify-image ## Rebuild and reload controller image
	@echo "Restarting controller pods..."
	@kubectl rollout restart deployment -n kubeopencode-system || true
	@echo "Controller image reloaded successfully"
.PHONY: e2e-reload

# Build agent images for e2e testing
# Uses E2E_IMG_TAG (default: dev) to avoid :latest which triggers PullAlways in Kind
e2e-agent-build: ## Build agent images for e2e testing (echo + opencode)
	docker build -t ghcr.io/kubeopencode/kubeopencode-agent-echo:$(E2E_IMG_TAG) agents/echo/
	$(MAKE) -C agents AGENT=opencode build IMG=ghcr.io/kubeopencode/kubeopencode-agent-opencode:$(E2E_IMG_TAG)
.PHONY: e2e-agent-build

# Load agent images into kind cluster
e2e-agent-load: ## Load agent images into kind cluster (echo + opencode)
	kind load docker-image ghcr.io/kubeopencode/kubeopencode-agent-echo:$(E2E_IMG_TAG) --name $(E2E_CLUSTER_NAME)
	kind load docker-image ghcr.io/kubeopencode/kubeopencode-agent-opencode:$(E2E_IMG_TAG) --name $(E2E_CLUSTER_NAME)
.PHONY: e2e-agent-load


# Run e2e tests
e2e-test: ## Run e2e tests
	@echo "Running e2e tests..."
	E2E_TEST_NAMESPACE=kubeopencode-e2e-test \
	E2E_ECHO_IMAGE=ghcr.io/kubeopencode/kubeopencode-agent-echo:$(E2E_IMG_TAG) \
	E2E_AGENT_IMAGE=ghcr.io/kubeopencode/kubeopencode-agent-opencode:$(E2E_IMG_TAG) \
	go test -v ./e2e/... -timeout 30m -ginkgo.v -ginkgo.flake-attempts=2
.PHONY: e2e-test

# Run specific e2e test by focus string
e2e-test-focus: ## Run specific e2e test (usage: make e2e-test-focus FOCUS="Task")
	@echo "Running focused e2e tests..."
	E2E_TEST_NAMESPACE=kubeopencode-e2e-test \
	E2E_ECHO_IMAGE=ghcr.io/kubeopencode/kubeopencode-agent-echo:$(E2E_IMG_TAG) \
	E2E_AGENT_IMAGE=ghcr.io/kubeopencode/kubeopencode-agent-opencode:$(E2E_IMG_TAG) \
	go test -v ./e2e/... -timeout 30m -ginkgo.v -ginkgo.focus="$(FOCUS)" -ginkgo.flake-attempts=2
.PHONY: e2e-test-focus

# Run e2e tests by label (recommended)
# Available labels: task, agent, server, opencode
# Examples:
#   make e2e-test-label LABEL="server"
#   make e2e-test-label LABEL="server || agent"
#   make e2e-test-label LABEL="!opencode"
e2e-test-label: ## Run e2e tests by label (usage: make e2e-test-label LABEL="server")
	@echo "Running e2e tests with label: $(LABEL)..."
	E2E_TEST_NAMESPACE=kubeopencode-e2e-test \
	E2E_ECHO_IMAGE=ghcr.io/kubeopencode/kubeopencode-agent-echo:$(E2E_IMG_TAG) \
	E2E_AGENT_IMAGE=ghcr.io/kubeopencode/kubeopencode-agent-opencode:$(E2E_IMG_TAG) \
	go test -v ./e2e/... -timeout 30m -ginkgo.v -ginkgo.label-filter="$(LABEL)" -ginkgo.flake-attempts=2
.PHONY: e2e-test-label

# Run OpenCode e2e tests only (uses free models, no API key needed)
e2e-test-opencode: ## Run OpenCode integration tests with free models
	@echo "Running OpenCode e2e tests (free model: opencode/big-pickle)..."
	E2E_TEST_NAMESPACE=kubeopencode-e2e-test \
	E2E_ECHO_IMAGE=ghcr.io/kubeopencode/kubeopencode-agent-echo:$(E2E_IMG_TAG) \
	E2E_AGENT_IMAGE=ghcr.io/kubeopencode/kubeopencode-agent-opencode:$(E2E_IMG_TAG) \
	go test -v ./e2e/... -timeout 30m -ginkgo.v -ginkgo.label-filter="opencode" -ginkgo.flake-attempts=2
.PHONY: e2e-test-opencode

# Full e2e test workflow (setup, test, teardown)
e2e: e2e-setup e2e-test ## Run full e2e test workflow
	@echo "E2E tests complete"
.PHONY: e2e

##@ Local Development

# Kind cluster name for local development
LOCAL_DEV_CLUSTER ?= kubeopencode
LOCAL_DEV_IMG_TAG ?= dev
LOCAL_DEV_AGENTS := opencode devbox attach

# All images needed for local dev (controller + agents)
LOCAL_DEV_CONTROLLER_IMG := $(IMG_REGISTRY)/$(IMG_ORG)/$(IMG_NAME)
LOCAL_DEV_AGENT_IMGS := $(foreach a,$(LOCAL_DEV_AGENTS),$(IMG_REGISTRY)/$(IMG_ORG)/kubeopencode-agent-$(a))

local-dev-setup: ## One-command local dev setup: cluster, images, helm, test resources
	@echo "=== KubeOpenCode Local Dev Setup ==="
	@# Step 1: Create Kind cluster (idempotent)
	@if kind get clusters 2>/dev/null | grep -q "^$(LOCAL_DEV_CLUSTER)$$"; then \
		echo "[1/5] Kind cluster '$(LOCAL_DEV_CLUSTER)' already exists, reusing"; \
	else \
		echo "[1/5] Creating Kind cluster '$(LOCAL_DEV_CLUSTER)'..."; \
		kind create cluster --name $(LOCAL_DEV_CLUSTER); \
	fi
	@# Step 2: Build images (controller + agents)
	@echo "[2/5] Building images..."
	@$(MAKE) docker-build
	@for agent in $(LOCAL_DEV_AGENTS); do \
		echo "  Building agent: $$agent"; \
		$(MAKE) agent-build AGENT=$$agent; \
	done
	@# Step 3: Load images into Kind
	@echo "[3/5] Loading images into Kind..."
	@kind load docker-image $(LOCAL_DEV_CONTROLLER_IMG):latest --name $(LOCAL_DEV_CLUSTER)
	@for img in $(LOCAL_DEV_AGENT_IMGS); do \
		docker tag $$img:$(VERSION) $$img:$(LOCAL_DEV_IMG_TAG) 2>/dev/null || true; \
		kind load docker-image $$img:$(LOCAL_DEV_IMG_TAG) --name $(LOCAL_DEV_CLUSTER); \
	done
	@# Step 4: Deploy with Helm
	@echo "[4/5] Deploying KubeOpenCode with Helm..."
	@helm upgrade --install kubeopencode ./charts/kubeopencode \
		--namespace kubeopencode-system \
		--create-namespace \
		--set controller.image.pullPolicy=Never \
		--set controller.image.tag=latest \
		--set agent.image.pullPolicy=Never \
		--set server.enabled=true \
		--set server.image.tag=latest \
		--wait --timeout 120s
	@# Step 5: Deploy local-dev test resources
	@echo "[5/5] Deploying test resources..."
	@kubectl apply -k deploy/local-dev/
	@echo ""
	@echo "=== Setup Complete ==="
	@echo "Controller: kubectl get pods -n kubeopencode-system"
	@echo "Agents:     kubectl get agent -n test"
	@echo "Template:   kubectl get agenttemplate -n test"
	@echo "UI:         kubectl port-forward -n kubeopencode-system svc/kubeopencode-server 2746:2746"
.PHONY: local-dev-setup

local-dev-reload: ## Rebuild and reload all images into local dev cluster
	@echo "=== Rebuilding and reloading images ==="
	@$(MAKE) docker-build
	@for agent in $(LOCAL_DEV_AGENTS); do \
		$(MAKE) agent-build AGENT=$$agent; \
	done
	@kind load docker-image $(LOCAL_DEV_CONTROLLER_IMG):latest --name $(LOCAL_DEV_CLUSTER)
	@for img in $(LOCAL_DEV_AGENT_IMGS); do \
		docker tag $$img:$(VERSION) $$img:$(LOCAL_DEV_IMG_TAG) 2>/dev/null || true; \
		kind load docker-image $$img:$(LOCAL_DEV_IMG_TAG) --name $(LOCAL_DEV_CLUSTER); \
	done
	@kubectl rollout restart deployment -n kubeopencode-system
	@kubectl rollout status deployment/kubeopencode-controller -n kubeopencode-system --timeout=90s
	@echo "Reload complete"
.PHONY: local-dev-reload

local-dev-teardown: ## Delete Kind cluster and all local dev resources
	@echo "=== Tearing down local dev environment ==="
	@kubectl delete -k deploy/local-dev/ --ignore-not-found=true 2>/dev/null || true
	@helm uninstall kubeopencode -n kubeopencode-system 2>/dev/null || true
	@kind delete cluster --name $(LOCAL_DEV_CLUSTER) 2>/dev/null || true
	@echo "Teardown complete"
.PHONY: local-dev-teardown

##@ Agent

agent-base-build: ## Build universal base image
	$(MAKE) -C agents base-build

agent-base-push: ## Push universal base image
	$(MAKE) -C agents base-push

agent-base-buildx: ## Multi-arch build and push base image
	$(MAKE) -C agents base-buildx

agent-build: ## Build agent image (requires base image)
	$(MAKE) -C agents build

agent-push: ## Push agent image
	$(MAKE) -C agents push

agent-buildx: ## Multi-arch build and push agent image
	$(MAKE) -C agents buildx

agent-build-all: ## Build base and all agent images
	$(MAKE) -C agents build-all

agent-push-all: ## Push base and all agent images
	$(MAKE) -C agents push-all

agent-buildx-all: ## Multi-arch build and push all images
	$(MAKE) -C agents buildx-all

##@ Website

website-install: ## Install website dependencies
	cd website && pnpm install
.PHONY: website-install

website-dev: ## Start website dev server
	cd website && pnpm start
.PHONY: website-dev

website-build: ## Build website for production
	cd website && pnpm build
.PHONY: website-build

##@ Help

# Display this help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
.PHONY: help
