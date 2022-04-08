#!/bin/sh -e
OLD_KUBE_PROXY_MODE="$(cat "$1")"
if [ -z "${OLD_KUBE_PROXY_MODE}" ] || [ "${OLD_KUBE_PROXY_MODE}" = "${KUBE_PROXY_MODE}" ]; then
  echo "${KUBE_PROXY_MODE}" >"$1"
  echo "Nothing to cleanup - the mode didn't change."
  exit 0
fi

# Workaround kube-proxy bug when switching from ipvs to iptables mode
if iptables -t filter -L KUBE-NODE-PORT; then
  echo "KUBE-NODE-PORT chain exists, flushing it..."
  iptables -t filter -F KUBE-NODE-PORT
fi

/usr/local/bin/kube-proxy --v=2 --cleanup --config=/var/lib/kube-proxy-config/config.yaml --proxy-mode="${OLD_KUBE_PROXY_MODE}"
echo "${KUBE_PROXY_MODE}" >"$1"
