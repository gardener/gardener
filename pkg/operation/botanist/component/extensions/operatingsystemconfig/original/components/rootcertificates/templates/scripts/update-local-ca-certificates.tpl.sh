#!/bin/bash
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


set -o errexit
set -o nounset
set -o pipefail

if [[ -f "/etc/debian_version" ]]; then
    # Copy certificates from default "localcertsdir" because /usr is mounted read-only in Garden Linux.
    # See https://github.com/gardenlinux/gardenlinux/issues/1490
    mkdir -p "{{ .pathLocalSSLCerts }}"
    if [[ -d "/usr/local/share/ca-certificates" ]]; then
        cp -af /usr/local/share/ca-certificates/* "{{ .pathLocalSSLCerts }}"
    fi
    # localcertsdir is supported on Debian based OS only
    /usr/sbin/update-ca-certificates --fresh --localcertsdir "{{ .pathLocalSSLCerts }}"
else
    /usr/sbin/update-ca-certificates --fresh
fi
