#!/bin/sh -e
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
