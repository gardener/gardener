# Copyright 2018 The Gardener Authors.
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

VCS                                := github.com
ORGANIZATION                       := gardener
PROJECT                            := gardener
REPOSITORY                         := $(VCS)/$(ORGANIZATION)/$(PROJECT)
VERSION                            := $(shell cat VERSION)
LD_FLAGS                           := "-w -X $(REPOSITORY)/pkg/version.Version=$(VERSION)"
PACKAGES                           := $(shell go list ./... | grep -vE '/vendor/|/pkg/client/garden|/pkg/apis|/pkg/openapi')
TEST_FOLDERS                       := cmd pkg plugin
LINT_FOLDERS                       := $(shell echo $(PACKAGES) | sed "s|$(REPOSITORY)|.|g")
REGISTRY                           := eu.gcr.io/gardener-project/gardener
APISERVER_IMAGE_REPOSITORY         := $(REGISTRY)/apiserver
APISERVER_IMAGE_TAG                := $(VERSION)
CONROLLER_MANAGER_IMAGE_REPOSITORY := $(REGISTRY)/controller-manager
CONROLLER_MANAGER_IMAGE_TAG        := $(VERSION)

TYPES_FILES      := $(shell find pkg/apis -name types.go)

BUILD_DIR        := build
BIN_DIR          := bin
GOBIN            := $(PWD)/$(BIN_DIR)
GO_EXTRA_FLAGS   := -v
PATH             := $(GOBIN):$(PATH)
USER             := $(shell id -u -n)

export PATH
export GOBIN

.PHONY: dev-setup
dev-setup:
	@./hack/dev-setup

.PHONY: dev-apiserver
dev-apiserver:
	@go run cmd/gardener-apiserver/main.go \
			--admission-control=ResourceReferenceManager,ShootSeedManager,ShootDNSHostedZone,ShootValidator,ShootQuotaValidator \
			--etcd-servers=http://$(shell minikube ip):32379 \
			--tls-cert-file ~/.minikube/apiserver.crt \
			--tls-private-key-file ~/.minikube/apiserver.key \
			--secure-port=8443 \
			--kubeconfig ~/.kube/config \
			--authentication-kubeconfig ~/.kube/config \
			--authorization-kubeconfig ~/.kube/config \
			--v=2

.PHONY: dev
dev:
	$(eval LD_FLAGS_RUN = "-w -X $(REPOSITORY)/pkg/version.Version="$(shell ./hack/get-next-version))
	@KUBECONFIG=~/.kube/config GARDENER_KUBECONFIG=~/.kube/config go run -ldflags $(LD_FLAGS_RUN) cmd/gardener-controller-manager/main.go --config=dev/componentconfig-gardener-controller-manager.yaml

.PHONY: verify
verify: vet fmt lint test

.PHONY: revendor
revendor:
	@dep ensure -update


.PHONY: build
build: apiserver-build controller-manager-build

.PHONY: release
release: apiserver-release controller-manager-release


.PHONY: apiserver-release
apiserver-release: apiserver-build apiserver-build-release apiserver-docker-image docker-login apiserver-docker-push apiserver-rename-binaries

.PHONY: apiserver-build
apiserver-build:
	@go build -o $(BIN_DIR)/gardener-apiserver $(GO_EXTRA_FLAGS) -ldflags $(LD_FLAGS) cmd/gardener-apiserver/*.go

.PHONY: apiserver-build-release
apiserver-build-release:
	@env GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/rel/gardener-apiserver $(GO_EXTRA_FLAGS) -ldflags $(LD_FLAGS) cmd/gardener-apiserver/*.go

.PHONY: apiserver-docker-image
apiserver-docker-image:
	@if [[ ! -f $(BIN_DIR)/rel/gardener-apiserver ]]; then echo "No binary found. Please run 'make apiserver-build-release'"; false; fi
	@docker build -t $(APISERVER_IMAGE_REPOSITORY):$(APISERVER_IMAGE_TAG) -f $(BUILD_DIR)/gardener-apiserver/Dockerfile --rm .

.PHONY: apiserver-docker-push
apiserver-docker-push:
	@if ! docker images $(APISERVER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(APISERVER_IMAGE_TAG); then echo "$(APISERVER_IMAGE_REPOSITORY) version $(APISERVER_IMAGE_TAG) is not yet built. Please run 'make apiserver-docker-image'"; false; fi
	@gcloud docker -- push $(APISERVER_IMAGE_REPOSITORY):$(APISERVER_IMAGE_TAG)

.PHONY: apiserver-rename-binaries
apiserver-rename-binaries:
	@if [[ -f $(BIN_DIR)/gardener-apiserver ]]; then cp $(BIN_DIR)/gardener-apiserver gardener-apiserver-darwin-amd64; fi
	@if [[ -f $(BIN_DIR)/rel/gardener-apiserver ]]; then cp $(BIN_DIR)/rel/gardener-apiserver gardener-apiserver-linux-amd64; fi


.PHONY: controller-manager-release
controller-manager-release: controller-manager-build controller-manager-build-release controller-manager-docker-image docker-login controller-manager-docker-push controller-manager-rename-binaries

.PHONY: controller-manager-build
controller-manager-build:
	@go build -o $(BIN_DIR)/gardener-controller-manager $(GO_EXTRA_FLAGS) -ldflags $(LD_FLAGS) cmd/gardener-controller-manager/*.go

.PHONY: controller-manager-build-release
controller-manager-build-release:
	@env GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/rel/gardener-controller-manager $(GO_EXTRA_FLAGS) -ldflags $(LD_FLAGS) cmd/gardener-controller-manager/*.go

.PHONY: controller-manager-docker-image
controller-manager-docker-image:
	@if [[ ! -f $(BIN_DIR)/rel/gardener-controller-manager ]]; then echo "No binary found. Please run 'make controller-manager-build-release'"; false; fi
	@docker build -t $(CONROLLER_MANAGER_IMAGE_REPOSITORY):$(CONROLLER_MANAGER_IMAGE_TAG) -f $(BUILD_DIR)/gardener-controller-manager/Dockerfile --rm .

.PHONY: controller-manager-docker-push
controller-manager-docker-push:
	@if ! docker images $(CONROLLER_MANAGER_IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(CONROLLER_MANAGER_IMAGE_TAG); then echo "$(CONROLLER_MANAGER_IMAGE_REPOSITORY) version $(CONROLLER_MANAGER_IMAGE_TAG) is not yet built. Please run 'make controller-manager-docker-image'"; false; fi
	@gcloud docker -- push $(CONROLLER_MANAGER_IMAGE_REPOSITORY):$(CONROLLER_MANAGER_IMAGE_TAG)

.PHONY: controller-manager-rename-binaries
controller-manager-rename-binaries:
	@if [[ -f $(BIN_DIR)/gardener-controller-manager ]]; then cp $(BIN_DIR)/gardener-controller-manager gardener-controller-manager-darwin-amd64; fi
	@if [[ -f $(BIN_DIR)/rel/gardener-controller-manager ]]; then cp $(BIN_DIR)/rel/gardener-controller-manager gardener-controller-manager-linux-amd64; fi


.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: clean
clean:
	@rm -rf $(BIN_DIR)/
	@rm -f *linux-amd64
	@rm -f *darwin-amd64

.PHONY: fmt
fmt:
	@go fmt $(PACKAGES)

.PHONY: vet
vet:
	@go vet $(PACKAGES)

.PHONY: lint
lint:
	@for package in $(LINT_FOLDERS); do \
		golint -set_exit_status $$(find $$package -maxdepth 1 -name "*.go" | grep -vE 'zz_generated|_test.go') || exit 1; \
	done

.PHONY: test
test:
	@ginkgo -r $(TEST_FOLDERS)

.PHONY: test-cov
test-cov:
	@ginkgo -cover -r $(TEST_FOLDERS)
	@echo "mode: set" > gardener.coverprofile && find . -name "*.coverprofile" -type f | xargs cat | grep -v mode: | sort -r | awk '{if($$1 != last) {print $$0;last=$$1}}' >> gardener.coverprofile
	@go tool cover -html=gardener.coverprofile -o=gardener.coverage.html
	@rm gardener.coverprofile

.PHONY: test-clean
test-clean:
	@find . -name "*.coverprofile" -type f -delete
	@rm -f gardener.coverage.html
