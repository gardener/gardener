# builder
ARG image
FROM ${image} AS builder
RUN set -eux; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		unzip \
		jq \
		parallel \
	; \
	rm -rf /var/lib/apt/lists/*
WORKDIR /go/src/github.com/gardener/gardener
COPY . .
RUN make create-tools-bin

# gardenertools
FROM scratch AS gardenertools
COPY --from=builder /go/src/github.com/gardener/gardener/hack/tools/bin /gardenertools
