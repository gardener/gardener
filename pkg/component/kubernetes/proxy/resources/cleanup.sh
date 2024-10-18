#!/bin/sh -e
if [ -f "$1" ]; then
  OLD_KUBE_PROXY_MODE="$(cat "$1")"
fi
if [ -z "${OLD_KUBE_PROXY_MODE}" ] || [ "${OLD_KUBE_PROXY_MODE}" = "${KUBE_PROXY_MODE}" ]; then
  echo "${KUBE_PROXY_MODE}" >"$1"
  echo "Nothing to cleanup - the mode didn't change."
  exit 0
fi

/usr/local/bin/kube-proxy --v=2 --cleanup --config=/var/lib/kube-proxy-config/config.yaml --proxy-mode="${OLD_KUBE_PROXY_MODE}"
echo "${KUBE_PROXY_MODE}" >"$1"
