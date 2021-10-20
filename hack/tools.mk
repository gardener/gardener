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
# as needed. If not built/installed yet, make will then make sure to build/install the required tools.

HACK_PKG_PATH              := $(shell go list -tags tools -f '{{ .Dir }}' github.com/gardener/gardener/hack)
TOOLS_BIN_DIR              := $(TOOLS_DIR)/bin
CONTROLLER_GEN             := $(TOOLS_BIN_DIR)/controller-gen
GOIMPORTS                  := $(TOOLS_BIN_DIR)/goimports
GOLANGCI_LINT              := $(TOOLS_BIN_DIR)/golangci-lint
GEN_CRD_API_REFERENCE_DOCS := $(TOOLS_BIN_DIR)/gen-crd-api-reference-docs
HELM                       := $(TOOLS_BIN_DIR)/helm
MOCKGEN                    := $(TOOLS_BIN_DIR)/mockgen
OPENAPI_GEN                := $(TOOLS_BIN_DIR)/openapi-gen
PROMTOOL                   := $(TOOLS_BIN_DIR)/promtool
SETUP_ENVTEST              := $(TOOLS_BIN_DIR)/setup-envtest
YAML2JSON                  := $(TOOLS_BIN_DIR)/yaml2json
YQ                         := $(TOOLS_BIN_DIR)/yq

export TOOLS_BIN_DIR := $(TOOLS_BIN_DIR)
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

$(CONTROLLER_GEN): go.mod
	go build -o $(CONTROLLER_GEN) sigs.k8s.io/controller-tools/cmd/controller-gen

$(GOIMPORTS): go.mod
	go build -o $(GOIMPORTS) golang.org/x/tools/cmd/goimports

$(GOLANGCI_LINT):
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TOOLS_BIN_DIR) v1.42.1

$(GEN_CRD_API_REFERENCE_DOCS): go.mod
	go build -o $(GEN_CRD_API_REFERENCE_DOCS) github.com/ahmetb/gen-crd-api-reference-docs

$(HELM):
	curl -sSfL https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | HELM_INSTALL_DIR=$(TOOLS_BIN_DIR) USE_SUDO=false bash -s -- --version 'v3.5.4'

$(MOCKGEN): go.mod
	go build -o $(MOCKGEN) github.com/golang/mock/mockgen

$(OPENAPI_GEN): go.mod
	go build -o $(OPENAPI_GEN) k8s.io/kube-openapi/cmd/openapi-gen

$(PROMTOOL): $(HACK_PKG_PATH)/tools/install-promtool.sh
	@$(HACK_PKG_PATH)/tools/install-promtool.sh

$(SETUP_ENVTEST): go.mod
	go build -o $(SETUP_ENVTEST) sigs.k8s.io/controller-runtime/tools/setup-envtest

$(YAML2JSON): go.mod
	go build -o $(YAML2JSON) github.com/bronze1man/yaml2json

$(YQ):
	curl -L -o "$(YQ)" https://github.com/mikefarah/yq/releases/download/v4.9.6/yq_$(shell uname -s | tr '[:upper:]' '[:lower:]')_$(shell uname -m | sed 's/x86_64/amd64/')
	chmod +x "$(YQ)"

.PHONY: clean-tools-bin
clean-tools-bin:
	rm -rf $(TOOLS_BIN_DIR)/*
