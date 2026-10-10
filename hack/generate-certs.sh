#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

# Args:
#   1: component_dir    (path where certs/ folder is located)
#   2: common_name      (CN for the certificate)
#   3: SANs             (subjectAltName list)
component_dir="$1"
common_name="$2"
SANs="$3"

cert_dir="$component_dir/certs"
mkdir -p "$cert_dir"

ca_key="$cert_dir/ca.key"
ca_crt="$cert_dir/ca.crt"
tls_key="$cert_dir/tls.key"
tls_csr="$cert_dir/tls.csr"
tls_crt="$cert_dir/tls.crt"

echo "Generating certificates in: $cert_dir"

if [[ ! -s "$ca_crt" || ! -s "$ca_key" ]]; then
    echo "No CA found. Generating new CA key and certificate."

    openssl genrsa -out "$ca_key" 3072

    openssl req -x509 -new -nodes \
        -key "$ca_key" \
        -sha256 \
        -days 3650 \
        -out "$ca_crt" \
        -subj "/CN=webhook-ca" \
        -addext "basicConstraints=CA:TRUE" \
        -addext "keyUsage=keyCertSign,cRLSign" \
        -addext "subjectKeyIdentifier=hash"
fi

if [[ -s "$ca_key" && -s "$ca_crt" ]]; then
    if openssl x509 -checkend 86400 -in "$ca_crt" >/dev/null 2>&1; then
        echo "CA certificate is valid and will be reused."
    else
        echo "CA certificate has expired. Regenerating."
        openssl req -x509 -new -nodes \
            -key "$ca_key" \
            -sha256 \
            -days 3650 \
            -out "$ca_crt" \
            -subj "/CN=webhook-ca" \
            -addext "basicConstraints=CA:TRUE" \
            -addext "keyUsage=keyCertSign,cRLSign" \
            -addext "subjectKeyIdentifier=hash"
    fi
fi

should_generate_cert=false

if [[ -s "$tls_key" && -s "$tls_crt" ]]; then
    if openssl x509 -checkend 86400 -in "$tls_crt" >/dev/null 2>&1; then
        echo "TLS certificate is valid and will be reused."
    else
        echo "TLS certificate has expired. Regenerating."
        should_generate_cert=true
    fi
else
    echo "No TLS certificate found. Generating new one."
    should_generate_cert=true
fi

if [[ "$should_generate_cert" == true ]]; then
    echo "Generating new TLS certificate..."

    openssl genrsa -out "$tls_key" 3072

    openssl req -new -key "$tls_key" -out "$tls_csr" \
        -subj "/CN=${common_name}" \
        -addext "subjectAltName=${SANs}"

    openssl x509 -req \
        -in "$tls_csr" \
        -CA "$ca_crt" -CAkey "$ca_key" \
        -out "$tls_crt" -days 365 -sha256 \
        -extfile <(printf "subjectAltName=%s" "$SANs")

    rm -f "$tls_csr"
    echo "TLS certificate generated successfully."
fi

echo "Certificate generation completed for ${component_dir}"
