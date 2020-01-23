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

REGISTRY                           := eu.gcr.io/gardener-project/gardener
APISERVER_IMAGE_REPOSITORY         := $(REGISTRY)/apiserver
CONROLLER_MANAGER_IMAGE_REPOSITORY := $(REGISTRY)/controller-manager
SCHEDULER_IMAGE_REPOSITORY         := $(REGISTRY)/scheduler
GARDENLET_IMAGE_REPOSITORY         := $(REGISTRY)/gardenlet
IMAGE_TAG                          := $(shell cat VERSION)
WORKDIR                            := $(shell pwd)
PUSH_LATEST                        := true
LD_FLAGS                           := $(shell ./hack/get-build-ld-flags)
LOCAL_GARDEN_LABEL                 := local-garden

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: dev-setup
dev-setup:
	@./hack/dev-setup

.PHONY: dev-setup-extensions
dev-setup-extensions:
	@./hack/dev-setup-extensions

.PHONY: local-garden-up
local-garden-up:
	# Remove old containers and create the docker user network
	@-./hack/local-garden/cleanup
	@-docker network create gardener-dev --label $(LOCAL_GARDEN_LABEL)

    # Start the nodeless kubernetes environment
	@./hack/local-garden/run-kube-etcd $(LOCAL_GARDEN_LABEL)
	@./hack/local-garden/run-kube-apiserver $(LOCAL_GARDEN_LABEL)
	@./hack/local-garden/run-kube-controller-manager $(LOCAL_GARDEN_LABEL)

	# This etcd will be used to storge gardener resources (e.g., seeds, shoots)
	@./hack/local-garden/run-gardener-etcd $(LOCAL_GARDEN_LABEL)

	# Applying proxy RBAC for the extension controller
	# After this step, you can start using the cluster at hack/local-garden/kubeconfigs/admin.conf
	@./hack/local-garden/apply-rbac-garden-ns

	# Now you can start using the cluster at with `export KUBECONFIG=hack/local-garden/kubeconfigs/default-admin.conf`
	# Then you need to run ./hack/dev-setup-register-gardener to register gardener.
	# Finally, run `make start-apiserver,start-controller-manager,start-scheduler,start-gardenlet` to start the gardener components as usual.

.PHONY: local-garden-down
local-garden-down:
	@-./hack/local-garden/cleanup

.PHONY: start-apiserver
start-apiserver:
	@./hack/start-apiserver

.PHONY: start-controller-manager
start-controller-manager:
	@./hack/start-controller-manager

.PHONY: start-scheduler
start-scheduler:
	@./hack/start-scheduler

.PHONY: start-gardenlet
start-gardenlet:
	@./hack/start-gardenlet

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod vendor
	@GO111MODULE=on go mod tidy

.PHONY: build
build:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
		-mod=vendor \
		-ldflags "$(LD_FLAGS)" \
		-o bin/gardener-apiserver \
		cmd/gardener-apiserver/*.go
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
		-mod=vendor \
		-ldflags "$(LD_FLAGS)" \
		-o bin/gardener-controller-manager \
		cmd/gardener-controller-manager/*.go
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
		-mod=vendor \
		-ldflags "$(LD_FLAGS)" \
		-o bin/gardener-scheduler \
		cmd/gardener-scheduler/*.go
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
		-mod=vendor \
		-ldflags "$(LD_FLAGS)" \
		-o bin/gardenlet \
		cmd/gardenlet/*.go

.PHONY: build-local
build-local:
	@GOBIN=${WORKDIR}/bin go install \
		-ldflags "$(LD_FLAGS)" \
		./cmd/...

.PHONY: release
release: build build-local docker-images docker-login docker-push rename-binaries

.PHONY: docker-images
docker-images:
	@docker build -t $(APISERVER_IMAGE_REPOSITORY):$(IMAGE_TAG)         -t $(APISERVER_IMAGE_REPOSITORY):latest         -f Dockerfile --target apiserver .
	@docker build -t $(CONROLLER_MANAGER_IMAGE_REPOSITORY):$(IMAGE_TAG) -t $(CONROLLER_MANAGER_IMAGE_REPOSITORY):latest -f Dockerfile --target controller-manager .
	@docker build -t $(SCHEDULER_IMAGE_REPOSITORY):$(IMAGE_TAG)         -t $(SCHEDULER_IMAGE_REPOSITORY):latest         -f Dockerfile --target scheduler .
	@docker build -t $(GARDENLET_IMAGE_REPOSITORY):$(IMAGE_TAG)         -t $(GARDENLET_IMAGE_REPOSITORY):latest         -f Dockerfile --target gardenlet .

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-push
docker-push:
	@if ! docker images $(APISERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(APISERVER_IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(CONROLLER_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(CONROLLER_MANAGER_IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(SCHEDULER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(SCHEDULER_IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(GARDENLET_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(GARDENLET_IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make docker-images'"; false; fi
	@gcloud docker -- push $(APISERVER_IMAGE_REPOSITORY):$(IMAGE_TAG)
	@if [[ "$(PUSH_LATEST)" == "true" ]]; then gcloud docker -- push $(APISERVER_IMAGE_REPOSITORY):latest; fi
	@gcloud docker -- push $(CONROLLER_MANAGER_IMAGE_REPOSITORY):$(IMAGE_TAG)
	@if [[ "$(PUSH_LATEST)" == "true" ]]; then gcloud docker -- push $(CONROLLER_MANAGER_IMAGE_REPOSITORY):latest; fi
	@gcloud docker -- push $(SCHEDULER_IMAGE_REPOSITORY):$(IMAGE_TAG)
	@if [[ "$(PUSH_LATEST)" == "true" ]]; then gcloud docker -- push $(SCHEDULER_IMAGE_REPOSITORY):latest; fi
	@gcloud docker -- push $(GARDENLET_IMAGE_REPOSITORY):$(IMAGE_TAG)
	@if [[ "$(PUSH_LATEST)" == "true" ]]; then gcloud docker -- push $(GARDENLET_IMAGE_REPOSITORY):latest; fi

.PHONY: rename-binaries
rename-binaries:
	@if [[ -f bin/gardener-apiserver ]]; then cp bin/gardener-apiserver gardener-apiserver-darwin-amd64; fi
	@if [[ -f bin/gardener-controller-manager ]]; then cp bin/gardener-controller-manager gardener-controller-manager-darwin-amd64; fi
	@if [[ -f bin/gardener-scheduler ]]; then cp bin/gardener-scheduler gardener-scheduler-darwin-amd64; fi
	@if [[ -f bin/gardenlet ]]; then cp bin/gardenlet gardenlet-darwin-amd64; fi
	@if [[ -f bin/rel/gardener-apiserver ]]; then cp bin/rel/gardener-apiserver gardener-apiserver-linux-amd64; fi
	@if [[ -f bin/rel/gardener-controller-manager ]]; then cp bin/rel/gardener-controller-manager gardener-controller-manager-linux-amd64; fi
	@if [[ -f bin/rel/gardener-scheduler ]]; then cp bin/rel/gardener-scheduler gardener-scheduler-linux-amd64; fi
	@if [[ -f bin/rel/gardenlet ]]; then cp bin/rel/gardenlet gardenlet-linux-amd64; fi

.PHONY: clean
clean:
	@rm -rf bin/
	@rm -f *linux-amd64
	@rm -f *darwin-amd64

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: verify
verify: check test

.PHONY: check
check:
	@.ci/check

.PHONY: test
test:
	@.ci/test

.PHONY: test-cov
test-cov:
	@env COVERAGE=1 .ci/test
	@echo "mode: set" > gardener.coverprofile && find . -name "*.coverprofile" -type f | xargs cat | grep -v mode: | sort -r | awk '{if($$1 != last) {print $$0;last=$$1}}' >> gardener.coverprofile
	@go tool cover -html=gardener.coverprofile -o=gardener.coverage.html
	@rm gardener.coverprofile

.PHONY: test-clean
test-clean:
	@find . -name "*.coverprofile" -type f -delete
	@rm -f gardener.coverage.html

.PHONY: generate
generate:
	@./hack/generate-code
	@./hack/generate-reference-doc
