# base
ARG image
FROM ${image} AS base
ENV GOTOOLCHAIN=auto
RUN echo "Installing Packages ..." \
		&& apt-get update \
		&& apt-get install -y --no-install-recommends \
			unzip \
			jq \
			parallel \
			zip \
		&& rm -rf /var/lib/apt/lists/*

# builder
FROM base AS builder
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY
WORKDIR /go/src/github.com/gardener/gardener
COPY . .
RUN make create-tools-bin TOOLS_BIN_DIR=hack/tools/bin && \
    rm -rf /go/src/github.com/gardener/gardener/hack/tools/bin/work && \
    find hack/tools/bin -type l -exec sh -c 'mv -f $(realpath $0) $0' {} \;

# golang-test
FROM base AS golang-test
COPY --from=builder /go/src/github.com/gardener/gardener/hack/tools/bin /gardenertools
