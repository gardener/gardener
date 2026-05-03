# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

REGISTRY                                   := europe-docker.pkg.dev/gardener-project/snapshots/gardener
APISERVER_IMAGE_REPOSITORY                 := $(REGISTRY)/apiserver
CONTROLLER_MANAGER_IMAGE_REPOSITORY        := $(REGISTRY)/controller-manager
SCHEDULER_IMAGE_REPOSITORY                 := $(REGISTRY)/scheduler
ADMISSION_IMAGE_REPOSITORY                 := $(REGISTRY)/admission-controller
RESOURCE_MANAGER_IMAGE_REPOSITORY          := $(REGISTRY)/resource-manager
NODE_AGENT_IMAGE_REPOSITORY                := $(REGISTRY)/node-agent
OPERATOR_IMAGE_REPOSITORY                  := $(REGISTRY)/operator
GARDENLET_IMAGE_REPOSITORY                 := $(REGISTRY)/gardenlet
GARDENADM_IMAGE_REPOSITORY                 := $(REGISTRY)/gardenadm
EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY  := $(REGISTRY)/extensions/provider-local
EXTENSION_ADMISSION_LOCAL_IMAGE_REPOSITORY := $(REGISTRY)/extensions/admission-local
PUSH_LATEST_TAG                            := false
VERSION                                    := $(shell cat VERSION)
EFFECTIVE_VERSION                          := $(VERSION)-$(shell git rev-parse HEAD)
BUILD_DATE                                 := $(shell date '+%Y-%m-%dT%H:%M:%S%z' | sed 's/\([0-9][0-9]\)$$/:\1/g')
REPO_ROOT                                  := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
TARGET_PLATFORMS                           ?= linux/$(shell go env GOARCH)
PRINT_HELP                                 ?=

# Disable globally go workspaces until https://github.com/gardener/gardener/issues/8811 is resolved.
# This resolves issues presented with error like 'pattern ./...: directory prefix . does not contain modules listed in go.work or their selected dependencies'
export GOWORK=off

ifndef ARTIFACTS
	export ARTIFACTS=/tmp/artifacts
endif

ifneq ($(shell { git diff-index --quiet HEAD -- && ! git ls-files --others --exclude-standard 2>/dev/null | grep -q .; } 2>/dev/null || echo dirty),)
	EFFECTIVE_VERSION := $(EFFECTIVE_VERSION)-dirty
endif

SHELL=/usr/bin/env bash -o pipefail

export SYSTEM_ARCH := $(SYSTEM_ARCH)

#########################################
# Tools                                 #
#########################################

TOOLS_BIN_DIR ?= hack/tools/bin/$(go env GOOS)-$(go env GOARCH)
TOOLS_DIR := hack/tools
include hack/tools.mk

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

BUILD_OUTPUT_FILE ?= .
BUILD_PACKAGES ?= ./...

.PHONY: install
install:
	@EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) ./hack/install.sh ./...

.PHONY: build
build:
	@EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) ./hack/build.sh -o $(BUILD_OUTPUT_FILE) $(BUILD_PACKAGES)

.PHONY: docker-images
docker-images:
	@echo "Building docker images with version and tag $(EFFECTIVE_VERSION) for target platforms $(TARGET_PLATFORMS)"
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(APISERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(APISERVER_IMAGE_REPOSITORY):latest                 -f Dockerfile --target apiserver .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)        -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):latest        -f Dockerfile --target controller-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(SCHEDULER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(SCHEDULER_IMAGE_REPOSITORY):latest                 -f Dockerfile --target scheduler .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(ADMISSION_IMAGE_REPOSITORY):latest                 -f Dockerfile --target admission-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(RESOURCE_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)          -t $(RESOURCE_MANAGER_IMAGE_REPOSITORY):latest          -f Dockerfile --target resource-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(NODE_AGENT_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(NODE_AGENT_IMAGE_REPOSITORY):latest                -f Dockerfile --target node-agent .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(OPERATOR_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                  -t $(OPERATOR_IMAGE_REPOSITORY):latest                  -f Dockerfile --target operator .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(GARDENLET_IMAGE_REPOSITORY):latest                 -f Dockerfile --target gardenlet .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(GARDENADM_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(GARDENADM_IMAGE_REPOSITORY):latest                 -f Dockerfile --target gardenadm .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)  -t $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):latest  -f Dockerfile --target gardener-extension-provider-local .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) --platform $(TARGET_PLATFORMS) -t $(EXTENSION_ADMISSION_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION) -t $(EXTENSION_ADMISSION_LOCAL_IMAGE_REPOSITORY):latest -f Dockerfile --target gardener-extension-admission-local .

.PHONY: docker-push
docker-push:
	@if ! docker images $(APISERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(APISERVER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(CONTROLLER_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(CONTROLLER_MANAGER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(SCHEDULER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(SCHEDULER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(ADMISSION_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(ADMISSION_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(RESOURCE_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(RESOURCE_MANAGER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(NODE_AGENT_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(NODE_AGENT_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(GARDENLET_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(GARDENLET_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(GARDENADM_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(GARDENADM_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@docker push $(APISERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(APISERVER_IMAGE_REPOSITORY):latest; fi
	@docker push $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):latest; fi
	@docker push $(SCHEDULER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(SCHEDULER_IMAGE_REPOSITORY):latest; fi
	@docker push $(ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(ADMISSION_IMAGE_REPOSITORY):latest; fi
	@docker push $(RESOURCE_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(RESOURCE_MANAGER_IMAGE_REPOSITORY):latest; fi
	@docker push $(NODE_AGENT_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(NODE_AGENT_IMAGE_REPOSITORY):latest; fi
	@docker push $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(GARDENLET_IMAGE_REPOSITORY):latest; fi
	@docker push $(GARDENADM_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(GARDENADM_IMAGE_REPOSITORY):latest; fi
	@docker push $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):latest; fi

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

LOGCHECK_DIR := $(TOOLS_DIR)/logcheck
PKG_APIS_DIR := $(REPO_ROOT)/pkg/apis

.PHONY: tidy
tidy:
	@unset GOWORK; go work use
	@GO111MODULE=on go mod tidy
	@cd $(LOGCHECK_DIR); go mod tidy
	@cd $(PKG_APIS_DIR); go mod tidy

.PHONY: clean
clean:
	@hack/clean.sh ./charts/... ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...

.PHONY: add-license-headers
add-license-headers: $(GO_ADD_LICENSE)
	@./hack/add-license-header.sh

.PHONY: check-generate
check-generate:
	@hack/check-generate.sh $(REPO_ROOT)

.PHONY: check-plutono-dashboards
check-plutono-dashboards:
	@hack/check-plutono-dashboards.sh

.PHONY: check
check: $(GO_ADD_LICENSE) $(GOIMPORTS) $(GOLANGCI_LINT) $(HELM) $(IMPORT_BOSS) $(LOGCHECK) $(YQ) $(TYPOS) logcheck-symlinks
	@sed ./.golangci.yaml.in -e "s#<<LOGCHECK_PLUGIN_PATH>>#$(TOOLS_BIN_DIR)#g" > ./.golangci.yaml
	@hack/check.sh --golangci-lint-config=./.golangci.yaml ./charts/... ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...
	@hack/check-imports.sh ./charts/... ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...

	@echo "> Check $(PKG_APIS_DIR)"
	@cd $(PKG_APIS_DIR); ../../hack/check.sh --golangci-lint-config=../../.golangci.yaml ./...
	@cd $(PKG_APIS_DIR); ../../hack/check-imports.sh ./...

	@echo "> Check $(LOGCHECK_DIR)"
	@cd $(LOGCHECK_DIR); $(abspath $(GOLANGCI_LINT)) run -c $(REPO_ROOT)/.golangci.yaml --timeout 10m ./...
	@cd $(LOGCHECK_DIR); go vet ./...
	@cd $(LOGCHECK_DIR); $(abspath $(GOIMPORTS)) -l .

	@hack/check-charts.sh ./charts
	@hack/check-license-header.sh
	@hack/check-skaffold-deps.sh
	@hack/check-plutono-dashboards.sh
	@hack/check-typos.sh
	@hack/check-file-names.sh

.PHONY: logcheck-symlinks
logcheck-symlinks:
	@LOGCHECK_DIR=$(LOGCHECK_DIR) ./hack/generate-logcheck-symlinks.sh

tools-for-generate: $(CONTROLLER_GEN) $(EXTENSION_GEN) $(CRD_REF_DOCS) $(GOIMPORTS) $(GO_TO_PROTOBUF) $(HELM) $(MOCKGEN) $(OPENAPI_GEN) $(PROTOC) $(PROTOC_GEN_GOGO) $(YQ)
	@go mod download

define GENERATE_HELP_INFO
# Usage: make generate [WHAT="<targets>"] [MODE="<mode>"] [CODEGEN_GROUPS="<groups>"] [MANIFESTS_DIRS="<folders>"] [MAX_PARALLEL_WORKERS="<num>"]
#
# Options:
#   WHAT                   - Specify the targets to run (e.g., "protobuf codegen manifests logcheck")
#   CODEGEN_GROUPS         - Specify which groups to run the 'codegen' target for, not applicable for other targets (e.g., "authentication_groups core_groups extensions_groups resources_groups
#                            operator_groups seedmanagement_groups operations_groups operatorconfig_groups controllermanager_groups admissioncontroller_groups scheduler_groups
#                            gardenlet_groups resourcemanager_groups shoottolerationrestriction_groups shootdnsrewriting_groups shootresourcereservation_groups provider_local_groups cloud_provider_local_groups extensions_config_groups")
#   MANIFESTS_DIRS         - Specify which directories to run the 'manifests' target in, not applicable for other targets (Default directories are "charts cmd example extensions imagevector pkg plugin test")
#   MODE                   - Specify the mode for the 'manifests' (default=parallel) or 'codegen' (default=sequential) target (e.g., "parallel" or "sequential")
#   MAX_PARALLEL_WORKERS   - Specify the number of maximum parallel workers that will be used when MODE='parallel' (default=4)
#
# Examples:
#   make generate
#   make generate WHAT="codegen protobuf"
#   make generate WHAT="codegen protobuf" MODE="sequential"
#   make generate WHAT="manifests" MANIFESTS_DIRS="pkg/component plugin" MODE="sequential"
#   make generate WHAT="codegen" CODEGEN_GROUPS="core_groups extensions_groups"
#   make generate WHAT="codegen manifests" CODEGEN_GROUPS="operator_groups controllermanager_groups" MANIFESTS_DIRS="charts extensions/pkg"
#   make generate WHAT="manifests" MAX_PARALLEL_WORKERS=8
#
endef
export GENERATE_HELP_INFO
.PHONY: generate
ifeq ($(PRINT_HELP),y)
generate:
	@echo "$$GENERATE_HELP_INFO"
else
generate: tools-for-generate
	@printf "\nFor more info on the generate command, Run 'make generate PRINT_HELP=y'\n"
	@cd $(PKG_APIS_DIR); REPO_ROOT=$(REPO_ROOT) ../../hack/generate.sh --what "manifests" --manifests-dirs "./pkg/apis" --mode "$(MODE)"
	@REPO_ROOT=$(REPO_ROOT) LOGCHECK_DIR=$(LOGCHECK_DIR) hack/generate.sh --what "$(WHAT)" --codegen-groups "$(CODEGEN_GROUPS)" --manifests-dirs "$(MANIFESTS_DIRS)" --mode "$(MODE)"
	$(MAKE) format
endif

.PHONY: format
format: $(GOIMPORTS) $(GOIMPORTSREVISER)
	@MODE=$(MODE) ./hack/format.sh ./charts ./cmd ./extensions ./pkg ./plugin ./test ./hack
	@cd $(LOGCHECK_DIR); $(abspath $(GOIMPORTS)) -l -w .

.PHONY: sast
sast: $(GOSEC)
	@./hack/sast.sh

.PHONY: sast-report
sast-report: $(GOSEC)
	@./hack/sast.sh --gosec-report true

.PHONY: test
test: $(REPORT_COLLECTOR) $(PROMTOOL) $(HELM) logcheck-symlinks
	@./hack/test.sh ./charts/... ./cmd/... ./extensions/pkg/... ./pkg/... ./plugin/...
	@cd $(PKG_APIS_DIR); ../../hack/test.sh ./...
	@cd $(LOGCHECK_DIR); go test -race -timeout=2m ./... | grep -v 'no test files'

.PHONY: test-integration
test-integration: $(REPORT_COLLECTOR) $(SETUP_ENVTEST) $(HELM)
	@./hack/test-integration.sh ./test/integration/...

.PHONY: test-cov
test-cov: $(PROMTOOL) $(HELM)
	@./hack/test-cover.sh ./charts/... ./cmd/... ./extensions/pkg/... ./pkg/... ./plugin/...

.PHONY: test-cov-clean
test-cov-clean:
	@./hack/test-cover-clean.sh

.PHONY: check-apidiff
check-apidiff:
	@REPO_ROOT=$(REPO_ROOT) ./hack/check-apidiff.sh

.PHONY: verify
verify: check format test test-integration

.PHONY: verify-extended
verify-extended: check-generate check format test-cov test-cov-clean test-integration

#####################################################################
# Rules for local environment                                       #
#####################################################################

DEV_SETUP                                := $(REPO_ROOT)/dev-setup
DEV_SETUP_WITH_LPP_RESIZE_SUPPORT        ?= false
DEV_SETUP_WITH_WORKLOAD_IDENTITY_SUPPORT ?= false
IPFAMILY                                 ?= ipv4

kind-% gind-% operator-% gardener-% garden-% seed-% seed2-% ci-e2e-kind: export IPFAMILY := $(IPFAMILY)
kind-%: export WITH_LPP_RESIZE_SUPPORT := $(DEV_SETUP_WITH_LPP_RESIZE_SUPPORT)

export KUBECONFIG_RUNTIME_CLUSTER         := $(DEV_SETUP)/kubeconfigs/runtime/kubeconfig
export KUBECONFIG_VIRTUAL_GARDEN_CLUSTER  := $(DEV_SETUP)/kubeconfigs/virtual-garden/kubeconfig
export KUBECONFIG_SEED_CLUSTER            := $(DEV_SETUP)/kubeconfigs/seed/kubeconfig
export KUBECONFIG_SEED2_CLUSTER           := $(DEV_SETUP)/kubeconfigs/seed2/kubeconfig
export KUBECONFIG_SELFHOSTEDSHOOT_CLUSTER := $(DEV_SETUP)/kubeconfigs/self-hosted-shoot/kubeconfig
export KUBECONFIG_REMOTE_CLUSTER          := $(DEV_SETUP)/kubeconfigs/remote/kubeconfig

# KUBECONFIG
kind-single-node-% kind-multi-node-% kind-multi-zone-%: export KUBECONFIG = $(KUBECONFIG_RUNTIME_CLUSTER)
kind-single-node2-% kind-multi-node2-%: export KUBECONFIG = $(KUBECONFIG_SEED2_CLUSTER)

operator-% gardener-% garden-% seed-% gardenadm-% remote-%: export KUBECONFIG = $(KUBECONFIG_RUNTIME_CLUSTER)

test-e2e-%: export KUBECONFIG = $(KUBECONFIG_VIRTUAL_GARDEN_CLUSTER)
test-e2e-local-operator test-e2e-local-gardenadm-%: export KUBECONFIG = $(KUBECONFIG_RUNTIME_CLUSTER)

# KUBECONFIG_SEED_SECRET_PATH (used to create a Secret for Seeds containing the kubeconfig such that gardenctl works)
kind-single-node-% kind-multi-node-% kind-multi-zone-%: export KUBECONFIG_SEED_SECRET_PATH = $(DEV_SETUP)/gardenlet/components/kubeconfigs/seed-local/kubeconfig
kind-single-node2-% kind-multi-node2-%: export KUBECONFIG_SEED_SECRET_PATH = $(DEV_SETUP)/gardenlet/components/kubeconfigs/seed-local2/kubeconfig
remote-%: export KUBECONFIG_SEED_SECRET_PATH = $(DEV_SETUP)/gardenlet/components/kubeconfigs/seed-remote/kubeconfig

# CLUSTER_NAME
kind-single-node-% kind-multi-node-% kind-multi-zone-%: export CLUSTER_NAME = gardener-local
kind-single-node2-% kind-multi-node2-%: export CLUSTER_NAME = gardener-local2

# KUSTOMIZE_OVERLAY (the basename of the overlay in /dev-setup/kind/cluster/overlays for the scenario)
# Derived from the target name by stripping the `kind-` prefix and the `-up`/`-down` suffix,
# e.g., `kind-single-node-up` -> `single-node`, `kind-multi-zone-down` -> `multi-zone`.
kind-%: export KUSTOMIZE_OVERLAY = $(subst kind-,,$(subst -up,,$(subst -down,,$@)))

# kind*-{up,down}
kind-single-node-up kind-single-node-down \
kind-single-node2-up kind-single-node2-down \
kind-multi-node-up kind-multi-node-down \
kind-multi-node2-up kind-multi-node2-down \
kind-multi-zone-up kind-multi-zone-down: $(KIND) $(KUBECTL) $(HELM) $(YQ) $(KUSTOMIZE)
	$(DEV_SETUP)/kind.sh $(lastword $(subst -, ,$@))

kind-up: kind-single-node-up
kind-down: kind-single-node-down
kind2-up: kind-single-node2-up
kind2-down: kind-single-node2-down

# gind-{up,down}
gind-up gind-down: $(YQ)
	$(DEV_SETUP)/gind.sh $(subst gind-,,$@)

# speed-up skaffold deployments by building all images concurrently
export SKAFFOLD_BUILD_CONCURRENCY = 0
# build the images for the platform matching the nodes of the active kubernetes cluster, even in `skaffold build`, which doesn't enable this by default
export SKAFFOLD_CHECK_CLUSTER_NODE_PLATFORMS ?= true
export SKAFFOLD_DEFAULT_REPO ?= registry.local.gardener.cloud:5001
export SKAFFOLD_PUSH = true
export SOURCE_DATE_EPOCH = $(shell date -d $(BUILD_DATE) +%s)
export GARDENER_VERSION = $(VERSION)
# use static label for skaffold to prevent rolling all gardener components on every `skaffold` invocation
export SKAFFOLD_LABEL = "skaffold.dev/run-id=gardener-local"

%up %dev %debug: export LD_FLAGS = $(shell $(REPO_ROOT)/hack/get-build-ld-flags.sh k8s.io/component-base $(REPO_ROOT)/VERSION Gardener $(BUILD_DATE))
# skaffold dev and debug clean up deployed modules by default, disable this
%dev %debug: export SKAFFOLD_CLEANUP = false
# skaffold dev triggers new builds and deployments immediately on file changes by default, this is too heavy in a large
# project like gardener, so trigger new builds and deployments manually instead.
%dev: export SKAFFOLD_TRIGGER = manual
# Artifacts might be already built when you decide to start debugging.
# However, these artifacts do not include the gcflags which `skaffold debug` sets automatically, so delve would not work.
# Disabling the skaffold cache for debugging ensures that you run artifacts with gcflags required for debugging.
%debug: export SKAFFOLD_CACHE_ARTIFACTS = false

# cloud-provider-local-{up,dev,debug,down}
cloud-provider-local-%: export SKAFFOLD_FILENAME = $(DEV_SETUP)/skaffold-cloud-provider-local.yaml
cloud-provider-local-up cloud-provider-local-dev cloud-provider-local-debug cloud-provider-local-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(DEV_SETUP)/cloud-provider-local.sh $(subst cloud-provider-local-,,$@)

# operator-{up,dev,debug,down}
operator-%: export SKAFFOLD_FILENAME = $(DEV_SETUP)/skaffold-operator.yaml
operator-up operator-dev operator-debug operator-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(DEV_SETUP)/operator.sh $(subst operator-,,$@)

# remote-{up,down}
remote-up remote-down: $(KUBECTL)
	$(DEV_SETUP)/remote.sh $(subst remote-,,$@) $(DEV_SETUP_WITH_WORKLOAD_IDENTITY_SUPPORT)

# garden-{up,down}
garden-up garden-down: $(KUBECTL)
	$(DEV_SETUP)/garden.sh $(subst garden-,,$@)

# seed-{up,down}
seed-% seed2-%: export SKAFFOLD_FILENAME = $(DEV_SETUP)/skaffold-seed.yaml
seed-up seed-dev seed-debug seed-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(DEV_SETUP)/seed.sh $(subst seed-,,$@)
seed2-%: export KUBECONFIG = $(KUBECONFIG_SEED2_CLUSTER)
seed2-up seed2-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(DEV_SETUP)/seed.sh $(subst seed2-,,$@)

# gardener-{up,dev,down}
gardener-up gardener-dev: $(SKAFFOLD) $(HELM) $(KUBECTL) operator-up garden-up seed-up
gardener-down: $(SKAFFOLD) $(HELM) $(KUBECTL) seed-down garden-down

# gardenadm-{up,down}
gardenadm:
	BUILD_OUTPUT_FILE=./bin/ BUILD_PACKAGES=./cmd/gardenadm $(MAKE) build
# gardenadm-{up,down}
gardenadm-%: export SKAFFOLD_FILENAME = $(DEV_SETUP)/skaffold-gardenadm.yaml
gardenadm-up gardenadm-down: $(SKAFFOLD) $(KUBECTL)
	$(DEV_SETUP)/gardenadm.sh $(subst gardenadm-,,$@)

# e2e tests
PARALLEL_E2E_TESTS ?= 5

test-e2e-local: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="default" ./test/e2e/gardener/...
test-e2e-local-workerless: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="default && workerless" ./test/e2e/gardener/...
test-e2e-local-simple: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && simple" ./test/e2e/gardener/...
test-e2e-local-migration: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && control-plane-migration" ./test/e2e/gardener/...
test-e2e-local-migration-ha-multi-node: $(GINKGO)
	SHOOT_FAILURE_TOLERANCE_TYPE=node ./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && control-plane-migration" ./test/e2e/gardener/...
test-e2e-local-ha-multi-node: $(GINKGO)
	SHOOT_FAILURE_TOLERANCE_TYPE=node ./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "basic || (high-availability && update-to-node)" ./test/e2e/gardener/...
test-e2e-local-ha-multi-zone: $(GINKGO)
	SHOOT_FAILURE_TOLERANCE_TYPE=zone ./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "basic || (high-availability && update-to-zone)" ./test/e2e/gardener/...
test-e2e-local-operator: $(GINKGO)
	./hack/test-e2e-local.sh operator --procs=1 --label-filter="default" ./test/e2e/operator/...
test-e2e-local-gardenadm-managed-infra: $(GINKGO)
	./hack/test-e2e-local.sh gardenadm --procs=1 --label-filter="managed-infra" ./test/e2e/gardenadm/...
test-e2e-local-gardenadm-unmanaged-infra-initjoin: $(GINKGO)
	./hack/test-e2e-local.sh gardenadm --procs=1 --label-filter="unmanaged-infra && initjoin" ./test/e2e/gardenadm/...
test-e2e-local-gardenadm-unmanaged-infra-connect: $(GINKGO)
	./hack/test-e2e-local.sh gardenadm --procs=1 --label-filter="unmanaged-infra && connect" ./test/e2e/gardenadm/...
test-e2e-local-gardenadm-unmanaged-infra-seed: $(GINKGO)
	./hack/test-e2e-local.sh gardenadm --procs=1 --label-filter="unmanaged-infra && (seed || hosted-shoot)" ./test/e2e/gardenadm/...

test-e2e-non-ha-pre-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="pre-upgrade && !high-availability" ./test/e2e/gardener/...
test-e2e-pre-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="pre-upgrade" ./test/e2e/gardener/...
test-e2e-non-ha-post-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="post-upgrade && !high-availability" ./test/e2e/gardener/...
test-e2e-post-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="post-upgrade" ./test/e2e/gardener/...

# CI-related e2e test rules
GARDENER_PREVIOUS_RELEASE      := ""
GARDENER_NEXT_RELEASE          := $(VERSION)
GARDENER_RELEASE_DOWNLOAD_PATH := $(REPO_ROOT)/dev

ci-e2e-kind: $(KIND) $(YQ)
	./hack/ci-e2e-kind.sh
ci-e2e-kind-migration: $(KIND) $(YQ)
	./hack/ci-e2e-kind-migration.sh
ci-e2e-kind-migration-ha-multi-node: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE=node ./hack/ci-e2e-kind-migration-ha-multi-node.sh
ci-e2e-kind-ha-multi-node: $(KIND) $(YQ)
	./hack/ci-e2e-kind-ha-multi-node.sh
ci-e2e-kind-ha-multi-zone: $(KIND) $(YQ)
	./hack/ci-e2e-kind-ha-multi-zone.sh
ci-e2e-kind-operator: $(KIND) $(YQ)
	./hack/ci-e2e-kind-operator.sh
ci-e2e-kind-gardenadm-unmanaged-infra: $(KIND) $(YQ)
	./hack/ci-e2e-kind-gardenadm-unmanaged-infra.sh
ci-e2e-kind-gardenadm-unmanaged-infra-external-gardener: $(KIND) $(YQ)
	./hack/ci-e2e-kind-gardenadm-unmanaged-infra-external-gardener.sh
ci-e2e-kind-gardenadm-managed-infra: $(KIND) $(YQ)
	./hack/ci-e2e-kind-gardenadm-managed-infra.sh
ci-e2e-kind-upgrade: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE= GARDENER_PREVIOUS_RELEASE=$(GARDENER_PREVIOUS_RELEASE) GARDENER_RELEASE_DOWNLOAD_PATH=$(GARDENER_RELEASE_DOWNLOAD_PATH) GARDENER_NEXT_RELEASE=$(GARDENER_NEXT_RELEASE) ./hack/ci-e2e-kind-upgrade.sh
ci-e2e-kind-ha-multi-node-upgrade: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE=node GARDENER_PREVIOUS_RELEASE=$(GARDENER_PREVIOUS_RELEASE) GARDENER_RELEASE_DOWNLOAD_PATH=$(GARDENER_RELEASE_DOWNLOAD_PATH) GARDENER_NEXT_RELEASE=$(GARDENER_NEXT_RELEASE) ./hack/ci-e2e-kind-upgrade.sh
ci-e2e-kind-ha-multi-zone-upgrade: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE=zone GARDENER_PREVIOUS_RELEASE=$(GARDENER_PREVIOUS_RELEASE) GARDENER_RELEASE_DOWNLOAD_PATH=$(GARDENER_RELEASE_DOWNLOAD_PATH) GARDENER_NEXT_RELEASE=$(GARDENER_NEXT_RELEASE) ./hack/ci-e2e-kind-upgrade.sh

# envtest
ENVTEST_TYPE ?= kubernetes

.PHONY: start-envtest
start-envtest: $(SETUP_ENVTEST)
	@./hack/start-envtest.sh --environment-type=$(ENVTEST_TYPE)
