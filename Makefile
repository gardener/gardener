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

REGISTRY                            := eu.gcr.io/gardener-project/gardener
APISERVER_IMAGE_REPOSITORY          := $(REGISTRY)/apiserver
CONTROLLER_MANAGER_IMAGE_REPOSITORY := $(REGISTRY)/controller-manager
SCHEDULER_IMAGE_REPOSITORY          := $(REGISTRY)/scheduler
ADMISSION_IMAGE_REPOSITORY          := $(REGISTRY)/admission-controller
SEED_ADMISSION_IMAGE_REPOSITORY     := $(REGISTRY)/seed-admission-controller
GARDENLET_IMAGE_REPOSITORY          := $(REGISTRY)/gardenlet
PUSH_LATEST_TAG                     := false
VERSION                             := $(shell cat VERSION)
EFFECTIVE_VERSION                   := $(VERSION)-$(shell git rev-parse HEAD)
REPO_ROOT                           := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
LOCAL_GARDEN_LABEL                  := local-garden
REMOTE_GARDEN_LABEL                 := remote-garden
CR_VERSION                          := $(shell go mod edit -json | jq -r '.Require[] | select(.Path=="sigs.k8s.io/controller-runtime") | .Version')

ifneq ($(strip $(shell git status --porcelain 2>/dev/null)),)
	EFFECTIVE_VERSION := $(EFFECTIVE_VERSION)-dirty
endif

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: dev-setup
dev-setup:
	@./hack/local-development/dev-setup

.PHONY: dev-setup-register-gardener
dev-setup-register-gardener:
	@./hack/local-development/dev-setup-register-gardener

.PHONY: local-garden-up
local-garden-up:
	@./hack/local-development/local-garden/start.sh $(LOCAL_GARDEN_LABEL)

.PHONY: local-garden-down
local-garden-down:
	@./hack/local-development/local-garden/stop.sh $(LOCAL_GARDEN_LABEL)

.PHONY: remote-garden-up
remote-garden-up:
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

.PHONY: start-gardenlet
start-gardenlet:
	@./hack/local-development/start-gardenlet

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: install
install:
	@EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) ./hack/install.sh ./...

.PHONY: docker-images
docker-images:
	@echo "Building docker images with version and tag $(EFFECTIVE_VERSION)"
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(APISERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)          -t $(APISERVER_IMAGE_REPOSITORY):latest          -f Dockerfile --target apiserver .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION) -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):latest -f Dockerfile --target controller-manager .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(SCHEDULER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)          -t $(SCHEDULER_IMAGE_REPOSITORY):latest          -f Dockerfile --target scheduler .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)          -t $(ADMISSION_IMAGE_REPOSITORY):latest          -f Dockerfile --target admission-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(SEED_ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)     -t $(SEED_ADMISSION_IMAGE_REPOSITORY):latest     -f Dockerfile --target seed-admission-controller .
	@docker build --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION)  -t $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)          -t $(GARDENLET_IMAGE_REPOSITORY):latest          -f Dockerfile --target gardenlet .

.PHONY: docker-images-ppc
docker-images-ppc:
	@echo "Building docker images for IBM's POWER(ppc64le) with version and tag $(EFFECTIVE_VERSION)"
	@docker build --build-arg GCR_PULL_URL="" --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(APISERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)          -t $(APISERVER_IMAGE_REPOSITORY):latest          -f Dockerfile --target apiserver .
	@docker build --build-arg GCR_PULL_URL="" --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION) -t $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):latest -f Dockerfile --target controller-manager .
	@docker build --build-arg GCR_PULL_URL="" --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(SCHEDULER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)          -t $(SCHEDULER_IMAGE_REPOSITORY):latest          -f Dockerfile --target scheduler .
	@docker build --build-arg GCR_PULL_URL="" --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)          -t $(ADMISSION_IMAGE_REPOSITORY):latest          -f Dockerfile --target admission-controller .
	@docker build --build-arg GCR_PULL_URL="" --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(SEED_ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)     -t $(SEED_ADMISSION_IMAGE_REPOSITORY):latest     -f Dockerfile --target seed-admission-controller .
	@docker build --build-arg GCR_PULL_URL="" --build-arg EFFECTIVE_VERSION=$(EFFECTIVE_VERSION) -t $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)          -t $(GARDENLET_IMAGE_REPOSITORY):latest          -f Dockerfile --target gardenlet .

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-push
docker-push:
	@if ! docker images $(APISERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(APISERVER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(CONTROLLER_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(CONTROLLER_MANAGER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(SCHEDULER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(SCHEDULER_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(ADMISSION_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(ADMISSION_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(SEED_ADMISSION_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(SEED_ADMISSION_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(GARDENLET_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(EFFECTIVE_VERSION); then echo "$(GARDENLET_IMAGE_REPOSITORY) version $(EFFECTIVE_VERSION) is not yet built. Please run 'make docker-images'"; false; fi
	@gcloud docker -- push $(APISERVER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then gcloud docker -- push $(APISERVER_IMAGE_REPOSITORY):latest; fi
	@gcloud docker -- push $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then gcloud docker -- push $(CONTROLLER_MANAGER_IMAGE_REPOSITORY):latest; fi
	@gcloud docker -- push $(SCHEDULER_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then gcloud docker -- push $(SCHEDULER_IMAGE_REPOSITORY):latest; fi
	@gcloud docker -- push $(ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then gcloud docker -- push $(ADMISSION_IMAGE_REPOSITORY):latest; fi
	@gcloud docker -- push $(SEED_ADMISSION_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then gcloud docker -- push $(SEED_ADMISSION_IMAGE_REPOSITORY):latest; fi
	@gcloud docker -- push $(GARDENLET_IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)
	@if [[ "$(PUSH_LATEST_TAG)" == "true" ]]; then gcloud docker -- push $(GARDENLET_IMAGE_REPOSITORY):latest; fi

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: install-requirements
install-requirements:
	@go install -mod=vendor github.com/onsi/ginkgo/ginkgo
	@go install -mod=vendor github.com/ahmetb/gen-crd-api-reference-docs
	@go install -mod=vendor github.com/golang/mock/mockgen
	@go install -mod=vendor sigs.k8s.io/controller-tools/cmd/controller-gen
	@GO111MODULE=off go get github.com/go-bindata/go-bindata/...
	@./hack/install-promtool.sh
	@./hack/install-requirements.sh

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod vendor
	@GO111MODULE=on go mod tidy
	@curl -sSLo hack/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/$(CR_VERSION)/hack/setup-envtest.sh

.PHONY: clean
clean:
	@hack/clean.sh ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...

.PHONY: check-generate
check-generate:
	@hack/check-generate.sh $(REPO_ROOT)

.PHONY: check
check:
	@hack/check.sh --golangci-lint-config=./.golangci.yaml ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...
	@hack/check-charts.sh ./charts

# We need to explicitly pass GO111MODULE=off to k8s.io/code-generator as it is significantly slower otherwise,
# see https://github.com/kubernetes/code-generator/issues/100.
.PHONY: generate
generate:
	@GO111MODULE=off hack/update-protobuf.sh
	@GO111MODULE=off hack/update-codegen.sh --parallel
	@hack/generate-parallel.sh cmd extensions pkg plugin test

.PHONY: generate-sequential
generate-sequential:
	@GO111MODULE=off hack/update-protobuf.sh
	@GO111MODULE=off hack/update-codegen.sh 
	@hack/generate.sh ./cmd/... ./extensions/... ./pkg/... ./plugin/... ./test/...

.PHONY: generate-extensions-crds
generate-extensions-crds:
	@controller-gen crd paths=./pkg/apis/extensions/... output:crd:dir=./dev/extensions-crds output:stdout

.PHONY: format
format:
	@./hack/format.sh ./cmd ./extensions ./pkg ./plugin ./test

.PHONY: test
test:
	@./hack/test.sh ./cmd/... ./extensions/... ./pkg/... ./plugin/...
	$(MAKE) test-prometheus

.PHONY: test-cov
test-cov:
	@./hack/test-cover.sh ./cmd/... ./extensions/... ./pkg/... ./plugin/...
	$(MAKE) test-prometheus

.PHONY: test-cov-clean
test-cov-clean:
	@./hack/test-cover-clean.sh

.PHONY: test-prometheus
test-prometheus:
	@./hack/test-prometheus.sh

.PHONY: verify
verify: check format test

.PHONY: verify-extended
verify-extended: install-requirements check-generate check format test-cov test-cov-clean
