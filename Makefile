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
LANDSCAPER_GARDENLET_IMAGE_REPOSITORY      := $(REGISTRY)/landscaper-gardenlet
LANDSCAPER_CONTROL_PLANE_IMAGE_REPOSITORY  := $(REGISTRY)/landscaper-control-plane
EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY  := $(REGISTRY)/extensions/provider-local
PUSH_LATEST_TAG                            := false
VERSION                                    := $(shell cat VERSION)
EFFECTIVE_VERSION                          := $(VERSION)-$(shell git rev-parse HEAD)
REPO_ROOT                                  := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
LOCAL_GARDEN_LABEL                         := local-garden
REMOTE_GARDEN_LABEL                        := remote-garden
ACTIVATE_SEEDAUTHORIZER                    := false
SEED_NAME                                  := ""
DEV_SETUP_WITH_WEBHOOKS                    := false
KIND_ENV                                   := "skaffold"

ifneq ($(strip $(shell git status --porcelain 2>/dev/null)),)
	EFFECTIVE_VERSION := $(EFFECTIVE_VERSION)-dirty
endif

#########################################
# Tools                                 #
#########################################

TOOLS_DIR := hack/tools
include hack/tools.mk

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: dev-setup
dev-setup:
	@if [[ "$(DEV_SETUP_WITH_WEBHOOKS)" == "true" ]]; then ./hack/local-development/dev-setup --with-webhooks; else ./hack/local-development/dev-setup; fi

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

.PHONY: start-landscaper-gardenlet
start-landscaper-gardenlet:
	@./hack/local-development/start-landscaper-gardenlet $(OPERATION)

.PHONY: start-landscaper-controlplane
start-landscaper-controlplane:
	@./hack/local-development/start-landscaper-controlplane $(OPERATION)

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
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(LANDSCAPER_GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)     -t $(LANDSCAPER_GARDENLET_IMAGE_REPOSITORY):latest     -f Dockerfile --target landscaper-gardenlet .
    @docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(LANDSCAPER_CONTROL_PLANE_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)  -t $(LANDSCAPER_CONTROL_PLANE_IMAGE_REPOSITORY):latest -f Dockerfile --target landscaper-controlplane .
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
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(LANDSCAPER_GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)     -t $(LANDSCAPER_GARDENLET_IMAGE_REPOSITORY):latest     -f Dockerfile --target landscaper-gardenlet .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(LANDSCAPER_CONTROL_PLANE_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)  -t $(LANDSCAPER_CONTROL_PLANE_IMAGE_REPOSITORY):latest -f Dockerfile --target landscaper-controlplane .
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
	@if ! docker images $(LANDSCAPER_GARDENLET_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(LANDSCAPER_GARDENLET_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(LANDSCAPER_CONTROL_PLANE_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(LANDSCAPER_CONTROL_PLANE_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
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
	@docker push $(LANDSCAPER_GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(LANDSCAPER_GARDENLET_IMAGE_REPOSITORY):latest; fi
	@docker push $(LANDSCAPER_CONTROL_PLANE_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
    @if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(LANDSCAPER_CONTROL_PLANE_IMAGE_REPOSITORY):latest; fi
	@docker push $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then docker push $(EXTENSION_PROVIDER_LOCAL_IMAGE_REPOSITORY):latest; fi

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: install-requirements
install-requirements:
	@./hack/install-requirements.sh

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod vendor
	@GO111MODULE=on go mod tidy

.PHONY: clean
clean:
	@hack/clean.sh ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/... ./landscaper/...

.PHONY: check-generate
check-generate:
	@hack/check-generate.sh $(REPO_ROOT)

.PHONY: check
check: $(GOIMPORTS) $(GOLANGCI_LINT) $(HELM)
	@hack/check.sh --golangci-lint-config=./.golangci.yaml ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...
	@hack/check-charts.sh ./charts

.PHONY: generate
generate: $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GOIMPORTS) $(GO_TO_PROTOBUF) $(HELM) $(MOCKGEN) $(OPENAPI_GEN) $(PROTOC_GEN_GOGO) $(YAML2JSON)
	@hack/update-protobuf.sh
	@hack/update-codegen.sh
	@hack/generate-parallel.sh charts cmd example extensions pkg plugin landscaper test
	@hack/generate-monitoring-docs.sh

.PHONY: generate-sequential
generate-sequential: $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS) $(GOIMPORTS) $(GO_TO_PROTOBUF) $(HELM) $(MOCKGEN) $(OPENAPI_GEN) $(PROTOC_GEN_GOGO) $(YAML2JSON)
	@hack/update-protobuf.sh
	@hack/update-codegen.sh
	@hack/generate.sh ./charts/... ./cmd/... ./example/... ./extensions/... ./pkg/... ./plugin/... ./landscaper/... ./test/...
	@hack/generate-monitoring-docs.sh

.PHONY: format
format: $(GOIMPORTS)
	@./hack/format.sh ./cmd ./extensions ./pkg ./plugin ./test ./landscaper ./hack

.PHONY: test
test: $(PROMTOOL)
	@./hack/test.sh ./cmd/... ./extensions/pkg/... ./pkg/... ./plugin/... ./landscaper/...

.PHONY: test-integration
test-integration: $(SETUP_ENVTEST)
	@./hack/test-integration.sh ./extensions/test/integration/envtest/... ./test/integration/envtest/...

.PHONY: test-cov
test-cov: $(PROMTOOL)
	@./hack/test-cover.sh ./cmd/... ./extensions/pkg/... ./pkg/... ./plugin/... ./landscaper/...

.PHONY: test-cov-clean
test-cov-clean:
	@./hack/test-cover-clean.sh

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

kind-up:
	kind create cluster --name gardener-local --config $(REPO_ROOT)/example/gardener-local/kind/cluster-$(KIND_ENV).yaml --kubeconfig $(REPO_ROOT)/example/gardener-local/kind/kubeconfig
	docker exec gardener-local-control-plane sh -c "sysctl fs.inotify.max_user_instances=8192" # workaround https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files
	cp $(REPO_ROOT)/example/gardener-local/kind/kubeconfig $(REPO_ROOT)/example/provider-local/base/kubeconfig

kind-down:
	kind delete cluster --name gardener-local
	rm -f $(REPO_ROOT)/example/provider-local/base/kubeconfig

# workaround for https://github.com/GoogleContainerTools/skaffold/issues/6416
export SKAFFOLD_LABEL := skaffold.dev/run-id=gardener-local
gardener-up: $(SKAFFOLD) $(HELM)
	$(SKAFFOLD) run

gardener-down: $(SKAFFOLD) $(HELM)
	@# delete stuff gradually in the right order, otherwise several dependencies will prevent the cleanup from succeeding
	kubectl delete seed local --ignore-not-found --wait --timeout 5m
	$(SKAFFOLD) delete -m provider-local,gardenlet
	kubectl delete validatingwebhookconfiguration/validate-namespace-deletion --ignore-not-found
	kubectl annotate project local confirmation.gardener.cloud/deletion=true
	$(SKAFFOLD) delete -m local-env
	$(SKAFFOLD) delete -m etcd,controlplane
	@# workaround for https://github.com/gardener/gardener/issues/5164
	kubectl delete ns seed-local --ignore-not-found
	@# cleanup namespaces that don't get deleted automatically
	kubectl delete ns gardener-system-seed-lease istio-ingress istio-system --ignore-not-found

register-local-env:
	kubectl apply -k $(REPO_ROOT)/example/provider-local/overlays/local

tear-down-local-env:
	kubectl annotate project local confirmation.gardener.cloud/deletion=true
	kubectl delete -k $(REPO_ROOT)/example/provider-local/overlays/local

test-e2e-local:
	./hack/test-e2e-local.sh
