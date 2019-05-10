#############      builder       #############
FROM golang:1.12.5 AS builder

WORKDIR /go/src/github.com/gardener/gardener
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install \
  -ldflags "-X github.com/gardener/gardener/pkg/version.gitVersion=$(cat VERSION) \
            -X github.com/gardener/gardener/pkg/version.gitTreeState=$([ -z git status --porcelain 2>/dev/null ] && echo clean || echo dirty) \
            -X github.com/gardener/gardener/pkg/version.gitCommit=$(git rev-parse --verify HEAD) \
            -X github.com/gardener/gardener/pkg/version.buildDate=$(date --rfc-3339=seconds | sed 's/ /T/')" \
  ./...

#############      apiserver     #############
FROM alpine:3.8 AS apiserver

RUN apk add --update bash curl tzdata

COPY --from=builder /go/bin/gardener-apiserver /gardener-apiserver

WORKDIR /

ENTRYPOINT ["/gardener-apiserver"]

############# controller-manager #############
FROM alpine:3.8 AS controller-manager

RUN apk add --update bash curl openvpn tzdata

COPY --from=builder /go/bin/gardener-controller-manager /gardener-controller-manager
COPY charts /charts

WORKDIR /

ENTRYPOINT ["/gardener-controller-manager"]
