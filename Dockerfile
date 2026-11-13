# builder
FROM --platform=$BUILDPLATFORM golang:1.27.4@sha256:5e8afd493a7124a8586e134d9b532731161d4b9becf9dee5a5956b107426f93d AS builder
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY
WORKDIR /go/src/github.com/gardener/gardener
COPY . .
ARG EFFECTIVE_VERSION
ARG TARGETOS
ARG TARGETARCH
RUN make build EFFECTIVE_VERSION=$EFFECTIVE_VERSION GOOS=$TARGETOS GOARCH=$TARGETARCH BUILD_OUTPUT_FILE="/output/bin/"

# distroless-static
FROM gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329 AS distroless-static

# apiserver
FROM distroless-static AS apiserver
COPY --from=builder /output/bin/gardener-apiserver /gardener-apiserver
WORKDIR /
ENTRYPOINT ["/gardener-apiserver"]

# controller-manager
FROM distroless-static AS controller-manager
COPY --from=builder /output/bin/gardener-controller-manager /gardener-controller-manager
WORKDIR /
ENTRYPOINT ["/gardener-controller-manager"]

# scheduler
FROM distroless-static AS scheduler
COPY --from=builder /output/bin/gardener-scheduler /gardener-scheduler
WORKDIR /
ENTRYPOINT ["/gardener-scheduler"]

# gardenlet
FROM distroless-static AS gardenlet
COPY --from=builder /output/bin/gardenlet /gardenlet
WORKDIR /
ENTRYPOINT ["/gardenlet"]

# gardenadm
FROM distroless-static AS gardenadm
COPY --from=builder /output/bin/gardenadm /gardenadm
WORKDIR /
ENTRYPOINT ["/gardenadm"]

# admission-controller
FROM distroless-static AS admission-controller
COPY --from=builder /output/bin/gardener-admission-controller /gardener-admission-controller
WORKDIR /
ENTRYPOINT ["/gardener-admission-controller"]

# resource-manager
FROM distroless-static AS resource-manager
COPY --from=builder /output/bin/gardener-resource-manager /gardener-resource-manager
WORKDIR /
ENTRYPOINT ["/gardener-resource-manager"]

# node-agent
FROM distroless-static AS node-agent
COPY --from=builder /output/bin/gardener-node-agent /gardener-node-agent
WORKDIR /
ENTRYPOINT ["/gardener-node-agent"]

# operator
FROM distroless-static AS operator
COPY --from=builder /output/bin/gardener-operator /gardener-operator
WORKDIR /
ENTRYPOINT ["/gardener-operator"]

# gardener-extension-provider-local
FROM distroless-static AS gardener-extension-provider-local
COPY --from=builder /output/bin/gardener-extension-provider-local /gardener-extension-provider-local
WORKDIR /
ENTRYPOINT ["/gardener-extension-provider-local"]

# gardener-extension-admission-local
FROM distroless-static AS gardener-extension-admission-local
COPY --from=builder /output/bin/gardener-extension-admission-local /gardener-extension-admission-local
WORKDIR /
ENTRYPOINT ["/gardener-extension-admission-local"]
