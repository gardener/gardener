# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY  := $(REGISTRY)/extensions/provider-local
EXTENSION_ADMISSION_LOCAL_IMAGE_REPOSITORY := $(REGISTRY)/extensions/admission-local
PUSH_LATEST_TAG                            := false
VERSION                                    := $(shell cat VERSION)
EFFECTIVE_VERSION                          := $(VERSION)-$(shell git rev-parse HEAD)
BUILD_DATE                                 := $(shell date '+%Y-%m-%dT%H:%M:%S%z' | sed 's/\([0-9][0-9]\)$$/:\1/g')
REPO_ROOT                                  := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
GARDENER_LOCAL_KUBECONFIG                  := $(REPO_ROOT)/example/gardener-local/kind/local/kubeconfig
GARDENER_LOCAL2_KUBECONFIG                 := $(REPO_ROOT)/example/gardener-local/kind/local2/kubeconfig
GARDENER_EXTENSIONS_KUBECONFIG             := $(REPO_ROOT)/example/gardener-local/kind/extensions/kubeconfig
GARDENER_LOCAL_HA_SINGLE_ZONE_KUBECONFIG   := $(REPO_ROOT)/example/gardener-local/kind/ha-single-zone/kubeconfig
GARDENER_LOCAL2_HA_SINGLE_ZONE_KUBECONFIG  := $(REPO_ROOT)/example/gardener-local/kind/ha-single-zone2/kubeconfig
GARDENER_LOCAL_HA_MULTI_ZONE_KUBECONFIG    := $(REPO_ROOT)/example/gardener-local/kind/ha-multi-zone/kubeconfig
GARDENER_LOCAL_OPERATOR_KUBECONFIG         := $(REPO_ROOT)/example/gardener-local/kind/operator/kubeconfig
GARDENER_PREVIOUS_RELEASE                  := ""
GARDENER_NEXT_RELEASE                      := $(VERSION)
LOCAL_GARDEN_LABEL                         := local-garden
REMOTE_GARDEN_LABEL                        := remote-garden
SEED_NAME                                  := provider-extensions
SEED_KUBECONFIG                            := $(REPO_ROOT)/example/provider-extensions/seed/kubeconfig
DEV_SETUP_WITH_WEBHOOKS                    := false
IPFAMILY                                   ?= ipv4
PARALLEL_E2E_TESTS                         := 5
GARDENER_RELEASE_DOWNLOAD_PATH             := $(REPO_ROOT)/dev
DEV_SETUP_WITH_LPP_RESIZE_SUPPORT          ?= false
DEV_SETUP_WITH_WORKLOAD_IDENTITY_SUPPORT   ?= false
PRINT_HELP ?=

ifneq ($(SEED_NAME),provider-extensions)
	SEED_KUBECONFIG := $(REPO_ROOT)/example/provider-extensions/seed/kubeconfig-$(SEED_NAME)
endif

ifndef ARTIFACTS
	export ARTIFACTS=/tmp/artifacts
endif

ifneq ($(strip $(shell git status --porcelain 2>/dev/null)),)
	EFFECTIVE_VERSION := $(EFFECTIVE_VERSION)-dirty
endif

SHELL=/usr/bin/env bash -o pipefail

#########################################
# Tools                                 #
#########################################

TOOLS_BIN_DIR ?= hack/tools/bin/$(go env GOOS)-$(go env GOARCH)
TOOLS_DIR := hack/tools
include hack/tools.mk

LOGCHECK_DIR := $(TOOLS_DIR)/logcheck

#########################################
# Rules for local development scenarios #
#########################################

ENVTEST_TYPE ?= kubernetes

.PHONY: start-envtest
start-envtest: $(SETUP_ENVTEST)
	@./hack/start-envtest.sh --environment-type=$(ENVTEST_TYPE)

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: install
install:
	@EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) ./hack/install.sh ./...

BUILD_OUTPUT_FILE ?= .
BUILD_PACKAGES ?= ./...

.PHONY: build
build:
	@EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) ./hack/build.sh -o $(BUILD_OUTPUT_FILE) $(BUILD_PACKAGES)

.PHONY: docker-images
docker-images:
	@echo "Building docker images with version and tag $(EFFECTIVE_VERSION)"
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(APISERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(APISERVER_IMAGE_REPOSITORY):latest                 -f Dockerfile --target apiserver .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)        -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):latest        -f Dockerfile --target controller-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(SCHEDULER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(SCHEDULER_IMAGE_REPOSITORY):latest                 -f Dockerfile --target scheduler .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(ADMISSION_IMAGE_REPOSITORY):latest                 -f Dockerfile --target admission-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(RESOURCE_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)          -t $(RESOURCE_MANAGER_IMAGE_REPOSITORY):latest          -f Dockerfile --target resource-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(NODE_AGENT_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(NODE_AGENT_IMAGE_REPOSITORY):latest                -f Dockerfile --target node-agent .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(OPERATOR_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                  -t $(OPERATOR_IMAGE_REPOSITORY):latest                  -f Dockerfile --target operator .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                 -t $(GARDENLET_IMAGE_REPOSITORY):latest                 -f Dockerfile --target gardenlet .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)  -t $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):latest  -f Dockerfile --target gardener-extension-provider-local .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(EXTENSION_ADMISSION_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION) -t $(EXTENSION_ADMISSION_LOCAL_IMAGE_REPOSITORY):latest -f Dockerfile --target gardener-extension-admission-local .

.PHONY: docker-push
docker-push:
	@if ! docker images $(APISERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(APISERVER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(CONTROLLER_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(CONTROLLER_MANAGER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(SCHEDULER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(SCHEDULER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(ADMISSION_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(ADMISSION_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(RESOURCE_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(RESOURCE_MANAGER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(NODE_AGENT_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(NODE_AGENT_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(GARDENLET_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(GARDENLET_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
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
	@docker push $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):latest; fi

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: tidy
tidy:
	@GO111MODULE=on go mod tidy
	@cd $(LOGCHECK_DIR); go mod tidy

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
check: $(GO_ADD_LICENSE) $(GOIMPORTS) $(GOLANGCI_LINT) $(HELM) $(IMPORT_BOSS) $(LOGCHECK) $(YQ) $(VGOPATH) $(TYPOS) logcheck-symlinks
	@sed ./.golangci.yaml.in -e "s#<<LOGCHECK_PLUGIN_PATH>>#$(TOOLS_BIN_DIR)#g" > ./.golangci.yaml
	@hack/check.sh --golangci-lint-config=./.golangci.yaml ./charts/... ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...
	@VGOPATH=$(VGOPATH) hack/check-imports.sh ./charts/... ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/... ./third_party/...

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

tools-for-generate: $(CONTROLLER_GEN) $(EXTENSION_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GOIMPORTS) $(GO_TO_PROTOBUF) $(HELM) $(MOCKGEN) $(OPENAPI_GEN) $(PROTOC) $(PROTOC_GEN_GOGO) $(YQ) $(VGOPATH)

define GENERATE_HELP_INFO
# Usage: make generate [WHAT="<targets>"] [MODE="<mode>"] [CODEGEN_GROUPS="<groups>"] [MANIFESTS_DIRS="<folders>"]
#
# Options:
#   WHAT              - Specify the targets to run (e.g., "protobuf codegen manifests logcheck")
#   CODEGEN_GROUPS    - Specify which groups to run the 'codegen' target for, not applicable for other targets (e.g., "authentication_groups core_groups extensions_groups resources_groups
#                       operator_groups seedmanagement_groups operations_groups settings_groups operatorconfig_groups controllermanager_groups admissioncontroller_groups scheduler_groups
#                       gardenlet_groups resourcemanager_groups shoottolerationrestriction_groups shootdnsrewriting_groups shootresourcereservation_groups provider_local_groups extensions_config_groups")
#   MANIFESTS_DIRS    - Specify which directories to run the 'manifests' target in, not applicable for other targets (Default directories are "charts cmd example extensions imagevector pkg plugin test")
#   MODE              - Specify the mode for the 'manifests' (default=parallel) or 'codegen' (default=sequential) target (e.g., "parallel" or "sequential")
#
# Examples:
#   make generate
#   make generate WHAT="codegen protobuf"
#   make generate WHAT="codegen protobuf" MODE="sequential"
#   make generate WHAT="manifests" MANIFESTS_DIRS="pkg/component plugin" MODE="sequential"
#   make generate WHAT="codegen" CODEGEN_GROUPS="core_groups extensions_groups"
#   make generate WHAT="codegen manifests" CODEGEN_GROUPS="operator_groups controllermanager_groups" MANIFESTS_DIRS="charts extensions/pkg"
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
	@REPO_ROOT=$(REPO_ROOT) VGOPATH=$(VGOPATH) LOGCHECK_DIR=$(LOGCHECK_DIR) hack/generate.sh --what "$(WHAT)" --codegen-groups "$(CODEGEN_GROUPS)" --manifests-dirs "$(MANIFESTS_DIRS)" --mode "$(MODE)"
	$(MAKE) format
endif

.PHONY: format
format: $(GOIMPORTS) $(GOIMPORTSREVISER)
	@./hack/format.sh ./charts ./cmd ./extensions ./pkg ./plugin ./test ./hack
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
check-apidiff: $(GO_APIDIFF)
	@./hack/check-apidiff.sh

.PHONY: check-vulnerabilities
check-vulnerabilities: $(GO_VULN_CHECK)
	$(GO_VULN_CHECK) ./...

.PHONY: verify
verify: check format test test-integration sast

.PHONY: verify-extended
verify-extended: check-generate check format test-cov test-cov-clean test-integration sast-report

#####################################################################
# Rules for local environment                                       #
#####################################################################

kind-% kind2-% gardener-%: export IPFAMILY := $(IPFAMILY)
# KUBECONFIG
kind-up kind-down gardener-up gardener-dev gardener-debug gardener-down gardenadm%up gardenadm%down: export KUBECONFIG = $(GARDENER_LOCAL_KUBECONFIG)
test-e2e-local-simple test-e2e-local-migration test-e2e-local-workerless test-e2e-local test-e2e-local-gardenadm ci-e2e-kind ci-e2e-kind-upgrade ci-e2e-kind-gardenadm: export KUBECONFIG = $(GARDENER_LOCAL_KUBECONFIG)
kind2-up kind2-down gardenlet-kind2-up gardenlet-kind2-dev gardenlet-kind2-debug gardenlet-kind2-down: export KUBECONFIG = $(GARDENER_LOCAL2_KUBECONFIG)
kind-extensions-up kind-extensions-down gardener-extensions-up gardener-extensions-down: export KUBECONFIG = $(GARDENER_EXTENSIONS_KUBECONFIG)
kind-ha-single-zone-up kind-ha-single-zone-down gardener-ha-single-zone-up gardener-ha-single-zone-down: export KUBECONFIG = $(GARDENER_LOCAL_HA_SINGLE_ZONE_KUBECONFIG)
test-e2e-local-ha-single-zone test-e2e-local-migration-ha-single-zone ci-e2e-kind-ha-single-zone ci-e2e-kind-ha-single-zone-upgrade: export KUBECONFIG = $(GARDENER_LOCAL_HA_SINGLE_ZONE_KUBECONFIG)
kind2-ha-single-zone-up kind2-ha-single-zone-down gardenlet-kind2-ha-single-zone-up gardenlet-kind2-ha-single-zone-dev gardenlet-kind2-ha-single-zone-debug gardenlet-kind2-ha-single-zone-down: export KUBECONFIG = $(GARDENER_LOCAL2_HA_SINGLE_ZONE_KUBECONFIG)
kind-ha-multi-zone-up kind-ha-multi-zone-down gardener-ha-multi-zone-up: export KUBECONFIG = $(GARDENER_LOCAL_HA_MULTI_ZONE_KUBECONFIG)
test-e2e-local-ha-multi-zone ci-e2e-kind-ha-multi-zone ci-e2e-kind-ha-multi-zone-upgrade: export KUBECONFIG = $(GARDENER_LOCAL_HA_MULTI_ZONE_KUBECONFIG)
kind-operator-up kind-operator-down operator%up operator-dev operator-debug operator%down operator-seed-dev test-e2e-local-operator ci-e2e-kind-operator ci-e2e-kind-operator-seed: export KUBECONFIG = $(GARDENER_LOCAL_OPERATOR_KUBECONFIG)
operator-seed-% test-e2e-local-operator-seed: export VIRTUAL_GARDEN_KUBECONFIG = $(REPO_ROOT)/example/operator/virtual-garden/kubeconfig
test-e2e-local-operator-seed: export KUBECONFIG = $(VIRTUAL_GARDEN_KUBECONFIG)
# CLUSTER_NAME
kind-up kind-down: export CLUSTER_NAME = gardener-local
kind2-up kind2-down: export CLUSTER_NAME = gardener-local2
kind-ha-single-zone-up kind-ha-single-zone-down: export CLUSTER_NAME = gardener-local-ha-single-zone
kind2-ha-single-zone-up kind2-ha-single-zone-down: export CLUSTER_NAME = gardener-local2-ha-single-zone
kind-ha-multi-zone-up kind-ha-multi-zone-down: export CLUSTER_NAME = gardener-local-ha-multi-zone
kind-operator-up kind-operator-down: export CLUSTER_NAME = gardener-operator-local
# KIND_KUBECONFIG
kind-up kind-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-kind/base/kubeconfig
kind2-up kind2-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-kind2/base/kubeconfig
kind-ha-single-zone-up kind-ha-single-zone-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-kind-ha-single-zone/base/kubeconfig
kind2-ha-single-zone-up kind2-ha-single-zone-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-kind2-ha-single-zone/base/kubeconfig
kind-ha-multi-zone-up kind-ha-multi-zone-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-kind-ha-multi-zone/base/kubeconfig
kind-operator-up kind-operator-down: export KIND_KUBECONFIG = $(REPO_ROOT)/example/provider-local/seed-operator/base/kubeconfig
# CLUSTER_VALUES
kind-up kind-down: export CLUSTER_VALUES = $(REPO_ROOT)/example/gardener-local/kind/local/values.yaml
kind2-up kind2-down: export CLUSTER_VALUES = $(REPO_ROOT)/example/gardener-local/kind/local2/values.yaml
kind-ha-single-zone-up kind-ha-single-zone-down: export CLUSTER_VALUES = $(REPO_ROOT)/example/gardener-local/kind/ha-single-zone/values.yaml
kind2-ha-single-zone-up kind2-ha-single-zone-down: export CLUSTER_VALUES = $(REPO_ROOT)/example/gardener-local/kind/ha-single-zone2/values.yaml
kind-ha-multi-zone-up kind-ha-multi-zone-down: export CLUSTER_VALUES = $(REPO_ROOT)/example/gardener-local/kind/ha-multi-zone/values.yaml
# ADDITIONAL_PARAMETERS
kind2-up kind2-ha-single-zone-up: export ADDITIONAL_PARAMETERS = --skip-registry
kind2-down: export ADDITIONAL_PARAMETERS = --keep-backupbuckets-dir
kind-ha-multi-zone-up kind-operator-up: export ADDITIONAL_PARAMETERS = --multi-zonal

kind-up kind2-up kind-ha-single-zone-up kind2-ha-single-zone-up kind-ha-multi-zone-up: $(KIND) $(KUBECTL) $(HELM) $(YQ) $(KUSTOMIZE)
	./hack/kind-up.sh \
		--cluster-name $(CLUSTER_NAME) \
		--path-kubeconfig $(KIND_KUBECONFIG) \
		--path-cluster-values $(CLUSTER_VALUES) \
		--with-lpp-resize-support $(DEV_SETUP_WITH_LPP_RESIZE_SUPPORT) \
		$(ADDITIONAL_PARAMETERS)
kind-down kind2-down kind-ha-single-zone-down kind2-ha-single-zone-down kind-ha-multi-zone-down: $(KIND)
	./hack/kind-down.sh \
		--cluster-name $(CLUSTER_NAME) \
		--path-kubeconfig $(KIND_KUBECONFIG) \
		$(ADDITIONAL_PARAMETERS)

kind-extensions-up: $(KIND) $(KUBECTL)
	REPO_ROOT=$(REPO_ROOT) ./hack/kind-extensions-up.sh
kind-extensions-down:
	docker stop gardener-extensions-control-plane
kind-extensions-clean: $(KIND)
	./hack/kind-down.sh --cluster-name gardener-extensions --path-kubeconfig $(REPO_ROOT)/example/provider-extensions/garden/kubeconfig

kind-operator-up: $(KIND) $(KUBECTL) $(HELM) $(YQ) $(KUSTOMIZE)
	./hack/kind-up.sh \
		--cluster-name $(CLUSTER_NAME) \
		--path-kubeconfig $(KIND_KUBECONFIG) \
		--path-cluster-values $(REPO_ROOT)/example/gardener-local/kind/operator/values.yaml \
		--with-lpp-resize-support $(DEV_SETUP_WITH_LPP_RESIZE_SUPPORT) \
		$(ADDITIONAL_PARAMETERS)
	mkdir -p $(REPO_ROOT)/dev/local-backupbuckets/gardener-operator
kind-operator-down: $(KIND)
	./hack/kind-down.sh \
		--cluster-name $(CLUSTER_NAME) \
		--path-kubeconfig $(KIND_KUBECONFIG)
	# We need root privileges to clean the backup bucket directory, see https://github.com/gardener/gardener/issues/6752
	docker run --rm --user root:root -v $(REPO_ROOT)/dev/local-backupbuckets:/dev/local-backupbuckets alpine rm -rf /dev/local-backupbuckets/gardener-operator

# speed-up skaffold deployments by building all images concurrently
export SKAFFOLD_BUILD_CONCURRENCY = 0
gardener%up gardener%dev gardener%debug gardenlet%up gardenlet%dev gardenlet%debug operator%up operator-dev operator-debug operator-seed-dev gardenadm%up: export SKAFFOLD_DEFAULT_REPO = garden.local.gardener.cloud:5001
gardener%up gardener%dev gardener%debug gardenlet%up gardenlet%dev gardenlet%debug operator%up operator-dev operator-debug operator-seed-dev gardenadm%up: export SKAFFOLD_PUSH = true
gardener%up gardener%dev gardener%debug gardenlet%up gardenlet%dev gardenlet%debug operator%up operator-dev operator-debug operator-seed-dev gardenadm%up: export SOURCE_DATE_EPOCH = $(shell date -d $(BUILD_DATE) +%s)
gardener%up gardener%dev gardener%debug gardenlet%up gardenlet%dev gardenlet%debug operator%up operator-dev operator-debug operator-seed-dev gardenadm%up: export GARDENER_VERSION = $(VERSION)
# use static label for skaffold to prevent rolling all gardener components on every `skaffold` invocation
gardener%up gardener%dev gardener%debug gardenlet%up gardenlet%dev gardenlet%debug operator%up gardenadm%up: export SKAFFOLD_LABEL = skaffold.dev/run-id=gardener-local
# set ldflags for skaffold
gardener%up gardener%dev gardener%debug gardenlet%up gardenlet%dev gardenlet%debug operator%up operator-dev operator-debug operator-seed-dev gardenadm%up: export LD_FLAGS = $(shell $(REPO_ROOT)/hack/get-build-ld-flags.sh k8s.io/component-base $(REPO_ROOT)/VERSION Gardener $(BUILD_DATE))
# skaffold dev and debug clean up deployed modules by default, disable this
gardener%dev gardener%debug gardenlet%dev gardenlet%debug operator-dev operator-debug operator-seed-dev: export SKAFFOLD_CLEANUP = false
# skaffold dev triggers new builds and deployments immediately on file changes by default,
# this is too heavy in a large project like gardener, so trigger new builds and deployments manually instead.
gardener%dev gardenlet%dev operator-dev operator-seed-dev: export SKAFFOLD_TRIGGER = manual
# Artifacts might be already built when you decide to start debugging.
# However, these artifacts do not include the gcflags which `skaffold debug` sets automatically, so delve would not work.
# Disabling the skaffold cache for debugging ensures that you run artifacts with gcflags required for debugging.
gardener%debug gardenlet%debug operator-debug: export SKAFFOLD_CACHE_ARTIFACTS = false

gardener-ha-single-zone%: export SKAFFOLD_PROFILE=ha-single-zone
gardener-ha-multi-zone%: export SKAFFOLD_PROFILE=ha-multi-zone

gardener-up gardener-ha-single-zone-up gardener-ha-multi-zone-up: $(SKAFFOLD) $(HELM) $(KUBECTL) $(YQ)
	$(SKAFFOLD) run
gardener-dev gardener-ha-single-zone-dev gardener-ha-multi-zone-dev: $(SKAFFOLD) $(HELM) $(KUBECTL) $(YQ)
	$(SKAFFOLD) dev
gardener-debug gardener-ha-single-zone-debug gardener-ha-multi-zone-debug: $(SKAFFOLD) $(HELM) $(KUBECTL) $(YQ)
	$(SKAFFOLD) debug
gardener-down gardener-ha-single-zone-down gardener-ha-multi-zone-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	./hack/gardener-down.sh

gardener-extensions-%: export SKAFFOLD_LABEL = skaffold.dev/run-id=gardener-extensions

gardener-extensions-up: $(SKAFFOLD) $(HELM) $(KUBECTL) $(YQ) $(OIDC_METADATA)
	./hack/gardener-extensions-up.sh --path-garden-kubeconfig $(REPO_ROOT)/example/provider-extensions/garden/kubeconfig --path-seed-kubeconfig $(SEED_KUBECONFIG) --seed-name $(SEED_NAME) --with-workload-identity-support $(DEV_SETUP_WITH_WORKLOAD_IDENTITY_SUPPORT)
gardener-extensions-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	./hack/gardener-extensions-down.sh --path-garden-kubeconfig $(REPO_ROOT)/example/provider-extensions/garden/kubeconfig --path-seed-kubeconfig $(SEED_KUBECONFIG) --seed-name $(SEED_NAME) --with-workload-identity-support $(DEV_SETUP_WITH_WORKLOAD_IDENTITY_SUPPORT)

gardenlet-kind2-up gardenlet-kind2-dev gardenlet-kind2-debug gardenlet-kind2-down: export SKAFFOLD_PREFIX_NAME = kind2
gardenlet-kind2-ha-single-zone-up gardenlet-kind2-ha-single-zone-dev gardenlet-kind2-ha-single-zone-debug gardenlet-kind2-ha-single-zone-down: export SKAFFOLD_PREFIX_NAME = kind2-ha-single-zone
gardenlet-kind2-up gardenlet-kind2-dev gardenlet-kind2-debug gardenlet-kind2-down: export SKAFFOLD_COMMAND_KUBECONFIG := $(GARDENER_LOCAL_KUBECONFIG)
gardenlet-kind2-ha-single-zone-up gardenlet-kind2-ha-single-zone-dev gardenlet-kind2-ha-single-zone-debug gardenlet-kind2-ha-single-zone-down: export SKAFFOLD_COMMAND_KUBECONFIG := $(GARDENER_LOCAL_HA_SINGLE_ZONE_KUBECONFIG)

gardenlet-kind2-up gardenlet-kind2-ha-single-zone-up: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) deploy -m $(SKAFFOLD_PREFIX_NAME)-env -p $(SKAFFOLD_PREFIX_NAME) --kubeconfig=$(SKAFFOLD_COMMAND_KUBECONFIG)
	@# define GARDENER_LOCAL_KUBECONFIG so that it can be used by skaffold when checking whether the seed managed by this gardenlet is ready
	GARDENER_LOCAL_KUBECONFIG=$(SKAFFOLD_COMMAND_KUBECONFIG) $(SKAFFOLD) run -m gardenlet -p $(SKAFFOLD_PREFIX_NAME)
gardenlet-kind2-dev gardenlet-kind2-ha-single-zone-dev: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) deploy -m $(SKAFFOLD_PREFIX_NAME)-env -p $(SKAFFOLD_PREFIX_NAME) --kubeconfig=$(SKAFFOLD_COMMAND_KUBECONFIG)
	@# define GARDENER_LOCAL_KUBECONFIG so that it can be used by skaffold when checking whether the seed managed by this gardenlet is ready
	GARDENER_LOCAL_KUBECONFIG=$(SKAFFOLD_COMMAND_KUBECONFIG) $(SKAFFOLD) dev -m gardenlet -p $(SKAFFOLD_PREFIX_NAME)
gardenlet-kind2-debug gardenlet-kind2-ha-single-zone-debug: $(SKAFFOLD) $(HELM)
	$(SKAFFOLD) deploy -m $(SKAFFOLD_PREFIX_NAME)-env -p $(SKAFFOLD_PREFIX_NAME) --kubeconfig=$(SKAFFOLD_COMMAND_KUBECONFIG)
	@# define GARDENER_LOCAL_KUBECONFIG so that it can be used by skaffold when checking whether the seed managed by this gardenlet is ready
	GARDENER_LOCAL_KUBECONFIG=$(SKAFFOLD_COMMAND_KUBECONFIG) $(SKAFFOLD) debug -m gardenlet -p $(SKAFFOLD_PREFIX_NAME)
gardenlet-kind2-down gardenlet-kind2-ha-single-zone-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) delete -m $(SKAFFOLD_PREFIX_NAME)-env -p $(SKAFFOLD_PREFIX_NAME) --kubeconfig=$(SKAFFOLD_COMMAND_KUBECONFIG)
	$(SKAFFOLD) delete -m gardenlet,$(SKAFFOLD_PREFIX_NAME)-env -p $(SKAFFOLD_PREFIX_NAME)

operator-%: export SKAFFOLD_FILENAME = skaffold-operator.yaml

operator-up: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) run --cache-artifacts=$(shell ./hack/get-skaffold-cache-artifacts.sh)
operator-dev: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) dev
operator-debug: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(SKAFFOLD) debug
operator-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	$(KUBECTL) annotate garden --all confirmation.gardener.cloud/deletion=true
	$(KUBECTL) delete garden --all --ignore-not-found --wait --timeout 5m
	$(SKAFFOLD) delete

operator-seed-up: SKAFFOLD_MODE=run
operator-seed-up: SKAFFOLD_PROFILE=operator
operator-seed-dev: SKAFFOLD_MODE=dev
operator-seed-dev: SKAFFOLD_PROFILE=operator,operator-dev
operator-seed-up operator-seed-dev: $(SKAFFOLD) $(HELM) $(KUBECTL) operator-up
	$(SKAFFOLD) run -m garden -f=skaffold-operator-garden.yaml
	$(SKAFFOLD) run -m garden-config -f=skaffold-operator-garden.yaml --kubeconfig=$(VIRTUAL_GARDEN_KUBECONFIG) \
		--status-check=false --platform="linux/$(SYSTEM_ARCH)" 	# deployments don't exist in virtual-garden, see https://skaffold.dev/docs/status-check/; nodes don't exist in virtual-garden, ensure skaffold use the host architecture instead of amd64, see https://skaffold.dev/docs/workflows/handling-platforms/
	$(SKAFFOLD) $(SKAFFOLD_MODE) -m gardenlet -p $(SKAFFOLD_PROFILE) -f=skaffold.yaml --kubeconfig=$(VIRTUAL_GARDEN_KUBECONFIG) \
		--cache-artifacts=$(shell ./hack/get-skaffold-cache-artifacts.sh) \
		--status-check=false --platform="linux/$(SYSTEM_ARCH)" 	# deployments don't exist in virtual-garden, see https://skaffold.dev/docs/status-check/; nodes don't exist in virtual-garden, ensure skaffold use the host architecture instead of amd64, see https://skaffold.dev/docs/workflows/handling-platforms/
	TIMEOUT=900 ./hack/usage/wait-for.sh garden local VirtualGardenAPIServerAvailable RuntimeComponentsHealthy VirtualComponentsHealthy

operator-seed-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	./hack/operator-seed-down.sh --path-kind-kubeconfig $(KUBECONFIG) --path-garden-kubeconfig $(VIRTUAL_GARDEN_KUBECONFIG)

gardenadm-high-touch-up: $(SKAFFOLD) $(KUBECTL)
	$(SKAFFOLD) run -n gardenadm-high-touch -f=skaffold-gardenadm.yaml
gardenadm-high-touch-down: $(SKAFFOLD) $(KUBECTL)
	$(SKAFFOLD) delete -n gardenadm-high-touch -f=skaffold-gardenadm.yaml
gardenadm-medium-touch-up: $(SKAFFOLD) $(KUBECTL)
	$(SKAFFOLD) build -f=skaffold-gardenadm.yaml -m gardenadm,provider-local-node,provider-local -q | $(SKAFFOLD) render -f=skaffold-gardenadm.yaml -m provider-local-node,provider-local -o ./example/gardenadm-local/medium-touch/config.yaml --build-artifacts -
gardenadm-medium-touch-down: $(SKAFFOLD) $(KUBECTL)
	$(SKAFFOLD) delete -n gardenadm-medium-touch -f=skaffold-gardenadm.yaml

test-e2e-local: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="default" ./test/e2e/gardener/...
test-e2e-local-workerless: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="default && workerless" ./test/e2e/gardener/...
test-e2e-local-simple: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && simple" ./test/e2e/gardener/...
test-e2e-local-migration: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && control-plane-migration" ./test/e2e/gardener/...
test-e2e-local-migration-ha-single-zone: $(GINKGO)
	SHOOT_FAILURE_TOLERANCE_TYPE=node ./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && control-plane-migration" ./test/e2e/gardener/...
test-e2e-local-ha-single-zone: $(GINKGO)
	SHOOT_FAILURE_TOLERANCE_TYPE=node ./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "basic || (high-availability && update-to-node)" ./test/e2e/gardener/...
test-e2e-local-ha-multi-zone: $(GINKGO)
	SHOOT_FAILURE_TOLERANCE_TYPE=zone USE_PROVIDER_LOCAL_COREDNS_SERVER=true ./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "basic || (high-availability && update-to-zone)" ./test/e2e/gardener/...
test-e2e-local-operator: $(GINKGO)
	./hack/test-e2e-local.sh operator --procs=1 --label-filter="default" ./test/e2e/operator/...
test-e2e-local-operator-seed: $(GINKGO)
	USE_PROVIDER_LOCAL_COREDNS_SERVER=true ./hack/test-e2e-local.sh operator-seed --procs=$(PARALLEL_E2E_TESTS) --label-filter="default && ManagedSeed" ./test/e2e/gardener/...
test-e2e-local-gardenadm: $(GINKGO)
	./hack/test-e2e-local.sh gardenadm --procs=1 ./test/e2e/gardenadm/...

test-non-ha-pre-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="pre-upgrade && !high-availability" ./test/e2e/gardener/...
test-pre-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="pre-upgrade" ./test/e2e/gardener/...

test-non-ha-post-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="post-upgrade && !high-availability" ./test/e2e/gardener/...
test-post-upgrade: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="post-upgrade" ./test/e2e/gardener/...

ci-e2e-kind: $(KIND) $(YQ)
	./hack/ci-e2e-kind.sh
ci-e2e-kind-migration: $(KIND) $(YQ)
	GARDENER_LOCAL_KUBECONFIG=$(GARDENER_LOCAL_KUBECONFIG) GARDENER_LOCAL2_KUBECONFIG=$(GARDENER_LOCAL2_KUBECONFIG) ./hack/ci-e2e-kind-migration.sh
ci-e2e-kind-migration-ha-single-zone: $(KIND) $(YQ)
	GARDENER_LOCAL_KUBECONFIG=$(GARDENER_LOCAL_HA_SINGLE_ZONE_KUBECONFIG) GARDENER_LOCAL2_KUBECONFIG=$(GARDENER_LOCAL2_HA_SINGLE_ZONE_KUBECONFIG) SHOOT_FAILURE_TOLERANCE_TYPE=node ./hack/ci-e2e-kind-migration-ha-single-zone.sh
ci-e2e-kind-ha-single-zone: $(KIND) $(YQ)
	./hack/ci-e2e-kind-ha-single-zone.sh
ci-e2e-kind-ha-multi-zone: $(KIND) $(YQ)
	./hack/ci-e2e-kind-ha-multi-zone.sh
ci-e2e-kind-operator: $(KIND) $(YQ)
	./hack/ci-e2e-kind-operator.sh
ci-e2e-kind-operator-seed: $(KIND) $(YQ)
	./hack/ci-e2e-kind-operator-seed.sh
ci-e2e-kind-gardenadm: $(KIND) $(YQ)
	./hack/ci-e2e-kind-gardenadm.sh

ci-e2e-kind-upgrade: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE= GARDENER_PREVIOUS_RELEASE=$(GARDENER_PREVIOUS_RELEASE) GARDENER_RELEASE_DOWNLOAD_PATH=$(GARDENER_RELEASE_DOWNLOAD_PATH) GARDENER_NEXT_RELEASE=$(GARDENER_NEXT_RELEASE) ./hack/ci-e2e-kind-upgrade.sh
ci-e2e-kind-ha-single-zone-upgrade: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE=node GARDENER_PREVIOUS_RELEASE=$(GARDENER_PREVIOUS_RELEASE) GARDENER_RELEASE_DOWNLOAD_PATH=$(GARDENER_RELEASE_DOWNLOAD_PATH) GARDENER_NEXT_RELEASE=$(GARDENER_NEXT_RELEASE) ./hack/ci-e2e-kind-upgrade.sh
ci-e2e-kind-ha-multi-zone-upgrade: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE=zone USE_PROVIDER_LOCAL_COREDNS_SERVER=true GARDENER_PREVIOUS_RELEASE=$(GARDENER_PREVIOUS_RELEASE) GARDENER_RELEASE_DOWNLOAD_PATH=$(GARDENER_RELEASE_DOWNLOAD_PATH) GARDENER_NEXT_RELEASE=$(GARDENER_NEXT_RELEASE) ./hack/ci-e2e-kind-upgrade.sh
