# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# This make file is supposed to be included in the top-level make file.
# It can be reused by repos vendoring g/g to have some common make recipes for building and installing development
# tools as needed.
# Recipes in the top-level make file should declare dependencies on the respective tool recipes (e.g. $(CONTROLLER_GEN))
# as needed. If the required tool (version) is not built/installed yet, make will make sure to build/install it.
# The *_VERSION variables in this file contain the "default" values, but can be overwritten in the top level make file.

ifeq ($(strip $(shell go list -m 2>/dev/null)),github.com/gardener/gardener)
TOOLS_PKG_PATH             := ./hack/tools
else
# dependency on github.com/gardener/gardener/hack/tools is optional and only needed if other projects want to reuse
# install-promtool.sh, or logcheck. If they don't use it and the project doesn't depend on the package,
# silence the error to minimize confusion.
TOOLS_PKG_PATH             := $(shell go list -tags tools -f '{{ .Dir }}' github.com/gardener/gardener/hack/tools 2>/dev/null)
endif

SYSTEM_NAME                := $(shell uname -s | tr '[:upper:]' '[:lower:]')
SYSTEM_ARCH                := $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
TOOLS_BIN_DIR              := $(TOOLS_DIR)/bin/$(SYSTEM_NAME)-$(SYSTEM_ARCH)
CONTROLLER_GEN             := $(TOOLS_BIN_DIR)/controller-gen
EXTENSION_GEN              := $(TOOLS_BIN_DIR)/extension-generator
GEN_CRD_API_REFERENCE_DOCS := $(TOOLS_BIN_DIR)/gen-crd-api-reference-docs
GINKGO                     := $(TOOLS_BIN_DIR)/ginkgo
GOIMPORTS                  := $(TOOLS_BIN_DIR)/goimports
GOIMPORTSREVISER           := $(TOOLS_BIN_DIR)/goimports-reviser
GOLANGCI_LINT              := $(TOOLS_BIN_DIR)/golangci-lint
GOSEC                      := $(TOOLS_BIN_DIR)/gosec
GO_ADD_LICENSE             := $(TOOLS_BIN_DIR)/addlicense
GO_APIDIFF                 := $(TOOLS_BIN_DIR)/go-apidiff
GO_VULN_CHECK              := $(TOOLS_BIN_DIR)/govulncheck
GO_TO_PROTOBUF             := $(TOOLS_BIN_DIR)/go-to-protobuf
HELM                       := $(TOOLS_BIN_DIR)/helm
IMPORT_BOSS                := $(TOOLS_BIN_DIR)/import-boss
KIND                       := $(TOOLS_BIN_DIR)/kind
KUBECTL                    := $(TOOLS_BIN_DIR)/kubectl
KUSTOMIZE                  := $(TOOLS_BIN_DIR)/kustomize
LOGCHECK                   := $(TOOLS_BIN_DIR)/logcheck.so # plugin binary
MOCKGEN                    := $(TOOLS_BIN_DIR)/mockgen
OPENAPI_GEN                := $(TOOLS_BIN_DIR)/openapi-gen
PROMTOOL                   := $(TOOLS_BIN_DIR)/promtool
PROTOC                     := $(TOOLS_BIN_DIR)/protoc
PROTOC_GEN_GOGO            := $(TOOLS_BIN_DIR)/protoc-gen-gogo
REPORT_COLLECTOR           := $(TOOLS_BIN_DIR)/report-collector
OIDC_METADATA              := $(TOOLS_BIN_DIR)/oidcmeta
SETUP_ENVTEST              := $(TOOLS_BIN_DIR)/setup-envtest
SKAFFOLD                   := $(TOOLS_BIN_DIR)/skaffold
YQ                         := $(TOOLS_BIN_DIR)/yq
VGOPATH                    := $(TOOLS_BIN_DIR)/vgopath
TYPOS                      := $(TOOLS_BIN_DIR)/typos

# default tool versions
# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.1.6
# renovate: datasource=github-releases depName=securego/gosec
GOSEC_VERSION ?= v2.22.4
# renovate: datasource=github-releases depName=joelanford/go-apidiff
GO_APIDIFF_VERSION ?= v0.8.3
# renovate: datasource=github-releases depName=google/addlicense
GO_ADD_LICENSE_VERSION ?= v1.1.1
# renovate: datasource=github-releases depName=incu6us/goimports-reviser
GOIMPORTSREVISER_VERSION ?= v3.9.1
GO_VULN_CHECK_VERSION ?= latest
# renovate: datasource=github-releases depName=helm/helm
HELM_VERSION ?= v3.17.3
# renovate: datasource=github-releases depName=kubernetes-sigs/kind
KIND_VERSION ?= v0.28.0
# renovate: datasource=github-releases depName=kubernetes/kubernetes
KUBECTL_VERSION ?= v1.33.1
# renovate: datasource=github-releases depName=kubernetes-sigs/kustomize
KUSTOMIZE_VERSION ?= v5.3.0
# renovate: datasource=github-releases depName=prometheus/prometheus
PROMTOOL_VERSION ?= 3.4.0
# renovate: datasource=github-releases depName=protocolbuffers/protobuf
PROTOC_VERSION ?= v31.0
# renovate: datasource=github-releases depName=GoogleContainerTools/skaffold
SKAFFOLD_VERSION ?= v2.16.0
# renovate: datasource=github-releases depName=mikefarah/yq
YQ_VERSION ?= v4.45.4
# renovate: datasource=github-releases depName=ironcore-dev/vgopath
VGOPATH_VERSION ?= v0.1.8
# renovate: datasource=github-releases depName=crate-ci/typos
TYPOS_VERSION ?= v1.32.0

# tool versions from go.mod
CONTROLLER_GEN_VERSION ?= $(call version_gomod,sigs.k8s.io/controller-tools)
GINKGO_VERSION ?= $(call version_gomod,github.com/onsi/ginkgo/v2)
GEN_CRD_API_REFERENCE_DOCS_VERSION ?= $(call version_gomod,github.com/ahmetb/gen-crd-api-reference-docs)
GOIMPORTS_VERSION ?= $(call version_gomod,golang.org/x/tools)
CODE_GENERATOR_VERSION ?= $(call version_gomod,k8s.io/code-generator)
MOCKGEN_VERSION ?= $(call version_gomod,go.uber.org/mock)
OPENAPI_GEN_VERSION ?= $(call version_gomod,k8s.io/kube-openapi)
CONTROLLER_RUNTIME_VERSION ?= $(call version_gomod,sigs.k8s.io/controller-runtime)
K8S_VERSION ?= $(subst v0,v1,$(call version_gomod,k8s.io/api))

# default dir for importing tool binaries
TOOLS_BIN_SOURCE_DIR ?= /gardenertools

export TOOLS_BIN_DIR := $(TOOLS_BIN_DIR)
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

#########################################
# Common                                #
#########################################

# Tool targets should declare go.mod as a prerequisite, if the tool's version is managed via go modules. This causes
# make to rebuild the tool in the desired version, when go.mod is changed.
# For tools where the version is not managed via go.mod, we use a file per tool and version as an indicator for make
# whether we need to install the tool or a different version of the tool (make doesn't rerun the rule if the rule is
# changed).

# Use this "function" to add the version file as a prerequisite for the tool target: e.g.
#   $(HELM): $(call tool_version_file,$(HELM),$(HELM_VERSION))
tool_version_file = $(TOOLS_BIN_DIR)/.version_$(subst $(TOOLS_BIN_DIR)/,,$(1))_$(2)

# Use this function to get the version of a go module from go.mod
version_gomod = $(shell go list -mod=mod -f '{{ .Version }}' -m $(1))

# This target cleans up any previous version files for the given tool and creates the given version file.
# This way, we can generically determine, which version was installed without calling each and every binary explicitly.
$(TOOLS_BIN_DIR)/.version_%:
	@version_file=$@; rm -f $${version_file%_*}*
	@mkdir -p $(TOOLS_BIN_DIR)
	@touch $@

.PHONY: clean-tools-bin
clean-tools-bin:
	rm -rf $(TOOLS_BIN_DIR)/*

.PHONY: import-tools-bin
import-tools-bin:
ifeq ($(shell if [ -d $(TOOLS_BIN_SOURCE_DIR) ]; then echo "found"; fi),found)
	@echo "Copying tool binaries from $(TOOLS_BIN_SOURCE_DIR)"
	@cp -rpT $(TOOLS_BIN_SOURCE_DIR) $(TOOLS_BIN_DIR)
endif

.PHONY: create-tools-bin
create-tools-bin: $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GINKGO) $(GOIMPORTS) $(GOIMPORTSREVISER) $(GOSEC) $(GO_ADD_LICENSE) $(GO_APIDIFF) $(GO_VULN_CHECK) $(GO_TO_PROTOBUF) $(HELM) $(IMPORT_BOSS) $(KIND) $(KUBECTL) $(MOCKGEN) $(OPENAPI_GEN) $(PROMTOOL) $(PROTOC) $(PROTOC_GEN_GOGO) $(SETUP_ENVTEST) $(SKAFFOLD) $(YQ) $(VGOPATH) $(KUSTOMIZE) $(TYPOS)

#########################################
# Tools                                 #
#########################################

$(CONTROLLER_GEN): $(call tool_version_file,$(CONTROLLER_GEN),$(CONTROLLER_GEN_VERSION))
	go build -o $(CONTROLLER_GEN) sigs.k8s.io/controller-tools/cmd/controller-gen

$(GEN_CRD_API_REFERENCE_DOCS): $(call tool_version_file,$(GEN_CRD_API_REFERENCE_DOCS),$(GEN_CRD_API_REFERENCE_DOCS_VERSION))
	go build -o $(GEN_CRD_API_REFERENCE_DOCS) github.com/ahmetb/gen-crd-api-reference-docs

$(GINKGO): $(call tool_version_file,$(GINKGO),$(GINKGO_VERSION))
	go build -o $(GINKGO) github.com/onsi/ginkgo/v2/ginkgo

$(GOIMPORTS): $(call tool_version_file,$(GOIMPORTS),$(GOIMPORTS_VERSION))
	go build -o $(GOIMPORTS) golang.org/x/tools/cmd/goimports

$(GOIMPORTSREVISER): $(call tool_version_file,$(GOIMPORTSREVISER),$(GOIMPORTSREVISER_VERSION))
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/incu6us/goimports-reviser/v3@$(GOIMPORTSREVISER_VERSION)

$(GOLANGCI_LINT): $(call tool_version_file,$(GOLANGCI_LINT),$(GOLANGCI_LINT_VERSION))
	@# CGO_ENABLED has to be set to 1 in order for golangci-lint to be able to load plugins
	@# see https://github.com/golangci/golangci-lint/issues/1276
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) CGO_ENABLED=1 go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(GOSEC): $(call tool_version_file,$(GOSEC),$(GOSEC_VERSION))
	@GOSEC_VERSION=$(GOSEC_VERSION) bash $(TOOLS_PKG_PATH)/install-gosec.sh

$(GO_ADD_LICENSE):  $(call tool_version_file,$(GO_ADD_LICENSE),$(GO_ADD_LICENSE_VERSION))
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/google/addlicense@$(GO_ADD_LICENSE_VERSION)

$(GO_APIDIFF): $(call tool_version_file,$(GO_APIDIFF),$(GO_APIDIFF_VERSION))
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/joelanford/go-apidiff@$(GO_APIDIFF_VERSION)

$(GO_VULN_CHECK): $(call tool_version_file,$(GO_VULN_CHECK),$(GO_VULN_CHECK_VERSION))
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install golang.org/x/vuln/cmd/govulncheck@$(GO_VULN_CHECK_VERSION)

$(GO_TO_PROTOBUF): $(call tool_version_file,$(GO_TO_PROTOBUF),$(CODE_GENERATOR_VERSION))
	go build -o $(GO_TO_PROTOBUF) k8s.io/code-generator/cmd/go-to-protobuf

$(HELM): $(call tool_version_file,$(HELM),$(HELM_VERSION))
	curl -sSfL https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | HELM_INSTALL_DIR=$(TOOLS_BIN_DIR) USE_SUDO=false bash -s -- --version $(HELM_VERSION)

$(IMPORT_BOSS): $(call tool_version_file,$(IMPORT_BOSS),$(K8S_VERSION))
	mkdir -p hack/tools/bin/work/import-boss
	curl -L -o hack/tools/bin/work/import-boss/main.go https://raw.githubusercontent.com/kubernetes/kubernetes/$(K8S_VERSION)/cmd/import-boss/main.go
	go build -o $(IMPORT_BOSS) ./hack/tools/bin/work/import-boss

$(KIND): $(call tool_version_file,$(KIND),$(KIND_VERSION))
	curl -L -o $(KIND) https://kind.sigs.k8s.io/dl/$(KIND_VERSION)/kind-$(SYSTEM_NAME)-$(SYSTEM_ARCH)
	chmod +x $(KIND)

$(KUBECTL): $(call tool_version_file,$(KUBECTL),$(KUBECTL_VERSION))
	curl -Lo $(KUBECTL) https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$(SYSTEM_NAME)/$(SYSTEM_ARCH)/kubectl
	chmod +x $(KUBECTL)

$(KUSTOMIZE): $(call tool_version_file,$(KUSTOMIZE),$(KUSTOMIZE_VERSION))
	curl -L -o - \
		https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F$(KUSTOMIZE_VERSION)/kustomize_$(KUSTOMIZE_VERSION)_$(SYSTEM_NAME)_$(SYSTEM_ARCH).tar.gz | \
	tar zxvf - -C $(abspath $(TOOLS_BIN_DIR))
	touch $(KUSTOMIZE) && chmod +x $(KUSTOMIZE)

ifeq ($(strip $(shell go list -m 2>/dev/null)),github.com/gardener/gardener)
$(LOGCHECK): $(TOOLS_PKG_PATH)/logcheck/go.* $(shell find $(TOOLS_PKG_PATH)/logcheck -type f -name '*.go')
	cd $(TOOLS_PKG_PATH)/logcheck;GOTOOLCHAIN=auto CGO_ENABLED=1 go build -o $(abspath $(LOGCHECK)) -buildmode=plugin ./plugin
else
$(LOGCHECK): go.mod
	GOTOOLCHAIN=auto CGO_ENABLED=1 go build -o $(LOGCHECK) -buildmode=plugin github.com/gardener/gardener/hack/tools/logcheck/plugin
endif

$(MOCKGEN): $(call tool_version_file,$(MOCKGEN),$(MOCKGEN_VERSION))
	go build -o $(MOCKGEN) go.uber.org/mock/mockgen

$(OPENAPI_GEN): $(call tool_version_file,$(OPENAPI_GEN),$(OPENAPI_GEN_VERSION))
	go build -o $(OPENAPI_GEN) k8s.io/kube-openapi/cmd/openapi-gen

$(PROMTOOL): $(call tool_version_file,$(PROMTOOL),$(PROMTOOL_VERSION))
	@PROMTOOL_VERSION=$(PROMTOOL_VERSION) $(TOOLS_PKG_PATH)/install-promtool.sh

$(PROTOC): $(call tool_version_file,$(PROTOC),$(PROTOC_VERSION))
	@PROTOC_VERSION=$(PROTOC_VERSION) $(TOOLS_PKG_PATH)/install-protoc.sh

$(TYPOS): $(call tool_version_file,$(TYPOS),$(TYPOS_VERSION))
	@TYPOS_VERSION=$(TYPOS_VERSION) $(TOOLS_PKG_PATH)/install-typos.sh

$(PROTOC_GEN_GOGO): $(call tool_version_file,$(PROTOC_GEN_GOGO),$(CODE_GENERATOR_VERSION))
	go build -o $(PROTOC_GEN_GOGO) k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo

ifeq ($(strip $(shell go list -m 2>/dev/null)),github.com/gardener/gardener)
$(REPORT_COLLECTOR): $(TOOLS_PKG_PATH)/report-collector/*.go
	go build -o $(REPORT_COLLECTOR) $(TOOLS_PKG_PATH)/report-collector
else
$(REPORT_COLLECTOR): go.mod
	go build -o $(REPORT_COLLECTOR) github.com/gardener/gardener/hack/tools/report-collector
endif

ifeq ($(strip $(shell go list -m 2>/dev/null)),github.com/gardener/gardener)
$(OIDC_METADATA): $(TOOLS_PKG_PATH)/oidcmeta/*.go
	go build -o $(OIDC_METADATA) $(TOOLS_PKG_PATH)/oidcmeta
else
$(OIDC_METADATA): go.mod
	go build -o $(OIDC_METADATA) github.com/gardener/gardener/hack/tools/oidcmeta
endif

ifeq ($(strip $(shell go list -m 2>/dev/null)),github.com/gardener/gardener)
$(EXTENSION_GEN): $(TOOLS_PKG_PATH)/extension-generator/*.go
	go build -o $(EXTENSION_GEN) $(TOOLS_PKG_PATH)/extension-generator
else
$(EXTENSION_GEN): go.mod
	go build -o $(EXTENSION_GEN) github.com/gardener/gardener/hack/tools/extension-generator
endif

$(SETUP_ENVTEST): $(call tool_version_file,$(SETUP_ENVTEST),$(CONTROLLER_RUNTIME_VERSION))
	curl -Lo $(SETUP_ENVTEST) https://github.com/kubernetes-sigs/controller-runtime/releases/download/$(CONTROLLER_RUNTIME_VERSION)/setup-envtest-$(SYSTEM_NAME)-$(SYSTEM_ARCH)
	chmod +x $(SETUP_ENVTEST)

$(SKAFFOLD): $(call tool_version_file,$(SKAFFOLD),$(SKAFFOLD_VERSION))
	curl -Lo $(SKAFFOLD) https://storage.googleapis.com/skaffold/releases/$(SKAFFOLD_VERSION)/skaffold-$(SYSTEM_NAME)-$(SYSTEM_ARCH)
	chmod +x $(SKAFFOLD)

$(YQ): $(call tool_version_file,$(YQ),$(YQ_VERSION))
	curl -L -o $(YQ) https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_$(SYSTEM_NAME)_$(SYSTEM_ARCH)
	chmod +x $(YQ)

$(VGOPATH): $(call tool_version_file,$(VGOPATH),$(VGOPATH_VERSION))
	go build -o $(VGOPATH) github.com/ironcore-dev/vgopath
