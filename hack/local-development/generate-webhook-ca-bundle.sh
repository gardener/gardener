#!/usr/bin/env bash
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

tmpdir=$(mktemp -d)
service=gardener-admission-controller
namespace=garden

cat <<EOF >> "$tmpdir/csr.conf"
[req]
default_bits       = 2048
prompt             = no
default_md         = sha256
req_extensions     = req_ext
distinguished_name = req_distinguished_name

[req_distinguished_name]
organizationName = Self-signed certificate

[v3_ext]
authorityKeyIdentifier = keyid,issuer:always
basicConstraints       = CA:FALSE
keyUsage               = keyEncipherment,dataEncipherment
extendedKeyUsage       = serverAuth,clientAuth
subjectAltName         = @alt_names

[req_ext]
subjectAltName = @alt_names

[alt_names]
IP.1  = 127.0.0.1
IP.2  = ::1
DNS.1 = ${service}
DNS.2 = ${service}.${namespace}
DNS.3 = ${service}.${namespace}.svc
DNS.4 = ${service}.${namespace}.svc.cluster
DNS.5 = ${service}.${namespace}.svc.cluster.local
EOF


openssl genrsa -out "$tmpdir/ca.key" 2048

# CA
openssl req -x509 -new -nodes -key "$tmpdir/ca.key" -out "$tmpdir/ca.crt" -days 9000 -subj "/CN=gardener-admission-controller"
openssl genrsa -out "$tmpdir/server.key" 2048

# CSR
openssl req -new -key "$tmpdir/server.key" -out "$tmpdir/server.csr" -config "$tmpdir/csr.conf"

# Signing
openssl x509 -req -in "$tmpdir/server.csr" -CA "$tmpdir/ca.crt" -CAkey "$tmpdir/ca.key" -CAcreateserial -out "$tmpdir/server.crt" -extfile "$tmpdir/csr.conf" \
  -days 9000 -sha256 -extensions v3_ext

caBundle=$(openssl enc -a -A < "$tmpdir/ca.crt")
echo "$caBundle" > ../../dev/gardener-admission-controller-caBundle
cat "$tmpdir/server.key" > ../../dev/gardener-admission-controller-server.key
cat "$tmpdir/server.crt" > ../../dev/gardener-admission-controller-server.crt
