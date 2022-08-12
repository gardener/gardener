#############      builder       #############
FROM golang:1.19.0 AS builder

WORKDIR /go/src/github.com/gardener/gardener
COPY . .

ARG EFFECTIVE_VERSION

RUN make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION

############# base
FROM alpine:3.16.2 AS base

############# alpine-openvpn
FROM base AS alpine-openvpn

RUN apk add --no-cache --update openvpn tzdata

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
FROM alpine-openvpn AS dependencies-gardenlet

WORKDIR /volume

RUN mkdir -p ./lib ./usr/sbin ./usr/share ./usr/lib ./tmp ./etc \
    && cp -d /lib/ld-musl-* ./lib \
    && cp -d /lib/libcrypto.so.* ./usr/lib \
    && cp -d /lib/libssl.so.* ./usr/lib \
    && cp -d /usr/lib/liblzo2.so.* ./usr/lib \
    && cp -d /usr/sbin/openvpn ./usr/sbin \
    && cp -r /usr/share/zoneinfo ./usr/share/zoneinfo \
    # nonroot user
    && echo 'nonroot:x:65532:65532:nonroot,,,:/home/nonroot:/sbin/nologin' > ./etc/passwd \
    && echo 'nonroot:x:65532:nonroot' > ./etc/group \
    && mkdir -p ./home/nonroot \
    && chown 65532:65532 ./home/nonroot \
    && chown 65532:65532 ./tmp

FROM scratch AS gardenlet

COPY --from=builder /go/bin/gardenlet /gardenlet
COPY --from=dependencies-gardenlet /volume /
COPY charts /charts

USER nonroot
WORKDIR /

ENTRYPOINT ["/gardenlet"]

############# admission-controller #############
FROM distroless-static AS admission-controller

COPY --from=builder /go/bin/gardener-admission-controller /gardener-admission-controller

WORKDIR /

ENTRYPOINT ["/gardener-admission-controller"]

############# seed-admission-controller #############
FROM distroless-static AS seed-admission-controller

COPY --from=builder /go/bin/gardener-seed-admission-controller /gardener-seed-admission-controller

WORKDIR /

ENTRYPOINT ["/gardener-seed-admission-controller"]

############# resource-manager #############
FROM distroless-static AS resource-manager

COPY --from=builder /go/bin/gardener-resource-manager /gardener-resource-manager

WORKDIR /

ENTRYPOINT ["/gardener-resource-manager"]

############# gardener-extension-provider-local #############
FROM distroless-static AS gardener-extension-provider-local

COPY --from=builder /go/bin/gardener-extension-provider-local /gardener-extension-provider-local
COPY charts/gardener/provider-local /charts/gardener/provider-local

WORKDIR /

ENTRYPOINT ["/gardener-extension-provider-local"]
