#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

if [[ -f "/etc/debian_version" ]]; then
    # Copy certificates from default "localcertsdir" because /usr is mounted read-only in Garden Linux.
    # See https://github.com/gardenlinux/gardenlinux/issues/1490
    mkdir -p "{{ .pathLocalSSLCerts }}"
    if [[ -d "/usr/local/share/ca-certificates" && -n "$(ls -A '/usr/local/share/ca-certificates')" ]]; then
        cp -af /usr/local/share/ca-certificates/* "{{ .pathLocalSSLCerts }}"
    fi
    # localcertsdir is supported on Debian based OS only
    /usr/sbin/update-ca-certificates --fresh --localcertsdir "{{ .pathLocalSSLCerts }}"
else
    /usr/sbin/update-ca-certificates --fresh
fi
