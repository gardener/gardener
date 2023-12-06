# base
ARG image
FROM ${image} AS base
RUN echo "Installing Packages ..." \
		&& apt-get update \
		&& apt-get install -y --no-install-recommends \
			unzip \
			jq \
			parallel \
		&& rm -rf /var/lib/apt/lists/*

# builder
FROM base as builder
WORKDIR /go/src/github.com/gardener/gardener
COPY . .
RUN make create-tools-bin

# golang-test
FROM base AS golang-test
COPY --from=builder /go/src/github.com/gardener/gardener/hack/tools/bin /gardenertools
