#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset

MULTI_ZONAL="false"
CLUSTER_NAME=""
IPFAMILY="ipv4"

SUDO=""
if [[ "$(id -u)" != "0" ]]; then
  SUDO="sudo "
fi

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

if ! command -v ip &>/dev/null; then
  if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "'ip' command not found. Please install 'ip' command, refer https://github.com/gardener/gardener/blob/master/docs/development/local_setup.md#installing-iproute2" 1>&2
    exit 1
  fi
  echo "Skipping loopback device setup because 'ip' command is not available..."
  return
fi

# In some scenarios a developer might run the kind cluster with a docker daemon that is running inside a VM.
#
# Most people are using Docker Desktop which is able to mirror the IP addresses 
# added to the loopback device on the host into it's managed VM running the docker daemon.
#
# However if people are not using this approach,
# docker might not be able to start the kind containers as the IP
# is not visible for it since it's only attached to the loopback device on the host.
#
# We test the loopback devices by checking on the host directly as well as checking 
# the loopback on the docker host using an container that is running in the host network.

container_ip() {
	docker run --rm --cap-add NET_ADMIN --network=host alpine ip $@
}

for ip_func in "ip" "container_ip"; do
  LOOPBACK_DEVICE=$(${ip_func} address | grep LOOPBACK | sed "s/^[0-9]\+: //g" | awk '{print $1}' | sed "s/:$//g")
  echo "Checking loopback device ${LOOPBACK_DEVICE} with ${ip_func}..."

  for address in "${LOOPBACK_IP_ADDRESSES[@]}"; do
    if ${ip_func} address show dev ${LOOPBACK_DEVICE} | grep -q $address/; then
      echo "IP address $address already assigned to ${LOOPBACK_DEVICE}."
    else
      echo "Adding IP address $address to ${LOOPBACK_DEVICE}..."
      if [[ ${ip_func} == "ip" ]]; then
        sudo_ip_func=${SUDO}${ip_func}
      else 
        sudo_ip_func=${ip_func}
      fi
      ${sudo_ip_func} address add "$address" dev "${LOOPBACK_DEVICE}"
    fi
  done

  echo "Setting up loopback device ${LOOPBACK_DEVICE} with ${ip_func} completed."
done
