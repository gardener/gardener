#############      builder       #############
FROM golang:1.20.4 AS builder

WORKDIR /go/src/github.com/gardener/gardener
COPY . .

ARG EFFECTIVE_VERSION

RUN make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION

############# distroless-static
FROM gcr.io/distroless/static-debian11:nonroot as distroless-static

#############      apiserver     #############
FROM distroless-static AS apiserver

COPY --from=builder /go/bin/gardener-apiserver /gardener-apiserver

WORKDIR /

ENTRYPOINT ["/gardener-apiserver"]

############# controller-manager #############
FROM distroless-static AS controller-manager

COPY --from=builder /go/bin/gardener-controller-manager /gardener-controller-manager
COPY charts /charts

WORKDIR /

ENTRYPOINT ["/gardener-controller-manager"]

############# scheduler #############
FROM distroless-static AS scheduler

COPY --from=builder /go/bin/gardener-scheduler /gardener-scheduler

WORKDIR /

ENTRYPOINT ["/gardener-scheduler"]

############# gardenlet #############
FROM distroless-static AS gardenlet

COPY --from=builder /go/bin/gardenlet /gardenlet
COPY charts /charts

USER nonroot
WORKDIR /

ENTRYPOINT ["/gardenlet"]

############# admission-controller #############
FROM distroless-static AS admission-controller

COPY --from=builder /go/bin/gardener-admission-controller /gardener-admission-controller

WORKDIR /

ENTRYPOINT ["/gardener-admission-controller"]

############# resource-manager #############
FROM distroless-static AS resource-manager

COPY --from=builder /go/bin/gardener-resource-manager /gardener-resource-manager

WORKDIR /

ENTRYPOINT ["/gardener-resource-manager"]

############# operator #############
FROM distroless-static AS operator

COPY --from=builder /go/bin/gardener-operator /gardener-operator
COPY charts /charts

WORKDIR /

ENTRYPOINT ["/gardener-operator"]

############# gardener-extension-provider-local #############
FROM distroless-static AS gardener-extension-provider-local

COPY --from=builder /go/bin/gardener-extension-provider-local /gardener-extension-provider-local
COPY charts/gardener/provider-local /charts/gardener/provider-local

WORKDIR /

ENTRYPOINT ["/gardener-extension-provider-local"]
