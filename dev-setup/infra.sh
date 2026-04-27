#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

COMMAND="${1:-up}"
VALID_COMMANDS=("up" "down" "setup-loopback-devices")

INFRA_COMPOSE_FILE="$(dirname "$0")/infra/docker-compose.yaml"
DIR_BACKUP_BUCKET="$(dirname "$0")/../dev/local-backupbuckets"
DIR_REGISTRY="$(dirname "$0")/../dev/local-registry"

SUDO=""
if [[ "$(id -u)" != "0" ]]; then
  SUDO="sudo "
fi

case "$COMMAND" in
  up)
    check_shell_dependencies() {
      errors=()

      if ! sed --version >/dev/null 2>&1; then
        errors+=("Current sed version does not support --version flag. Please ensure GNU sed is installed.")
      fi

      if tar --version 2>&1 | grep -q "bsdtar"; then
        errors+=("BSD tar detected. Please ensure GNU tar is installed.")
      fi

      if grep --version 2>&1 | grep -q "BSD grep"; then
        errors+=("BSD grep detected. Please ensure GNU grep is installed.")
      fi

      if [[ "$OSTYPE" == "darwin"* ]]; then
        if ! date --version >/dev/null 2>&1; then
          errors+=("Current date version does not support --version flag. Please ensure coreutils are installed.")
        fi

        if gzip --version 2>&1 | grep -q "Apple"; then
          errors+=("Apple built-in gzip utility detected. Please ensure GNU gzip is installed.")
        fi
      fi

      if [ "${#errors[@]}" -gt 0 ]; then
        printf 'Error: Required shell dependencies not met. Please refer to https://github.com/gardener/gardener/blob/master/docs/development/local_setup.md#macos-only-install-gnu-core-utilities:\n'
        printf '    - %s\n' "${errors[@]}"
        exit 1
      fi
    }

    # The local registry needs to be available from the host for pushing and from the containers for pulling.
    # On the host, we bind the registry to localhost (see infra/docker-compose.yaml), because 127.0.0.1 and ::1
    # are configured as HTTP-only (insecure-registries) by default in Docker, which allows `docker push` without
    # changing the Docker daemon config.
    # From within the containers (e.g., the kind nodes), the registry domain is resolved via Docker's built-in
    # DNS server to the IP of the registry container because of the host alias configured in docker compose.
    #
    # We could also bind the registry to an 172.18.255.* address similar to bind9 to make the registry reachable
    # from the host and containers via the same IP. This would be cleaner, because we wouldn't need to add entries
    # to /etc/hosts for the registry domain and would resolve the domain from the host via bind9 just as all other
    # domains.
    # However, this would require changing the Docker daemon config to set registry.local.gardener.cloud as an
    # insecure registry to allow pushing to it from the host. The insecureRegistries settings in the skaffold config
    # doesn't apply here, because skaffold uses the Docker daemon/CLI under the hood for pushing images, which only
    # considers the Docker daemon's registry configuration.
    ensure_local_registry_hosts() {
      local host="registry.local.gardener.cloud"

      for ip in 127.0.0.1 ::1 ; do
        if ! grep -Eq "^$ip $host$" /etc/hosts; then
          echo "> Adding entry '$ip $host' to /etc/hosts..."
          echo "$ip $host" | ${SUDO}tee -a /etc/hosts
        else
          echo "> /etc/hosts already contains entry '$ip $host', skipping..."
        fi
      done
    }

    setup_local_dns_resolver() {
      local dns_ip=172.18.255.53
      local dns_ipv6=fd00:ff::53

      # Special handling in CI: we don't have a fully-fledged systemd-resolved or similar in the CI environment, so we set
      # up dnsmasq as a local DNS resolver with conditional forwarding for the local.gardener.cloud domain to the local
      # setup's DNS server (bind9).
      # Setting bind9 as the nameserver in /etc/resolv.conf directly does not work, as bind9 itself forwards to the host's
      # nameservers configured in resolv.conf, creating a cyclic dependency. With dnsmasq however, we can configure it to
      # forward requests only for the local.gardener.cloud domain to the local setup's DNS server, and forward all other
      # requests to the default nameservers (the Prow cluster's coredns), which works fine.
      if [ -n "${CI:-}" -a -n "${ARTIFACTS:-}" ]; then
        mkdir -p /etc/dnsmasq.d/
        cp /etc/resolv.conf /etc/resolv-default.conf
        tee /etc/dnsmasq.d/gardener-local.conf <<EOF
# Force dnsmasq to listen ONLY on standard localhost and prevent it from scanning other interfaces/IPs.
# Without this, it ignores the server directive for local.gardener.cloud because the IP is bound to the loopback
# interface and assumes doing so would create an infinite loop.
listen-address=127.0.0.1
bind-interfaces

# Configure conditional forwarding for local.gardener.cloud but use the resolv.conf from Kubernetes (coredns) as
# upstream for all other requests, which is required for resolving the registry cache services in the Prow cluster.
server=/local.gardener.cloud/$dns_ip
resolv-file=/etc/resolv-default.conf

# Export dnsmasq logs to a file for debugging purposes
log-facility=/var/log/dnsmasq.log
log-queries
EOF

        service dnsmasq start || service dnsmasq restart

        echo "> Setting dnsmasq as nameserver in /etc/resolv.conf..."
        # /etc/resolv.conf is shared between all containers in the pod, i.e., it will also be used by the injected sidecar
        # containers (e.g., for uploading artifacts to GCS). Hence, we keep the previous nameservers as fallback if dnsmasq
        # is not working, but set dnsmasq as the first entry to ensure it is used as primary resolver for the test job.
        # We cannot use sed -i on the /etc/resolv.conf bind mount that Kubernetes adds, so we need to write to a temp file
        # and then overwrite the resolv.conf with the combined content.
        echo "nameserver 127.0.0.1" > /tmp/resolv.conf
        cat /etc/resolv.conf >> /tmp/resolv.conf
        cat /tmp/resolv.conf > /etc/resolv.conf
        rm /tmp/resolv.conf

        echo "> Content of /etc/resolv.conf after setting dnsmasq as nameserver"
        cat /etc/resolv.conf

        return 0
      fi

      if [[ "$OSTYPE" == "darwin"* ]]; then
        local desired_resolver_config="nameserver $dns_ip"
        if ! grep -q "$desired_resolver_config" /etc/resolver/local.gardener.cloud ; then
          echo "Configuring macOS to resolve the local.gardener.cloud zone using the local setup's DNS server"
          ${SUDO}mkdir -p /etc/resolver
          echo "$desired_resolver_config" | ${SUDO}tee /etc/resolver/local.gardener.cloud
        fi
      elif [[ "$OSTYPE" == "linux"* && -f /etc/systemd/resolved.conf ]]; then
        if [[ ! -d /etc/systemd/resolved.conf.d ]]; then
          ${SUDO}mkdir -p /etc/systemd/resolved.conf.d
        fi
        if ! grep -q "$dns_ip" /etc/systemd/resolved.conf.d/gardener-local.conf || ! grep -q "$dns_ipv6" /etc/systemd/resolved.conf.d/gardener-local.conf ; then
          echo "Configuring systemd-resolved to resolve the local.gardener.cloud zone using the local setup's DNS server"
          cat <<EOF | ${SUDO}tee /etc/systemd/resolved.conf.d/gardener-local.conf
[Resolve]
DNS=$dns_ip $dns_ipv6
Domains=~local.gardener.cloud
EOF
          echo "restarting systemd-resolved"
          ${SUDO}systemctl restart systemd-resolved
        fi
      elif ! nslookup -type=ns local.gardener.cloud >/dev/null 2>/dev/null ; then
        echo "Warning: Unknown OS. Make sure your host resolves the local.gardener.cloud zone using the local setup's DNS server at $dns_ip or $dns_ipv6 respectively."
        return 0
      fi
    }

    # setup_kind_network is similar to kind's network creation logic, ref https://github.com/kubernetes-sigs/kind/blob/23d2ac0e9c41028fa252dd1340411d70d46e2fd4/pkg/cluster/internal/providers/docker/network.go#L50
    # In addition to kind's logic, we ensure stable CIDRs that we can rely on in our local setup manifests and code.
    setup_kind_network() {
      # check if network already exists
      local existing_network_id
      existing_network_id="$(docker network list --filter=name=^kind$ --format='{{.ID}}')"

      if [ -n "$existing_network_id" ] ; then
        # ensure the network is configured correctly
        local network network_options network_ipam expected_network_ipam
        network="$(docker network inspect $existing_network_id | yq '.[]')"
        network_options="$(echo "$network" | yq '.EnableIPv6 + "," + .Options["com.docker.network.bridge.enable_ip_masquerade"]')"
        network_ipam="$(echo "$network" | yq '.IPAM.Config' -o=json -I=0 | sed -E 's/"IPRange":"",//g')"
        expected_network_ipam='[{"Subnet":"172.18.0.0/24","Gateway":"172.18.0.1"},{"Subnet":"fd00:10::/64","Gateway":"fd00:10::1"}]'

        if [ "$network_options" = 'true,true' ] && [ "$network_ipam" = "$expected_network_ipam" ] ; then
          # kind network is already configured correctly, nothing to do
          return 0
        else
          echo "kind network is not configured correctly for local gardener setup, recreating network with correct configuration..."
          docker network rm $existing_network_id
        fi
      fi

      # (re-)create kind network with expected settings
      docker network create kind --driver=bridge \
        --subnet 172.18.0.0/24 --gateway 172.18.0.1 \
        --ipv6 --subnet fd00:10::/64 --gateway fd00:10::1 \
        --opt com.docker.network.bridge.enable_ip_masquerade=true
    }

    change_registry_upstream_urls_to_prow_caches() {
      if [[ "${CI:-false}" != "true" ]]; then
        return
      fi

      local mutated_compose_file="${INFRA_COMPOSE_FILE%.yaml}-prow.yaml"
      cp "$INFRA_COMPOSE_FILE" "$mutated_compose_file"
      INFRA_COMPOSE_FILE="$mutated_compose_file"

      declare -A prow_registry_cache_urls=(
        [gcr]="http://registry-gcr-io.kube-system.svc.cluster.local:5001"
        [k8s]="http://registry-registry-k8s-io.kube-system.svc.cluster.local:5001"
        [quay]="http://registry-quay-io.kube-system.svc.cluster.local:5001"
        [europe-docker-pkg-dev]="http://registry-europe-docker-pkg-dev.kube-system.svc.cluster.local:5001"
      )

      echo "Running in CI. Checking availability of registry-cache instances in prow cluster"

      for key in "${!prow_registry_cache_urls[@]}"; do
        registry_cache_url="${prow_registry_cache_urls[$key]}"

        # Extract DNS from URL (remove http:// and :5001)
        registry_cache_dns="${registry_cache_url#http://}"
        registry_cache_dns="${registry_cache_dns%:5001}"

        registry_cache_ip=$(getent hosts "$registry_cache_dns" | awk '{ print $1 }' || true)
        if [[ "$registry_cache_ip" == "" ]]; then
          echo "Unable to resolve IP of $key registry cache in prow cluster ($registry_cache_dns). Not using it as upstream."
          continue
        fi

        echo "Using $key registry cache in prow cluster ($registry_cache_dns) as upstream for local registry cache."
        yq -i ".services.registry-cache-${key}.environment.REGISTRY_PROXY_REMOTEURL = \"${registry_cache_url}\"" "$INFRA_COMPOSE_FILE"
      done
    }

    check_shell_dependencies

    mkdir -m 0755 -p "$DIR_BACKUP_BUCKET" "$DIR_REGISTRY"

    "$(dirname "$0")/infra.sh" setup-loopback-devices

    setup_kind_network

    change_registry_upstream_urls_to_prow_caches

    docker compose -f "$INFRA_COMPOSE_FILE" up -d

    ensure_local_registry_hosts
    setup_local_dns_resolver

    if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
      # All outgoing traffic from the cluster is masqueraded to the host's IPv6 address. This is to ensure outgoing traffic
      # can also be routed back to the cluster.
      ip6tables -t nat -A POSTROUTING -o $(ip route | grep '^default' | awk '{print $5}') -s $(docker network inspect kind -f='{{json .IPAM.Config}}' | jq -r '.[1].Subnet') -j MASQUERADE
    fi
    ;;

  down)
    # Reset dynamic updates to the DNS zones by removing the volumes.
    docker compose -f "$INFRA_COMPOSE_FILE" down --volumes

    # Remove all load balancer containers (including the ones of shoot clusters) to get rid of any orphaned containers.
    echo "Removing load balancer containers of all clusters"
    for container in $(docker container ls -aq --filter network=kind --filter label=gardener.cloud/role=loadbalancer); do
      docker container rm -f "$container"
    done

    # Delete the local backup bucket directory
    # When deleting the secondary cluster, we might still need it for the other cluster.
    # We need root privileges to clean the backup bucket directory, see https://github.com/gardener/gardener/issues/6752
    docker run --rm --user root:root -v "$DIR_BACKUP_BUCKET":/dev/local-backupbuckets alpine rm -rf /dev/local-backupbuckets/garden-*
    rm -rf "$DIR_BACKUP_BUCKET"
    ;;

  setup-loopback-devices)
    # 172.18.255.53 exposes the bind9 DNS server
    # 172.18.255.123 exposes the gind apiserver load balancer to the host
    LOOPBACK_IP_ADDRESSES=(172.18.255.53 fd00:ff::53 172.18.255.123)

    # load balancer range IPv4 (172.18.255.224/27)
    LOOPBACK_IP_ADDRESSES+=( $(printf "172.18.255.%d\n" {224..255}) )
    if [[ "$IPFAMILY" == "ipv6" ]] || [[ "$IPFAMILY" == "dual" ]]; then
      # load balancer range IPv6 (fd00:ff::e0/123)
      LOOPBACK_IP_ADDRESSES+=( $(printf "fd00:ff::e%x\n" {0..15}) $(printf "fd00:ff::f%x\n" {0..15}) )
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
      if [[ ${ip_func} == "container_ip" ]] && docker info | grep -q 'Operating System: Docker Desktop' ; then
        # For Docker Desktop, we don't need to add the IP addresses to the loopback device inside the VM, as Docker Desktop
        # automatically mirrors the IPs of the host's loopback device into the VM.
        continue
      fi

      LOOPBACK_DEVICE=$(${ip_func} address | grep LOOPBACK | sed "s/^[0-9]\+: //g" | awk '{print $1}' | sed "s/:$//g")
      echo "Checking loopback device ${LOOPBACK_DEVICE} with ${ip_func}..."

      for address in "${LOOPBACK_IP_ADDRESSES[@]}"; do
        addr_output=$(${ip_func} address show dev ${LOOPBACK_DEVICE})
        if echo "$addr_output" | grep -q "$address/"; then
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
    ;;

  *)
    echo "Error: Invalid command '${COMMAND}'. Valid options are: ${VALID_COMMANDS[*]}." >&2
    exit 1
    ;;
esac
