#!/bin/sh -e
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

OLD_KUBE_PROXY_MODE="$(cat "$1")"
if [ -z "${OLD_KUBE_PROXY_MODE}" ] || [ "${OLD_KUBE_PROXY_MODE}" = "${KUBE_PROXY_MODE}" ]; then
  echo "${KUBE_PROXY_MODE}" >"$1"
  echo "Nothing to cleanup - the mode didn't change."
  exit 0
fi

# Workaround kube-proxy bug (https://github.com/kubernetes/kubernetes/issues/109286) when switching from ipvs to iptables mode.
# The fix (https://github.com/kubernetes/kubernetes/pull/109288) is present in 1.25+.
if [ "${EXECUTE_WORKAROUND_FOR_K8S_ISSUE_109286}" = "true" ]; then
  if iptables -t filter -L KUBE-NODE-PORT; then
    echo "KUBE-NODE-PORT chain exists, flushing it..."
    iptables -t filter -F KUBE-NODE-PORT
  fi
fi

/usr/local/bin/kube-proxy --v=2 --cleanup --config=/var/lib/kube-proxy-config/config.yaml --proxy-mode="${OLD_KUBE_PROXY_MODE}"
echo "${KUBE_PROXY_MODE}" >"$1"
