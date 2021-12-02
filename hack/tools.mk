# Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This make file is supposed to be included in the top-level make file.
# It can be reused by repos vendoring g/g to have some common make recipes for building and installing development
# tools as needed.
# Recipes in the top-level make file should declare dependencies on the respective tool recipes (e.g. $(CONTROLLER_GEN))
# as needed. If the required tool (version) is not built/installed yet, make will make sure to build/install it.
# The *_VERSION variables in this file contain the "default" values, but can be overwritten in the top level make file.

HACK_PKG_PATH              := $(shell go list -tags tools -f '{{ .Dir }}' github.com/gardener/gardener/hack)
TOOLS_BIN_DIR              := $(TOOLS_DIR)/bin
CONTROLLER_GEN             := $(TOOLS_BIN_DIR)/controller-gen
DOCFORGE                   := $(TOOLS_BIN_DIR)/docforge
GEN_CRD_API_REFERENCE_DOCS := $(TOOLS_BIN_DIR)/gen-crd-api-reference-docs
GOIMPORTS                  := $(TOOLS_BIN_DIR)/goimports
GOLANGCI_LINT              := $(TOOLS_BIN_DIR)/golangci-lint
GO_TO_PROTOBUF             := $(TOOLS_BIN_DIR)/go-to-protobuf
HELM                       := $(TOOLS_BIN_DIR)/helm
MOCKGEN                    := $(TOOLS_BIN_DIR)/mockgen
OPENAPI_GEN                := $(TOOLS_BIN_DIR)/openapi-gen
PROMTOOL                   := $(TOOLS_BIN_DIR)/promtool
PROTOC_GEN_GOGO            := $(TOOLS_BIN_DIR)/protoc-gen-gogo
SETUP_ENVTEST              := $(TOOLS_BIN_DIR)/setup-envtest
YAML2JSON                  := $(TOOLS_BIN_DIR)/yaml2json
YQ                         := $(TOOLS_BIN_DIR)/yq

# default tool versions
GOLANGCI_LINT_VERSION ?= v1.42.1
HELM_VERSION ?= v3.5.4
YQ_VERSION ?= v4.9.6
DOCFORGE_VERSION ?= v0.21.0

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

# This target cleans up any previous version files for the given tool and creates the given version file.
# This way, we can generically determine, which version was installed without calling each and every binary explicitly.
$(TOOLS_BIN_DIR)/.version_%:
	@version_file=$@; rm -f $${version_file%_*}*
	@touch $@

.PHONY: clean-tools-bin
clean-tools-bin:
	rm -rf $(TOOLS_BIN_DIR)/*

#########################################
# Tools                                 #
#########################################

$(CONTROLLER_GEN): go.mod
	go build -o $(CONTROLLER_GEN) sigs.k8s.io/controller-tools/cmd/controller-gen

$(GEN_CRD_API_REFERENCE_DOCS): go.mod
	go build -o $(GEN_CRD_API_REFERENCE_DOCS) github.com/ahmetb/gen-crd-api-reference-docs

$(GOIMPORTS): go.mod
	go build -o $(GOIMPORTS) golang.org/x/tools/cmd/goimports

$(GOLANGCI_LINT): $(call tool_version_file,$(GOLANGCI_LINT),$(GOLANGCI_LINT_VERSION))
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TOOLS_BIN_DIR) $(GOLANGCI_LINT_VERSION)

$(GO_TO_PROTOBUF): go.mod
	go build -o $(GO_TO_PROTOBUF) k8s.io/code-generator/cmd/go-to-protobuf

$(HELM): $(call tool_version_file,$(HELM),$(HELM_VERSION))
	curl -sSfL https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | HELM_INSTALL_DIR=$(TOOLS_BIN_DIR) USE_SUDO=false bash -s -- --version $(HELM_VERSION)

$(MOCKGEN): go.mod
	go build -o $(MOCKGEN) github.com/golang/mock/mockgen

$(OPENAPI_GEN): go.mod
	go build -o $(OPENAPI_GEN) k8s.io/kube-openapi/cmd/openapi-gen

$(PROMTOOL): $(HACK_PKG_PATH)/tools/install-promtool.sh
	@$(HACK_PKG_PATH)/tools/install-promtool.sh

$(SETUP_ENVTEST): go.mod
	go build -o $(SETUP_ENVTEST) sigs.k8s.io/controller-runtime/tools/setup-envtest

$(PROTOC_GEN_GOGO): go.mod
	go build -o $(PROTOC_GEN_GOGO) k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo

$(YAML2JSON): go.mod
	go build -o $(YAML2JSON) github.com/bronze1man/yaml2json

$(YQ): $(call tool_version_file,$(YQ),$(YQ_VERSION))
	curl -L -o $(YQ) https://github.com/mikefarah/yq/releases/download/$(YQ_VERSION)/yq_$(shell uname -s | tr '[:upper:]' '[:lower:]')_$(shell uname -m | sed 's/x86_64/amd64/')
	chmod +x $(YQ)

$(DOCFORGE): $(call tool_version_file,$(DOCFORGE),$(DOCFORGE_VERSION))
	curl -L -o $(DOCFORGE) https://github.com/gardener/docforge/releases/download/$(DOCFORGE_VERSION)/docforge-$(shell uname -s | tr '[:upper:]' '[:lower:]')-$(shell uname -m | sed 's/x86_64/amd64/')
	chmod +x $(DOCFORGE)
