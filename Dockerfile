#############      builder       #############
FROM eu.gcr.io/gardener-project/3rd/golang:1.15.5 AS builder

WORKDIR /go/src/github.com/gardener/gardener
COPY . .

ARG EFFECTIVE_VERSION

RUN make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION

############# base
FROM eu.gcr.io/gardener-project/3rd/alpine:3.12.1 AS base

#############      apiserver     #############
FROM base AS apiserver

RUN apk add --update tzdata

COPY --from=builder /go/bin/gardener-apiserver /gardener-apiserver

WORKDIR /

ENTRYPOINT ["/gardener-apiserver"]

############# controller-manager #############
FROM base AS controller-manager

RUN apk add --update tzdata

COPY --from=builder /go/bin/gardener-controller-manager /gardener-controller-manager
COPY charts /charts

WORKDIR /

ENTRYPOINT ["/gardener-controller-manager"]

############# scheduler #############
FROM base AS scheduler

COPY --from=builder /go/bin/gardener-scheduler /gardener-scheduler

WORKDIR /

ENTRYPOINT ["/gardener-scheduler"]

############# gardenlet #############
FROM base AS gardenlet

RUN apk add --update openvpn tzdata

COPY --from=builder /go/bin/gardenlet /gardenlet
COPY charts /charts

WORKDIR /

ENTRYPOINT ["/gardenlet"]

############# admission-controller #############
FROM base AS admission-controller

COPY --from=builder /go/bin/gardener-admission-controller /gardener-admission-controller

WORKDIR /

ENTRYPOINT ["/gardener-admission-controller"]

############# seed-admission-controller #############
FROM base AS seed-admission-controller

COPY --from=builder /go/bin/gardener-seed-admission-controller /gardener-seed-admission-controller

WORKDIR /

ENTRYPOINT ["/gardener-seed-admission-controller"]

############# konnectivity-server-base #############
# Make sure to update Makefile when updating this version!!!
FROM k8s.gcr.io/kas-network-proxy/proxy-server:v0.0.14 AS konnectivity-server-base

############# konnectivity-server-reloader #############
FROM base AS konnectivity-server-reloader

COPY --from=builder /go/bin/konnectivity-server-reloader /konnectivity-server-reloader
COPY --from=konnectivity-server-base /proxy-server /proxy-server

WORKDIR /

ENTRYPOINT ["/konnectivity-server-reloader"]
