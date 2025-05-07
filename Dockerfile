# builder
FROM golang:1.24.3 AS builder
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY
WORKDIR /go/src/github.com/gardener/gardener
COPY . .
ARG EFFECTIVE_VERSION
RUN make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION

# distroless-static
FROM gcr.io/distroless/static-debian12:nonroot as distroless-static

# apiserver
FROM distroless-static AS apiserver
COPY --from=builder /go/bin/gardener-apiserver /gardener-apiserver
WORKDIR /
ENTRYPOINT ["/gardener-apiserver"]

# controller-manager
FROM distroless-static AS controller-manager
COPY --from=builder /go/bin/gardener-controller-manager /gardener-controller-manager
WORKDIR /
ENTRYPOINT ["/gardener-controller-manager"]

# scheduler
FROM distroless-static AS scheduler
COPY --from=builder /go/bin/gardener-scheduler /gardener-scheduler
WORKDIR /
ENTRYPOINT ["/gardener-scheduler"]

# gardenlet
FROM distroless-static AS gardenlet
COPY --from=builder /go/bin/gardenlet /gardenlet
WORKDIR /
ENTRYPOINT ["/gardenlet"]

# admission-controller
FROM distroless-static AS admission-controller
COPY --from=builder /go/bin/gardener-admission-controller /gardener-admission-controller
WORKDIR /
ENTRYPOINT ["/gardener-admission-controller"]

# resource-manager
FROM distroless-static AS resource-manager
COPY --from=builder /go/bin/gardener-resource-manager /gardener-resource-manager
WORKDIR /
ENTRYPOINT ["/gardener-resource-manager"]

# node-agent
FROM distroless-static AS node-agent
COPY --from=builder /go/bin/gardener-node-agent /gardener-node-agent
WORKDIR /
ENTRYPOINT ["/gardener-node-agent"]

# operator
FROM distroless-static AS operator
COPY --from=builder /go/bin/gardener-operator /gardener-operator
WORKDIR /
ENTRYPOINT ["/gardener-operator"]

# gardener-extension-provider-local
FROM distroless-static AS gardener-extension-provider-local
COPY --from=builder /go/bin/gardener-extension-provider-local /gardener-extension-provider-local
WORKDIR /
ENTRYPOINT ["/gardener-extension-provider-local"]

# gardener-extension-admission-local
FROM distroless-static AS gardener-extension-admission-local
COPY --from=builder /go/bin/gardener-extension-admission-local /gardener-extension-admission-local
WORKDIR /
ENTRYPOINT ["/gardener-extension-admission-local"]
