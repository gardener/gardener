#!/usr/bin/env bash
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

ensure-config-file() {
  local file=$1
  local tmpl="$file".tmpl
  if [[ -n "$2" ]]; then
    tmpl=$2
  fi
  if [[ ! -f "$file" ]]; then
    echo "Creating \"$file\" from template."
    cp "$tmpl" "$file"
  fi
}

check-not-initial() {
  local file=$1
  local yqArg=$2

  if [[ $yqArg == "" ]]; then
    if [[ ! -f "$file" ]]; then
      echo "File \"$file\" does not exist. Please check your config."
      exit 1
    fi
  else
    local yqResult
    yqResult=$(yq "${yqArg}" "$file")
    if [[  $yqResult  == "" ]] || [[  $yqResult  == "null" ]] || [[  $yqResult == "[]" ]] || [[  $yqResult == "{}" ]]; then
      echo "\"$yqArg\" in file \"$file\" is empty or does not exist. Please check your config."
      exit 1
    fi
  fi

}

ensure-gardener-dns-annotations() {
  local namespace=$1
  local name=$2
  local domain=$3

  kubectl annotate --overwrite svc -n "$namespace" "$name" \
    dns.gardener.cloud/class=garden \
    dns.gardener.cloud/ttl="60" \
    dns.gardener.cloud/dnsnames="$domain" \
    cert.gardener.cloud/commonname="$domain" \
    cert.gardener.cloud/dnsnames="$domain" \
    cert.gardener.cloud/purpose=managed \
    cert.gardener.cloud/secretname=tls
}

command="${1:-up}"
valid_commands=("up down")
workload_identity_support="${2:-false}"

cp -f "$SCRIPT_DIR/remote/kubeconfigs/kubeconfig" "$SCRIPT_DIR/kubeconfigs/runtime/kubeconfig"
cp -f "$SCRIPT_DIR/remote/kubeconfigs/kubeconfig" "$SCRIPT_DIR/gardenlet/components/kubeconfigs/seed-remote/kubeconfig"

client_certificate_data=$(kubectl config view --raw -o jsonpath='{.users[0].user.client-certificate-data}')
if [[ -n "$client_certificate_data" ]] && [[ $(echo "$client_certificate_data" | base64 --decode | openssl x509 -noout -checkend 300) == "Certificate will expire" ]]; then
  echo "Runtime kubeconfig ${SCRIPT_DIR}/remote/kubeconfigs/kubeconfig has expired or will expire in 5min. Please provide a valid kubeconfig and try again!"
  rm -f "$SCRIPT_DIR/kubeconfigs/runtime/kubeconfig"
  rm -f "$SCRIPT_DIR/gardenlet/components/kubeconfigs/seed-remote/kubeconfig"
  exit 1
fi

existing_gardens=""
if kubectl get crd gardens.operator.gardener.cloud >/dev/null 2>&1; then
  existing_gardens=$(kubectl get garden --no-headers -o custom-columns=":metadata.name")
fi

case "$command" in
  up)
    use_shoot_info="false"
    temp_shoot_info=$(mktemp)
    cleanup-shoot-info() {
      rm -f "$temp_shoot_info"
    }
    trap cleanup-shoot-info EXIT

    echo "Ensuring config files"
    ensure-config-file "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml
    ensure-config-file "$SCRIPT_DIR"/garden/overlays/remote/secret-backup.yaml
    ensure-config-file "$SCRIPT_DIR"/garden/overlays/remote/secret-dns.yaml
    ensure-config-file "$SCRIPT_DIR"/gardenconfig/overlays/remote/cloudprofile/cloudprofiles.yaml
    ensure-config-file "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/credentials-project-garden/credentialsbindings.yaml
    ensure-config-file "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/credentials-project-remote/credentialsbindings.yaml
    ensure-config-file "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/etcd-backup/secret.yaml
    ensure-config-file "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml
    # Kustomize does not support conditional logic, so we create all files and only check the content of the relevant
    # ones based on the workload identity support. The other files are just touched.
    # All objects could coexist in the cluster, so it is possible to switch between workload identity and secret based credentials.
    if [[ "$workload_identity_support" == "true" ]]; then
      ensure-config-file "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml
      ensure-config-file "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/credentials-project-garden/infrastructure-workloadidentities.yaml
      ensure-config-file "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/credentials-project-remote/infrastructure-workloadidentities.yaml
      touch "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml
      touch "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/credentials-project-garden/infrastructure-secrets.yaml
      touch "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/credentials-project-remote/infrastructure-secrets.yaml
    else
      ensure-config-file "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml
      ensure-config-file "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/credentials-project-garden/infrastructure-secrets.yaml
      ensure-config-file "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/credentials-project-remote/infrastructure-secrets.yaml
      touch "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml
      touch "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/credentials-project-garden/infrastructure-workloadidentities.yaml
      touch "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/credentials-project-remote/infrastructure-workloadidentities.yaml
    fi

    echo "Check if essential config options are initialized"
    check-not-initial "$SCRIPT_DIR"/garden/overlays/remote/secret-dns.yaml 'select(document_index == 0) | .data'

    if [[ "$workload_identity_support" == "true" ]]; then
      echo "Using workload identities"
      check-not-initial "$REPO_ROOT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml 'select(document_index == 0) | .metadata.namespace'
      check-not-initial "$REPO_ROOT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml 'select(document_index == 0) | .spec.audiences'
      check-not-initial "$REPO_ROOT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml 'select(document_index == 0) | .spec.targetSystem.type'
      check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/garden/project/with-workload-identity/credentials/infrastructure-workloadidentities.yaml '.metadata.namespace'
      check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/garden/project/with-workload-identity/credentials/infrastructure-workloadidentities.yaml '.spec.audiences'
      check-not-initial "$REPO_ROOT_DIR"/example/provider-extensions/garden/project/with-workload-identity/credentials/infrastructure-workloadidentities.yaml '.spec.targetSystem.type'

      internal_dns_workload_identity=$(yq -e 'select(document_index == 1) | .metadata.name' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml)
      internal_dns_workload_identity_ns=$(yq -e 'select(document_index == 1) | .metadata.namespace' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml)
      internal_dns_provider_type=$(yq -e 'select(document_index == 1) | .spec.targetSystem.type' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml)
      default_dns_workload_identity=$(yq -e 'select(document_index == 0) | .metadata.name' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml)
      default_dns_workload_identity_ns=$(yq -e 'select(document_index == 0) | .metadata.namespace' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml)
      default_dns_provider_type=$(yq -e 'select(document_index == 0) | .spec.targetSystem.type' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml)

      role1=$(yq 'select(document_index == 0) | .metadata.labels.["gardener.cloud/role"]' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml)
      role2=$(yq 'select(document_index == 1) | .metadata.labels.["gardener.cloud/role"]' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml)

      if [[ $role1 != "default-domain" ]]; then
        echo "first workload identity in $SCRIPT_DIR/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml must be labeled as gardener.cloud/role=default-domain"
        exit 1
      fi
      if [[ $role2 != "internal-domain" ]]; then
        echo "second workload identity in $SCRIPT_DIR/gardenconfig/overlays/remote/credentials/domains/domain-workload-identities.yaml must be labeled as gardener.cloud/role=internal-domain"
        exit 1
      fi

      yq -e -i "
        .spec.config.seedConfig.spec.dns.provider.credentialsRef.apiVersion = \"security.gardener.cloud/v1alpha1\" |
        .spec.config.seedConfig.spec.dns.provider.credentialsRef.kind = \"WorkloadIdentity\" |
        .spec.config.seedConfig.spec.dns.provider.credentialsRef.name = \"$internal_dns_workload_identity\" |
        .spec.config.seedConfig.spec.dns.provider.credentialsRef.namespace = \"$internal_dns_workload_identity_ns\" |
        .spec.config.seedConfig.spec.dns.provider.type = \"$internal_dns_provider_type\" |
        .spec.config.seedConfig.spec.dns.internal.credentialsRef.apiVersion = \"security.gardener.cloud/v1alpha1\" |
        .spec.config.seedConfig.spec.dns.internal.credentialsRef.kind = \"WorkloadIdentity\" |
        .spec.config.seedConfig.spec.dns.internal.credentialsRef.name = \"$internal_dns_workload_identity\" |
        .spec.config.seedConfig.spec.dns.internal.credentialsRef.namespace = \"$internal_dns_workload_identity_ns\" |
        .spec.config.seedConfig.spec.dns.internal.type = \"$internal_dns_provider_type\" |
        .spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.apiVersion = \"security.gardener.cloud/v1alpha1\" |
        .spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.kind = \"WorkloadIdentity\" |
        .spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.name = \"$default_dns_workload_identity\" |
        .spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.namespace = \"$default_dns_workload_identity_ns\" |
        .spec.config.seedConfig.spec.dns.defaults[0].type = \"$default_dns_provider_type\"
      " "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml
    else
      echo "Using secret based credentials"
      check-not-initial "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml 'select(document_index == 0) | .data'
      check-not-initial "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml 'select(document_index == 1) | .data'
      check-not-initial "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml 'select(document_index == 0) | .metadata.annotations.["dns.gardener.cloud/domain"]'
      check-not-initial "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml 'select(document_index == 1) | .metadata.annotations.["dns.gardener.cloud/domain"]'
      check-not-initial "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml 'select(document_index == 0) | .metadata.annotations.["dns.gardener.cloud/provider"]'
      check-not-initial "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml 'select(document_index == 1) | .metadata.annotations.["dns.gardener.cloud/provider"]'

      role1=$(yq 'select(document_index == 0) | .metadata.labels.["gardener.cloud/role"]' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml)
      role2=$(yq 'select(document_index == 1) | .metadata.labels.["gardener.cloud/role"]' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml)

      if [[ $role1 != "default-domain" ]]; then
        echo "first secret in $SCRIPT_DIR/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml must be labeled as gardener.cloud/role=default-domain"
        exit 1
      fi
      if [[ $role2 != "internal-domain" ]]; then
        echo "second secret in $SCRIPT_DIR/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml must be labeled as gardener.cloud/role=internal-domain"
        exit 1
      fi

      internal_dns_secret=$(yq -e 'select(document_index == 1) | .metadata.name' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml)
      internal_dns_secret_ns=$(yq -e 'select(document_index == 1) | .metadata.namespace' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml)
      internal_dns_provider_type=$(yq -e 'select(document_index == 1) | .metadata.annotations.["dns.gardener.cloud/provider"]' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml)
      default_dns_secret=$(yq -e 'select(document_index == 0) | .metadata.name' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml)
      default_dns_secret_ns=$(yq -e 'select(document_index == 0) | .metadata.namespace' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml)
      default_dns_provider_type=$(yq -e 'select(document_index == 0) | .metadata.annotations.["dns.gardener.cloud/provider"]' "$SCRIPT_DIR"/gardenconfig/overlays/remote/credentials/domains/domain-secrets.yaml)

      yq -e -i "
        .spec.config.seedConfig.spec.dns.provider.credentialsRef.apiVersion = \"v1\" |
        .spec.config.seedConfig.spec.dns.provider.credentialsRef.kind = \"Secret\" |
        .spec.config.seedConfig.spec.dns.provider.credentialsRef.name = \"$internal_dns_secret\" |
        .spec.config.seedConfig.spec.dns.provider.credentialsRef.namespace = \"$internal_dns_secret_ns\" |
        .spec.config.seedConfig.spec.dns.provider.type = \"$internal_dns_provider_type\" |
        .spec.config.seedConfig.spec.dns.internal.credentialsRef.apiVersion = \"v1\" |
        .spec.config.seedConfig.spec.dns.internal.credentialsRef.kind = \"Secret\" |
        .spec.config.seedConfig.spec.dns.internal.credentialsRef.name = \"$internal_dns_secret\" |
        .spec.config.seedConfig.spec.dns.internal.credentialsRef.namespace = \"$internal_dns_secret_ns\" |
        .spec.config.seedConfig.spec.dns.internal.type = \"$internal_dns_provider_type\" |
        .spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.apiVersion = \"v1\" |
        .spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.kind = \"Secret\" |
        .spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.name = \"$default_dns_secret\" |
        .spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.namespace = \"$default_dns_secret_ns\" |
        .spec.config.seedConfig.spec.dns.defaults[0].type = \"$default_dns_provider_type\"
      " "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml
    fi

    registry_domain=

    if kubectl get configmaps -n kube-system shoot-info -o yaml > "$temp_shoot_info"; then
      use_shoot_info="true"
      echo "Getting config from shoot"
      registry_domain=reg.$(yq -e '.data.domain' "$temp_shoot_info")
      pods_cidr=$(yq -e '.data.podNetwork' "$temp_shoot_info")
      nodes_cidr=$(yq -e '.data.nodeNetwork' "$temp_shoot_info")
      services_cidr=$(yq -e '.data.serviceNetwork' "$temp_shoot_info")
      region=$(yq -e '.data.region' "$temp_shoot_info")
      type=$(yq -e '.data.provider' "$temp_shoot_info")

      echo "Updating garden.yaml from shoot info"
      yq -e -i "
        .spec.runtimeCluster.networking.pods[0] = \"$pods_cidr\" |
        .spec.runtimeCluster.networking.nodes[0] = \"$nodes_cidr\" |
        .spec.runtimeCluster.networking.services[0] = \"$services_cidr\" |
        .spec.runtimeCluster.provider.region = \"$region\"
      " "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml

      echo "Updating gardenlet.yaml from shoot info"
      yq -e -i "
        .spec.config.seedConfig.spec.networks.pods = \"$pods_cidr\" |
        .spec.config.seedConfig.spec.networks.nodes = \"$nodes_cidr\" |
        .spec.config.seedConfig.spec.networks.services = \"$services_cidr\" |
        .spec.config.seedConfig.spec.provider.region = \"$region\" |
        .spec.config.seedConfig.spec.provider.type = \"$type\"
      " "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml
    else
      echo "######################################################################################"
      echo "Please enter domain names for registry and relay domains on the seed"
      echo "######################################################################################"
      echo "Registry domain:"
      read -er registry_domain
    fi

    echo "$registry_domain" > "$SCRIPT_DIR/remote/registry/registrydomain"

    echo "Deploying load-balancer services"
    kubectl --server-side=true apply -k "$SCRIPT_DIR"/remote/registry/load-balancer/base

    if [[ $use_shoot_info == "true" ]]; then
      ensure-gardener-dns-annotations registry registry "$registry_domain"
    else
      echo "######################################################################################"
      echo "Please create DNS entries and generate TLS certificates for registry and relay domains"
      echo "######################################################################################"
      echo "Registry domain:"
      echo "DNS entry for domain: \"$registry_domain\" -> IP from load balancer service \"kubectl get svc -n registry registry -o yaml\""
      echo "TLS certificate for domain \"$registry_domain\" -> Please store the TLS certificate in secret \"name: tls namespace: registry\" (https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)"
      echo "######################################################################################"
      read -rsp "When you are ready, please press ENTER to continue"
    fi

    echo "Check if garden.yaml is complete"
    check-not-initial "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml ".spec.dns.providers[0].type"
    check-not-initial "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml ".spec.runtimeCluster.ingress.domains[0].name"
    check-not-initial "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml ".spec.runtimeCluster.networking.nodes"
    check-not-initial "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml ".spec.runtimeCluster.networking.pods"
    check-not-initial "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml ".spec.runtimeCluster.networking.services"
    check-not-initial "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml ".spec.runtimeCluster.provider.region"
    check-not-initial "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml ".spec.runtimeCluster.provider.zones"
    check-not-initial "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml ".spec.virtualCluster.dns.domains[0].name"
    check-not-initial "$SCRIPT_DIR"/garden/overlays/remote/garden.yaml ".spec.virtualCluster.networking.services"

    echo "Check if gardenlet.yaml is complete"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.ingress.domain"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.networks.pods"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.networks.nodes"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.networks.services"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.provider.credentialsRef.apiVersion"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.provider.credentialsRef.kind"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.provider.credentialsRef.name"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.provider.credentialsRef.namespace"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.provider.type"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.internal.credentialsRef.apiVersion"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.internal.credentialsRef.kind"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.internal.credentialsRef.name"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.internal.credentialsRef.namespace"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.internal.type"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.internal.domain"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.apiVersion"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.kind"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.name"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.defaults[0].credentialsRef.namespace"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.defaults[0].type"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.dns.defaults[0].domain"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.provider.region"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.provider.type"
    check-not-initial "$SCRIPT_DIR"/gardenlet/overlays/remote/gardenlet.yaml ".spec.config.seedConfig.spec.provider.zones"

    echo "Check if /dev-setup/extensions/remote directory contains at least one extension (.yaml file)"
    yaml_file_count=$(find "$SCRIPT_DIR/extensions/remote" -maxdepth 1 -name "*.yaml" -type f 2>/dev/null | wc -l)
    if [[ $yaml_file_count -eq 0 ]]; then
      echo "Directory \"$SCRIPT_DIR/extensions/remote\" does not contain any extensions (.yaml files). Please check your config."
      exit 1
    fi

    echo "Deploying kyverno and container registry"
    kubectl --server-side=true --force-conflicts=true apply -k "$SCRIPT_DIR"/remote/kyverno
    until kubectl get clusterpolicies.kyverno.io ; do date; sleep 1; echo ""; done
    kubectl --server-side=true --force-conflicts=true apply -k "$SCRIPT_DIR"/remote/kyverno-policies

    virtual_garden_kubeconfig=""
    if [[ "$existing_gardens" != "" ]]; then
      virtual_garden_kubeconfig="$VIRTUAL_GARDEN_KUBECONFIG"
    fi
    "$SCRIPT_DIR"/remote/registry/deploy-registry.sh "$KUBECONFIG" "$registry_domain" "$virtual_garden_kubeconfig"
    ;;

  down)
    if [[ "$existing_gardens" != "" ]]; then
      echo "No clean up of remote cluster because of existing gardens"
      exit 1
    fi

    echo "Removing registry namespaces"
    kubectl delete ns garden registry --ignore-not-found

    echo "Removing container kyverno"
    kubectl delete -k "$SCRIPT_DIR"/remote/kyverno-policies --ignore-not-found
    kubectl delete -k "$SCRIPT_DIR"/remote/kyverno --ignore-not-found
    kubectl delete mutatingwebhookconfigurations kyverno-policy-mutating-webhook-cfg kyverno-resource-mutating-webhook-cfg kyverno-verify-mutating-webhook-cfg --ignore-not-found
    kubectl delete validatingwebhookconfigurations kyverno-cel-exception-validating-webhook-cfg kyverno-cleanup-validating-webhook-cfg kyverno-exception-validating-webhook-cfg kyverno-global-context-validating-webhook-cfg kyverno-policy-validating-webhook-cfg kyverno-resource-validating-webhook-cfg kyverno-ttl-validating-webhook-cfg --ignore-not-found

    echo "Removing kubeconfigs"
    rm -f "$SCRIPT_DIR/kubeconfigs/runtime/kubeconfig"
    rm -f "$SCRIPT_DIR/gardenlet/components/kubeconfigs/seed-remote/kubeconfig"
    ;;

  *)
    echo "Error: Invalid command '${command}'. Valid options are: ${valid_commands[*]}." >&2
    exit 1
    ;;
esac
