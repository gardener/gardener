# Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
IMAGE_TAG                          := $(shell cat VERSION)

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: dev-setup
dev-setup:
	@./hack/dev-setup

.PHONY: dev-setup-local
dev-setup-local:
	@./hack/dev-setup-local

.PHONY: start-api
start-api:
	@go run cmd/gardener-apiserver/main.go \
			--authentication-kubeconfig ~/.kube/config \
			--authorization-kubeconfig ~/.kube/config \
			--enable-admission-plugins=ResourceReferenceManager,ShootSeedManager,ShootDNSHostedZone,ShootValidator,ShootQuotaValidator \
			--etcd-servers=http://$(shell minikube ip):32379 \
			--kubeconfig ~/.kube/config \
			--tls-cert-file ~/.minikube/apiserver.crt \
			--tls-private-key-file ~/.minikube/apiserver.key \
			--secure-port=8443 \
			--v=2

.PHONY: start
start:
	@KUBECONFIG=~/.kube/config GARDENER_KUBECONFIG=~/.kube/config go run \
			-ldflags "-w -X github.com/gardener/gardener/pkg/version.Version=$(shell ./hack/get-next-version)" \
			cmd/gardener-controller-manager/main.go \
			--config=dev/componentconfig-gardener-controller-manager.yaml

.PHONY: start-local
start-local:
	@go run cmd/gardener-local-provider/main.go

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: revendor
revendor:
	@dep ensure -update

.PHONY: build
build:
	@.ci/build

.PHONY: build-local
build-local:
	@env LOCAL_BUILD=1 .ci/build

.PHONY: release
release: build build-local docker-images docker-login docker-push rename-binaries

.PHONY: docker-images
docker-images:
	@if [[ ! -f bin/rel/gardener-apiserver || ! -f bin/rel/gardener-controller-manager ]]; then echo "No binary found. Please run 'make build'"; false; fi
	@docker build -t $(APISERVER_IMAGE_REPOSITORY):$(IMAGE_TAG)         -t $(APISERVER_IMAGE_REPOSITORY):latest         -f build/gardener-apiserver/Dockerfile          --rm .
	@docker build -t $(CONROLLER_MANAGER_IMAGE_REPOSITORY):$(IMAGE_TAG) -t $(CONROLLER_MANAGER_IMAGE_REPOSITORY):latest -f build/gardener-controller-manager/Dockerfile --rm .

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-push
docker-push:
	@if ! docker images $(APISERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(APISERVER_IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make docker-images'"; false; fi
	@if ! docker images $(CONROLLER_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(CONROLLER_MANAGER_IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make docker-images'"; false; fi
	@gcloud docker -- push $(APISERVER_IMAGE_REPOSITORY):$(IMAGE_TAG)
	@gcloud docker -- push $(APISERVER_IMAGE_REPOSITORY):latest
	@gcloud docker -- push $(CONROLLER_MANAGER_IMAGE_REPOSITORY):$(IMAGE_TAG)
	@gcloud docker -- push $(CONROLLER_MANAGER_IMAGE_REPOSITORY):latest

.PHONY: rename-binaries
rename-binaries:
	@if [[ -f bin/gardener-apiserver ]]; then cp bin/gardener-apiserver gardener-apiserver-darwin-amd64; fi
	@if [[ -f bin/gardener-controller-manager ]]; then cp bin/gardener-controller-manager gardener-controller-manager-darwin-amd64; fi
	@if [[ -f bin/rel/gardener-apiserver ]]; then cp bin/rel/gardener-apiserver gardener-apiserver-linux-amd64; fi
	@if [[ -f bin/rel/gardener-controller-manager ]]; then cp bin/rel/gardener-controller-manager gardener-controller-manager-linux-amd64; fi

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
