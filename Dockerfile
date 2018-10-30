#############      builder       #############
FROM golang:1.11.1 AS builder

WORKDIR /go/src/github.com/gardener/gardener
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install \
  -ldflags "-w -X github.com/gardener/gardener/pkg/version.Version=$(cat VERSION)" \
  ./...

#############      apiserver     #############
FROM alpine:3.8 AS apiserver

RUN apk add --update bash curl

COPY --from=builder /go/bin/gardener-apiserver /gardener-apiserver

WORKDIR /

ENTRYPOINT ["/gardener-apiserver"]

############# controller-manager #############
FROM alpine:3.8 AS controller-manager

RUN apk add --update bash curl openvpn

COPY --from=builder /go/bin/gardener-controller-manager /gardener-controller-manager
COPY charts /charts

WORKDIR /

ENTRYPOINT ["/gardener-controller-manager"]

############# external-admission-controller #############
FROM alpine:3.8 AS external-admission-controller

RUN apk add --update bash curl

COPY --from=builder /go/bin/gardener-external-admission-controller /gardener-external-admission-controller

WORKDIR /

ENTRYPOINT ["/gardener-external-admission-controller"]
