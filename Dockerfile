#############      builder       #############
FROM golang:1.13.6 AS builder

WORKDIR /go/src/github.com/gardener/gardener
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install \
  -mod=vendor \
  -ldflags "-X github.com/gardener/gardener/pkg/version.gitVersion=$(cat VERSION) \
            -X github.com/gardener/gardener/pkg/version.gitTreeState=$([ -z git status --porcelain 2>/dev/null ] && echo clean || echo dirty) \
            -X github.com/gardener/gardener/pkg/version.gitCommit=$(git rev-parse --verify HEAD) \
            -X github.com/gardener/gardener/pkg/version.buildDate=$(date --iso-8601=seconds)" \
  ./...

############# base
FROM alpine:3.11.3 AS base

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
