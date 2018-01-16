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

VCS              := github.com
ORGANIZATION     := gardener
PROJECT          := gardener
REPOSITORY       := $(VCS)/$(ORGANIZATION)/$(PROJECT)
VERSION          := $(shell cat VERSION)
LD_FLAGS         := "-w -X $(REPOSITORY)/pkg/version.Version=$(VERSION)"
PACKAGES         := $(shell go list ./... | grep -vE '/vendor/|/pkg/client/garden|/pkg/apis')
TEST_FOLDERS     := cmd pkg plugin
LINT_FOLDERS     := $(shell echo $(PACKAGES) | sed "s|$(REPOSITORY)|.|g")
REGISTRY         := eu.gcr.io/sap-cloud-platform-dev1
IMAGE_REPOSITORY := $(REGISTRY)/garden/$(PROJECT)
IMAGE_TAG        := $(VERSION)

TYPES_FILES      := $(shell find pkg/apis -name types.go)

BUILD_DIR        := build
BIN_DIR          := bin
GOBIN            := $(PWD)/$(BIN_DIR)
GO_EXTRA_FLAGS   := -v
PATH             := $(GOBIN):$(PATH)
USER             := $(shell id -u -n)

export PATH
export GOBIN

.PHONY: dev
dev:
	$(eval LD_FLAGS_RUN = "-w -X $(REPOSITORY)/pkg/version.Version="$(shell ./hack/get-next-version))
	@KUBECONFIG=dev/garden-kubeconfig.yaml WATCH_NAMESPACE=$(USER) go run -ldflags $(LD_FLAGS_RUN) cmd/garden-controller-manager/main.go --config=dev/componentconfig-garden-controller-manager.yaml

.PHONY: dev-all
dev-all:
	$(eval LD_FLAGS_RUN = "-w -X $(REPOSITORY)/pkg/version.Version="$(shell ./hack/get-next-version))
	@KUBECONFIG=dev/garden-kubeconfig.yaml go run -ldflags $(LD_FLAGS_RUN) cmd/garden-controller-manager/main.go --config=dev/componentconfig-garden-controller-manager.yaml

.PHONY: verify
verify: vet fmt lint test

.PHONY: revendor
revendor:
	@dep ensure -update
	@dep prune


.PHONY: build
build: apiserver-build controller-manager-build

.PHONY: release
release: apiserver-release controller-manager-release


.PHONY: apiserver-release
apiserver-release: apiserver-build apiserver-docker-build apiserver-docker-image docker-login apiserver-docker-push apiserver-rename-binaries

.PHONY: apiserver-build
apiserver-build:
	@go build -o $(BIN_DIR)/garden-apiserver $(GO_EXTRA_FLAGS) -ldflags $(LD_FLAGS) cmd/garden-apiserver/*.go

.PHONY: apiserver-build-release
apiserver-build-release:
	@go build -o /go/bin/garden-apiserver $(GO_EXTRA_FLAGS) -ldflags $(LD_FLAGS) cmd/garden-apiserver/*.go

.PHONY: apiserver-docker-build
apiserver-docker-build:
	@./$(BUILD_DIR)/garden-apiserver/docker-build.sh
	@sudo chown $(user):$(group) $(BIN_DIR)/rel/garden-apiserver

.PHONY: apiserver-docker-image
apiserver-docker-image:
	@if [[ ! -f $(BIN_DIR)/rel/garden-apiserver ]]; then echo "No binary found. Please run 'make apiserver-docker-build'"; false; fi
	@docker build -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) -f $(BUILD_DIR)/garden-apiserver/Dockerfile --rm .

.PHONY: apiserver-docker-push
apiserver-docker-push:
	@if ! docker images $(IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make apiserver-docker-image'"; false; fi
	@gcloud docker -- push $(IMAGE_REPOSITORY):$(IMAGE_TAG)

.PHONY: apiserver-rename-binaries
apiserver-rename-binaries:
	@if [[ -f $(BIN_DIR)/garden-apiserver ]]; then cp $(BIN_DIR)/garden-apiserver garden-apiserver-darwin-amd64; fi
	@if [[ -f $(BIN_DIR)/rel/garden-apiserver ]]; then cp $(BIN_DIR)/rel/garden-apiserver garden-apiserver-linux-amd64; fi


.PHONY: controller-manager-release
controller-manager-release: controller-manager-build controller-manager-docker-build controller-manager-docker-image docker-login controller-manager-docker-push controller-manager-rename-binaries

.PHONY: controller-manager-build
controller-manager-build:
	@go build -o $(BIN_DIR)/garden-controller-manager $(GO_EXTRA_FLAGS) -ldflags $(LD_FLAGS) cmd/garden-controller-manager/*.go

.PHONY: controller-manager-build-release
controller-manager-build-release:
	@go build -o /go/bin/garden-controller-manager $(GO_EXTRA_FLAGS) -ldflags $(LD_FLAGS) cmd/garden-controller-manager/*.go

.PHONY: controller-manager-docker-build
controller-manager-docker-build:
	@./$(BUILD_DIR)/garden-controller-manager/docker-build.sh
	@sudo chown $(user):$(group) $(BIN_DIR)/rel/garden-controller-manager

.PHONY: controller-manager-docker-image
controller-manager-docker-image:
	@if [[ ! -f $(BIN_DIR)/rel/garden-controller-manager ]]; then echo "No binary found. Please run 'make controller-manager-docker-build'"; false; fi
	@docker build -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) -f $(BUILD_DIR)/garden-controller-manager/Dockerfile --rm .

.PHONY: controller-manager-docker-push
controller-manager-docker-push:
	@if ! docker images $(IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make controller-manager-docker-image'"; false; fi
	@gcloud docker -- push $(IMAGE_REPOSITORY):$(IMAGE_TAG)

.PHONY: controller-manager-rename-binaries
controller-manager-rename-binaries:
	@if [[ -f $(BIN_DIR)/garden-controller-manager ]]; then cp $(BIN_DIR)/garden-controller-manager garden-controller-manager-darwin-amd64; fi
	@if [[ -f $(BIN_DIR)/rel/garden-controller-manager ]]; then cp $(BIN_DIR)/rel/garden-controller-manager garden-controller-manager-linux-amd64; fi


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
