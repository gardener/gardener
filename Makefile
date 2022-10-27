# Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

REGISTRY                                   := eu.gcr.io/gardener-project/gardener
APISERVER_IMAGE_REPOSITORY                 := $(REGISTRY)/apiserver
CONTROLLER_MANAGER_IMAGE_REPOSITORY        := $(REGISTRY)/controller-manager
SCHEDULER_IMAGE_REPOSITORY                 := $(REGISTRY)/scheduler
ADMISSION_IMAGE_REPOSITORY                 := $(REGISTRY)/admission-controller
SEED_ADMISSION_IMAGE_REPOSITORY            := $(REGISTRY)/seed-admission-controller
RESOURCE_MANAGER_IMAGE_REPOSITORY          := $(REGISTRY)/resource-manager
GARDENLET_IMAGE_REPOSITORY                 := $(REGISTRY)/gardenlet
EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY  := $(REGISTRY)/extensions/provider-local
PUSH_LATEST_TAG                            := false
VERSION                                    := $(shell cat VERSION)
EFFECTIVE_VERSION                          := $(VERSION)-$(shell git rev-parse HEAD)
REPO_ROOT                                  := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
GARDENER_LOCAL_KUBECONFIG                  := $(REPO_ROOT)/example/gardener-local/kind/kubeconfig
GARDENER_LOCAL_HA_KUBECONFIG               := $(REPO_ROOT)/example/gardener-local/kind-ha/kubeconfig
GARDENER_LOCAL2_KUBECONFIG                 := $(REPO_ROOT)/example/gardener-local/kind2/kubeconfig
LOCAL_GARDEN_LABEL                         := local-garden
REMOTE_GARDEN_LABEL                        := remote-garden
ACTIVATE_SEEDAUTHORIZER                    := false
SEED_NAME                                  := ""
DEV_SETUP_WITH_WEBHOOKS                    := false
KIND_ENV                                   := "skaffold"
PARALLEL_E2E_TESTS                         := 5

ifneq ($(strip $(shell git status --porcelain 2>/dev/null)),)
	EFFECTIVE_VERSION := $(EFFECTIVE_VERSION)-dirty
endif

SHELL=/usr/bin/env bash -o pipefail

#########################################
# Tools                                 #
#########################################

TOOLS_DIR := hack/tools
include hack/tools.mk

LOGCHECK_DIR := $(TOOLS_DIR)/logcheck
GOMEGACHECK_DIR := $(TOOLS_DIR)/gomegacheck

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: dev-setup
dev-setup:
	@if [ "$(DEV_SETUP_WITH_WEBHOOKS)" = "true" ]; then ./hack/local-development/dev-setup --with-webhooks; else ./hack/local-development/dev-setup; fi

.PHONY: dev-setup-register-gardener
dev-setup-register-gardener:
	@./hack/local-development/dev-setup-register-gardener

.PHONY: local-garden-up
local-garden-up: $(HELM)
	@./hack/local-development/local-garden/start.sh $(LOCAL_GARDEN_LABEL) $(ACTIVATE_SEEDAUTHORIZER)

.PHONY: local-garden-down
local-garden-down:
	@./hack/local-development/local-garden/stop.sh $(LOCAL_GARDEN_LABEL)

.PHONY: remote-garden-up
remote-garden-up: $(HELM)
	@./hack/local-development/remote-garden/start.sh $(REMOTE_GARDEN_LABEL)

.PHONY: remote-garden-down
remote-garden-down:
	@./hack/local-development/remote-garden/stop.sh $(REMOTE_GARDEN_LABEL)

.PHONY: start-apiserver
start-apiserver:
	@./hack/local-development/start-apiserver

.PHONY: start-controller-manager
start-controller-manager:
	@./hack/local-development/start-controller-manager

.PHONY: start-scheduler
start-scheduler:
	@./hack/local-development/start-scheduler

.PHONY: start-admission-controller
start-admission-controller:
	@./hack/local-development/start-admission-controller

.PHONY: start-seed-admission-controller
start-seed-admission-controller:
	@./hack/local-development/start-seed-admission-controller

.PHONY: start-resource-manager
start-resource-manager:
	@./hack/local-development/start-resource-manager

.PHONY: start-gardenlet
start-gardenlet: $(HELM) $(YAML2JSON) $(YQ)
	@./hack/local-development/start-gardenlet

.PHONY: start-extension-provider-local
start-extension-provider-local:
	@./hack/local-development/start-extension-provider-local


#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: install
install:
	@EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) ./hack/install.sh ./...

.PHONY: docker-images
docker-images:
	@echo "Building docker images with version and tag $(EFFECTIVE_VERSION)"
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(APISERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(APISERVER_IMAGE_REPOSITORY):latest                -f Dockerfile --target apiserver .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)       -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):latest       -f Dockerfile --target controller-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(SCHEDULER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(SCHEDULER_IMAGE_REPOSITORY):latest                -f Dockerfile --target scheduler .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(ADMISSION_IMAGE_REPOSITORY):latest                -f Dockerfile --target admission-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(SEED_ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)           -t $(SEED_ADMISSION_IMAGE_REPOSITORY):latest           -f Dockerfile --target seed-admission-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(RESOURCE_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)         -t $(RESOURCE_MANAGER_IMAGE_REPOSITORY):latest         -f Dockerfile --target resource-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(GARDENLET_IMAGE_REPOSITORY):latest                -f Dockerfile --target gardenlet .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION) -t $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):latest -f Dockerfile --target gardener-extension-provider-local .

.PHONY: docker-images-ppc
docker-images-ppc:
	@echo "Building docker images for IBM's POWER(ppc64le) with version and tag $(EFFECTIVE_VERSION)"
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(APISERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(APISERVER_IMAGE_REPOSITORY):latest                -f Dockerfile --target apiserver .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)       -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):latest       -f Dockerfile --target controller-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(SCHEDULER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(SCHEDULER_IMAGE_REPOSITORY):latest                -f Dockerfile --target scheduler .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(ADMISSION_IMAGE_REPOSITORY):latest                -f Dockerfile --target admission-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(SEED_ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)           -t $(SEED_ADMISSION_IMAGE_REPOSITORY):latest           -f Dockerfile --target seed-admission-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(RESOURCE_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)         -t $(RESOURCE_MANAGER_IMAGE_REPOSITORY):latest         -f Dockerfile --target resource-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)                -t $(GARDENLET_IMAGE_REPOSITORY):latest                -f Dockerfile --target gardenlet .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION) -t $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):latest -f Dockerfile --target gardener-extension-provider-local .

.PHONY: docker-push
docker-push:
	@if ! docker images $(APISERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(APISERVER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(CONTROLLER_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(CONTROLLER_MANAGER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(SCHEDULER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(SCHEDULER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(ADMISSION_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(ADMISSION_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(SEED_ADMISSION_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(SEED_ADMISSION_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(RESOURCE_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(RESOURCE_MANAGER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
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
	@docker push $(SEED_ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(SEED_ADMISSION_IMAGE_REPOSITORY):latest; fi
	@docker push $(RESOURCE_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(RESOURCE_MANAGER_IMAGE_REPOSITORY):latest; fi
	@docker push $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(GARDENLET_IMAGE_REPOSITORY):latest; fi
	@docker push $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):latest; fi

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod tidy
	@GO111MODULE=on go mod vendor
	@cd $(LOGCHECK_DIR); go mod tidy
	@cd $(GOMEGACHECK_DIR); go mod tidy

.PHONY: clean
clean:
	@hack/clean.sh ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...

.PHONY: check-generate
check-generate:
	@hack/check-generate.sh $(REPO_ROOT)

.PHONY: check
check: $(GOIMPORTS) $(GOLANGCI_LINT) $(HELM) $(IMPORT_BOSS) $(LOGCHECK) $(GOMEGACHECK) $(YQ)
	@hack/check.sh --golangci-lint-config=./.golangci.yaml ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...
	@hack/check-imports.sh ./charts/... ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/... ./third_party/...

	@echo "> Check $(LOGCHECK_DIR)"
	@cd $(LOGCHECK_DIR); $(abspath $(GOLANGCI_LINT)) run -c $(REPO_ROOT)/.golangci.yaml --timeout 10m ./...
	@cd $(LOGCHECK_DIR); go vet ./...
	@cd $(LOGCHECK_DIR); $(abspath $(GOIMPORTS)) -l .

	@echo "> Check $(GOMEGACHECK_DIR)"
	@cd $(GOMEGACHECK_DIR); $(abspath $(GOLANGCI_LINT)) run -c $(REPO_ROOT)/.golangci.yaml --timeout 10m ./...
	@cd $(GOMEGACHECK_DIR); go vet ./...
	@cd $(GOMEGACHECK_DIR); $(abspath $(GOIMPORTS)) -l .

	@hack/check-charts.sh ./charts
	@hack/check-skaffold-deps.sh

.PHONY: generate
generate: $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GOIMPORTS) $(GO_TO_PROTOBUF) $(HELM) $(MOCKGEN) $(OPENAPI_GEN) $(PROTOC_GEN_GOGO) $(YAML2JSON)
	@hack/update-protobuf.sh
	@hack/update-codegen.sh
	@hack/generate-parallel.sh charts cmd example extensions pkg plugin test
	@cd $(LOGCHECK_DIR); go generate ./...
	@cd $(GOMEGACHECK_DIR); go generate ./...
	@hack/generate-monitoring-docs.sh
	$(MAKE) format

.PHONY: generate-sequential
generate-sequential: $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GOIMPORTS) $(GO_TO_PROTOBUF) $(HELM) $(MOCKGEN) $(OPENAPI_GEN) $(PROTOC_GEN_GOGO) $(YAML2JSON)
	@hack/update-protobuf.sh
	@hack/update-codegen.sh
	@hack/generate.sh ./charts/... ./cmd/... ./example/... ./extensions/... ./pkg/... ./plugin/... ./test/...
	@cd $(LOGCHECK_DIR); go generate ./...
	@cd $(GOMEGACHECK_DIR); go generate ./...
	@hack/generate-monitoring-docs.sh
	$(MAKE) format

.PHONY: format
format: $(GOIMPORTS)
	@./hack/format.sh ./cmd ./extensions ./pkg ./plugin ./test ./hack
	@cd $(LOGCHECK_DIR); $(abspath $(GOIMPORTS)) -l -w .
	@cd $(GOMEGACHECK_DIR); $(abspath $(GOIMPORTS)) -l -w .

.PHONY: test
test: $(REPORT_COLLECTOR) $(PROMTOOL)
	@./hack/test.sh ./cmd/... ./extensions/pkg/... ./pkg/... ./plugin/...
	@cd $(LOGCHECK_DIR); go test -race -timeout=2m ./... | grep -v 'no test files'
	@cd $(GOMEGACHECK_DIR); go test -race -timeout=2m ./... | grep -v 'no test files'

.PHONY: test-integration
test-integration: $(REPORT_COLLECTOR) $(SETUP_ENVTEST)
	@./hack/test-integration.sh ./test/integration/...

.PHONY: test-cov
test-cov: $(PROMTOOL)
	@./hack/test-cover.sh ./cmd/... ./extensions/pkg/... ./pkg/... ./plugin/...

.PHONY: test-cov-clean
test-cov-clean:
	@./hack/test-cover-clean.sh

.PHONY: check-apidiff
check-apidiff: $(GO_APIDIFF)
	@./hack/check-apidiff.sh

.PHONY: check-vulnerabilities
check-vulnerabilities: $(GO_VULN_CHECK)
	$(GO_VULN_CHECK) ./...

.PHONY: test-prometheus
test-prometheus: $(PROMTOOL)
	@./hack/test-prometheus.sh

.PHONY: check-docforge
check-docforge: $(DOCFORGE)
	@./hack/check-docforge.sh

.PHONY: verify
verify: check format test test-integration test-prometheus

.PHONY: verify-extended
verify-extended: check-generate check format test-cov test-cov-clean test-integration test-prometheus

#####################################################################
# Rules for local environment                                       #
#####################################################################

kind-up kind-down gardener-up gardener-down register-local-env tear-down-local-env register-kind2-env tear-down-kind2-env test-e2e-local-simple test-e2e-local-migration test-e2e-local: export KUBECONFIG = $(GARDENER_LOCAL_KUBECONFIG)

kind2-up kind2-down gardenlet-kind2-up gardenlet-kind2-down: export KUBECONFIG = $(GARDENER_LOCAL2_KUBECONFIG)

kind-ha-up kind-ha-down gardener-ha-up register-kind-ha-single-zone-env tear-down-kind-ha-single-zone-env register-kind-ha-multi-zone-env tear-down-kind-ha-multi-zone-env ci-e2e-kind-ha-single-zone ci-e2e-kind-ha-multi-zone: export KUBECONFIG = $(GARDENER_LOCAL_HA_KUBECONFIG)

kind-up: $(KIND) $(KUBECTL)
	mkdir -m 775 -p $(REPO_ROOT)/dev/local-backupbuckets $(REPO_ROOT)/dev/local-registry
	$(KIND) create cluster --name gardener-local --config $(REPO_ROOT)/example/gardener-local/kind/cluster-$(KIND_ENV).yaml --kubeconfig $(KUBECONFIG)
	docker exec gardener-local-control-plane sh -c "sysctl fs.inotify.max_user_instances=8192" # workaround https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
	cp $(KUBECONFIG) $(REPO_ROOT)/example/provider-local/seed-kind/base/kubeconfig
	$(KUBECTL) apply -k $(REPO_ROOT)/example/gardener-local/registry --server-side
	$(KUBECTL) wait --for=condition=available deployment -l app=registry -n registry --timeout 5m
	$(KUBECTL) apply -k $(REPO_ROOT)/example/gardener-local/calico --server-side
	$(KUBECTL) apply -k $(REPO_ROOT)/example/gardener-local/metrics-server --server-side

kind-ha-up: $(KIND) $(KUBECTL)
	mkdir -m 775 -p $(REPO_ROOT)/dev/local-backupbuckets $(REPO_ROOT)/dev/local-registry
	$(KIND) create cluster --name gardener-local-ha --config $(REPO_ROOT)/example/gardener-local/kind-ha/cluster-$(KIND_ENV).yaml --kubeconfig $(KUBECONFIG)
	docker exec gardener-local-ha-control-plane sh -c "sysctl fs.inotify.max_user_instances=8192" # workaround https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
	docker exec gardener-local-ha-worker sh -c "sysctl fs.inotify.max_user_instances=8192" # workaround https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
	docker exec gardener-local-ha-worker2 sh -c "sysctl fs.inotify.max_user_instances=8192" # workaround https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
	cp $(KUBECONFIG) $(REPO_ROOT)/example/provider-local/seed-kind-ha/base/kubeconfig
	$(KUBECTL) taint node gardener-local-ha-control-plane node-role.kubernetes.io/master:NoSchedule-
	$(KUBECTL) taint node gardener-local-ha-control-plane node-role.kubernetes.io/control-plane:NoSchedule-
	$(KUBECTL) apply -k $(REPO_ROOT)/example/gardener-local/registry --server-side
	$(KUBECTL) wait --for=condition=available deployment -l app=registry -n registry --timeout 5m
	$(KUBECTL) apply -k $(REPO_ROOT)/example/gardener-local/calico --server-side
	$(KUBECTL) apply -k $(REPO_ROOT)/example/gardener-local/metrics-server --server-side

kind2-up: $(KIND) $(KUBECTL)
	$(KIND) create cluster --name gardener-local2 --config $(REPO_ROOT)/example/gardener-local/kind2/cluster-$(KIND_ENV).yaml --kubeconfig $(KUBECONFIG)
	docker exec gardener-local2-control-plane sh -c "sysctl fs.inotify.max_user_instances=8192" # workaround https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
	cp $(KUBECONFIG) $(REPO_ROOT)/example/provider-local/seed-kind2/base/kubeconfig
	$(KUBECTL) apply -k $(REPO_ROOT)/example/gardener-local/calico --server-side
	$(KUBECTL) apply -k $(REPO_ROOT)/example/gardener-local/metrics-server --server-side

kind-down: $(KIND)
	$(KIND) delete cluster --name gardener-local
	rm -f $(REPO_ROOT)/example/provider-local/seed-kind/base/kubeconfig
	rm -rf dev/local-backupbuckets

kind2-down: $(KIND)
	$(KIND) delete cluster --name gardener-local2
	rm -f $(REPO_ROOT)/example/provider-local/seed-kind2/base/kubeconfig

kind-ha-down: $(KIND)
	$(KIND) delete cluster --name gardener-local-ha
	rm -f $(REPO_ROOT)/example/provider-local/seed-kind-ha/base/kubeconfig
	rm -rf dev/local-backupbuckets

# speed-up skaffold deployments by building all images concurrently
export SKAFFOLD_BUILD_CONCURRENCY = 0
# use static label for skaffold to prevent rolling all gardener components on every `skaffold` invocation
gardener-up gardener-down gardener-ha-up gardener-ha-down gardenlet-kind2-up gardenlet-kind2-down: export SKAFFOLD_LABEL = skaffold.dev/run-id=gardener-local

# set ldflags for skaffold
gardener-up gardener-ha-up gardenlet-kind2-up: export LD_FLAGS = $(shell $(REPO_ROOT)/hack/get-build-ld-flags.sh)

gardener-up: $(SKAFFOLD) $(HELM) $(KUBECTL)
	SKAFFOLD_DEFAULT_REPO=localhost:5001 SKAFFOLD_PUSH=true $(SKAFFOLD) run

gardener-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	@# delete stuff gradually in the right order, otherwise several dependencies will prevent the cleanup from succeeding
	$(KUBECTL) delete seed local --ignore-not-found --wait --timeout 5m
	$(KUBECTL) delete seed local2 --ignore-not-found --wait --timeout 5m
	$(SKAFFOLD) delete -m provider-local,gardenlet
	$(KUBECTL) delete validatingwebhookconfiguration/gardener-admission-controller --ignore-not-found
	$(KUBECTL) annotate project local garden confirmation.gardener.cloud/deletion=true
	$(SKAFFOLD) delete -m local-env
	$(SKAFFOLD) delete -m etcd,controlplane
	@# workaround for https://github.com/gardener/gardener/issues/5164
	$(KUBECTL) delete ns seed-local --ignore-not-found
	@# cleanup namespaces that don't get deleted automatically
	$(KUBECTL) delete ns gardener-system-seed-lease istio-ingress istio-system --ignore-not-found

gardener-ha-up: $(SKAFFOLD) $(HELM) $(KUBECTL)
	SKAFFOLD_DEFAULT_REPO=localhost:5001 SKAFFOLD_PUSH=true $(SKAFFOLD) run -p ha

gardener-ha-down: $(SKAFFOLD) $(HELM) $(KUBECTL)
	@# delete stuff gradually in the right order, otherwise several dependencies will prevent the cleanup from succeeding
	$(KUBECTL) delete seed local-ha --ignore-not-found --wait --timeout 5m
	$(SKAFFOLD) delete -m provider-local,gardenlet -p ha
	$(KUBECTL) delete validatingwebhookconfiguration/gardener-admission-controller --ignore-not-found
	$(KUBECTL) annotate project local garden confirmation.gardener.cloud/deletion=true
	$(SKAFFOLD) delete -m local-env -p ha
	$(SKAFFOLD) delete -m etcd,controlplane -p ha
	@# workaround for https://github.com/gardener/gardener/issues/5164
	$(KUBECTL) delete ns seed-local-ha --ignore-not-found
	@# cleanup namespaces that don't get deleted automatically
	$(KUBECTL) delete ns gardener-system-seed-lease istio-ingress istio-system --ignore-not-found

gardenlet-kind2-up: $(SKAFFOLD) $(HELM)
	$(SKAFFOLD) deploy -m kind2-env -p kind2 --kubeconfig=$(GARDENER_LOCAL_KUBECONFIG)
	@# define GARDENER_LOCAL_KUBECONFIG so that it can be used by skaffold when checking whether the seed managed by this gardenlet is ready
	GARDENER_LOCAL_KUBECONFIG=$(GARDENER_LOCAL_KUBECONFIG) SKAFFOLD_DEFAULT_REPO=localhost:5001 SKAFFOLD_PUSH=true $(SKAFFOLD) run -m provider-local,gardenlet -p kind2

gardenlet-kind2-down: $(SKAFFOLD) $(HELM)
	$(SKAFFOLD) delete -m kind2-env -p kind2 --kubeconfig=$(GARDENER_LOCAL_KUBECONFIG)
	$(SKAFFOLD) delete -m gardenlet,kind2-env -p kind2

register-local-env: $(KUBECTL)
	$(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/garden/local
	$(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/seed-kind/local

tear-down-local-env: $(KUBECTL)
	$(KUBECTL) annotate project local confirmation.gardener.cloud/deletion=true
	$(KUBECTL) delete -k $(REPO_ROOT)/example/provider-local/seed-kind/local
	$(KUBECTL) delete -k $(REPO_ROOT)/example/provider-local/garden/local

register-kind2-env: $(KUBECTL)
	$(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/seed-kind2/local

tear-down-kind2-env: $(KUBECTL)
	$(KUBECTL) delete -k $(REPO_ROOT)/example/provider-local/seed-kind2/local

register-kind-ha-single-zone-env: $(KUBECTL)
	$(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/garden/local
	$(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/seed-kind-ha/local-single-zone

tear-down-kind-ha-single-zone-env: $(KUBECTL)
	$(KUBECTL) annotate project local confirmation.gardener.cloud/deletion=true
	$(KUBECTL) delete -k $(REPO_ROOT)/example/provider-local/seed-kind-ha/local-single-zone
	$(KUBECTL) delete -k $(REPO_ROOT)/example/provider-local/garden/local

register-kind-ha-multi-zone-env: $(KUBECTL)
	$(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/garden/local
	$(KUBECTL) apply -k $(REPO_ROOT)/example/provider-local/seed-kind-ha/local-multi-zone

tear-down-kind-ha-multi-zone-env: $(KUBECTL)
	$(KUBECTL) annotate project local confirmation.gardener.cloud/deletion=true
	$(KUBECTL) delete -k $(REPO_ROOT)/example/provider-local/seed-kind-ha/local-multi-zone
	$(KUBECTL) delete -k $(REPO_ROOT)/example/provider-local/garden/local

test-e2e-local-simple: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && simple"

test-e2e-local-migration: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter "Shoot && control-plane-migration"

test-e2e-local: $(GINKGO)
	./hack/test-e2e-local.sh --procs=$(PARALLEL_E2E_TESTS) --label-filter="default"

ci-e2e-kind: $(KIND) $(YQ)
	./hack/ci-e2e-kind.sh

ci-e2e-kind-ha-node: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE=node ./hack/ci-e2e-kind-ha.sh

ci-e2e-kind-ha-zone: $(KIND) $(YQ)
	SHOOT_FAILURE_TOLERANCE_TYPE=zone ./hack/ci-e2e-kind-ha.sh

ci-e2e-kind-migration: $(KIND) $(YQ)
	./hack/ci-e2e-kind-migration.sh
