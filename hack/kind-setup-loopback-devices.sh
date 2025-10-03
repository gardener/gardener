#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset

MULTI_ZONAL="false"
CLUSTER_NAME=""
IPFAMILY="ipv4"

ip() {
  docker run --rm --cap-add NET_ADMIN --network=host alpine ip $@
}

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --cluster-name)
      shift; CLUSTER_NAME="${1}"
      ;;
    --ip-family)
      shift; IPFAMILY="${1}"
      ;;
    --multi-zonal)
      MULTI_ZONAL=true
      ;;
    esac

    shift
  done
}

parse_flags "$@"

LOOPBACK_IP_ADDRESSES=(172.18.255.1)
if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
  LOOPBACK_IP_ADDRESSES+=(::1)
fi

if [[ "$MULTI_ZONAL" == "true" ]]; then
  LOOPBACK_IP_ADDRESSES+=(172.18.255.10 172.18.255.11 172.18.255.12)
  if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
    LOOPBACK_IP_ADDRESSES+=(::10 ::11 ::12)
  fi
fi

if [[ "$CLUSTER_NAME" != "*local2*" ]] ; then
  LOOPBACK_IP_ADDRESSES+=(172.18.255.22)
  if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
    LOOPBACK_IP_ADDRESSES+=(::22)
  fi
fi

if [[ "$CLUSTER_NAME" == "gardener-operator-local" ]]; then
  LOOPBACK_IP_ADDRESSES+=(172.18.255.3)
  if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
    LOOPBACK_IP_ADDRESSES+=(::3)
  fi
elif [[ "$CLUSTER_NAME" == "gardener-local2" || "$CLUSTER_NAME" == "gardener-local-multi-node2" ]]; then
  LOOPBACK_IP_ADDRESSES+=(172.18.255.2)
  if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
    LOOPBACK_IP_ADDRESSES+=(::2)
  fi
fi

LOOPBACK_DEVICE=$(ip address | grep LOOPBACK | sed "s/^[0-9]\+: //g" | awk '{print $1}' | sed "s/:$//g")
echo "Checking loopback device ${LOOPBACK_DEVICE}..."
for address in "${LOOPBACK_IP_ADDRESSES[@]}"; do
  if ip address show dev ${LOOPBACK_DEVICE} | grep -q $address/; then
    echo "IP address $address already assigned to ${LOOPBACK_DEVICE}."
  else
    echo "Adding IP address $address to ${LOOPBACK_DEVICE}..."
    ip address add "$address" dev "${LOOPBACK_DEVICE}"
  fi
done
echo "Setting up loopback device ${LOOPBACK_DEVICE} completed."
