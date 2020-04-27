#############      builder       #############
FROM golang:1.14.2 AS builder

WORKDIR /go/src/github.com/gardener/gardener
COPY . .

RUN make install

############# base
FROM alpine:3.11.6 AS base

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

############# seed-admission-controller #############
FROM base AS seed-admission-controller

COPY --from=builder /go/bin/gardener-seed-admission-controller /gardener-seed-admission-controller

WORKDIR /

ENTRYPOINT ["/gardener-seed-admission-controller"]

############# registry-migrator #############
FROM base AS registry-migrator

COPY --from=builder /go/bin/registry-migrator /registry-migrator

WORKDIR /

ENTRYPOINT ["/registry-migrator"]
