#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
GARDEN_KUBECONFIG="${GARDEN_KUBECONFIG:-}"
SEED_KUBECONFIG="${SEED_KUBECONFIG:-}"
SEED_NAME="${SEED_NAME:-}"
BOOTSTRAP_TOKEN_ID=""
BOOTSTRAP_TOKEN_SECRET=""
GARDENLET_NAMESPACE="garden"
GARDENLET_DEPLOYMENT_NAME="gardenlet"

function log_info() {
  echo -e "${GREEN}[INFO]${NC} $1"
}

function log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

function log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

function usage() {
  cat <<EOF
Usage: $0 [OPTIONS]

Re-bootstraps the gardenlet by creating a new bootstrap token and kubeconfig.
This is useful when the gardenlet's client certificate has expired.

Prerequisites:
  - kubectl
  - yq (https://github.com/mikefarah/yq)

Required Options:
  --garden-kubeconfig PATH    Path to garden cluster kubeconfig
  --seed-kubeconfig PATH      Path to seed cluster kubeconfig
  --seed-name NAME            Name of the seed

Optional:
  --token-id ID               Bootstrap token ID (6 characters, random if not provided)
  --token-secret SECRET       Bootstrap token secret (16 characters, random if not provided)
  -h, --help                  Show this help message

Examples:
  $0 --garden-kubeconfig ~/.kube/garden.yaml --seed-kubeconfig ~/.kube/seed.yaml --seed-name my-seed
  $0 --garden-kubeconfig garden.yaml --seed-kubeconfig seed.yaml --seed-name my-seed

EOF
}

function generate_random_string() {
  local length=$1
  LC_ALL=C tr -dc 'a-z0-9' < /dev/urandom | head -c "$length" || true
}

function validate_requirements() {
  log_info "Validating requirements..."

  if [[ -z "$GARDEN_KUBECONFIG" ]]; then
    log_error "Garden kubeconfig is required. Use --garden-kubeconfig option."
    exit 1
  fi

  if [[ ! -f "$GARDEN_KUBECONFIG" ]]; then
    log_error "Garden kubeconfig file not found: $GARDEN_KUBECONFIG"
    exit 1
  fi

  if [[ -z "$SEED_KUBECONFIG" ]]; then
    log_error "Seed kubeconfig is required. Use --seed-kubeconfig option."
    exit 1
  fi

  if [[ ! -f "$SEED_KUBECONFIG" ]]; then
    log_error "Seed kubeconfig file not found: $SEED_KUBECONFIG"
    exit 1
  fi

  if [[ -z "$SEED_NAME" ]]; then
    log_error "Seed name is required. Use --seed-name option."
    exit 1
  fi

  if ! command -v kubectl &> /dev/null; then
    log_error "kubectl is not installed or not in PATH"
    exit 1
  fi

  if ! command -v yq &> /dev/null; then
    log_error "yq is not installed or not in PATH"
    log_error "Please install yq to use this script (https://github.com/mikefarah/yq)"
    exit 1
  fi

  log_info "All requirements validated successfully"
}

function create_bootstrap_token() {
  log_info "Creating bootstrap token in garden cluster..."

  # Generate token ID and secret if not provided
  if [[ -z "$BOOTSTRAP_TOKEN_ID" ]]; then
    BOOTSTRAP_TOKEN_ID=$(generate_random_string 6)
    log_info "Generated token ID: $BOOTSTRAP_TOKEN_ID"
  fi

  if [[ -z "$BOOTSTRAP_TOKEN_SECRET" ]]; then
    BOOTSTRAP_TOKEN_SECRET=$(generate_random_string 16)
    log_info "Generated token secret: $BOOTSTRAP_TOKEN_SECRET"
  fi

  local token_name="bootstrap-token-${BOOTSTRAP_TOKEN_ID}"

  # Check if token already exists
  if kubectl --kubeconfig="$GARDEN_KUBECONFIG" -n kube-system get secret "$token_name" &> /dev/null; then
    log_warn "Bootstrap token secret '$token_name' already exists in garden cluster"
    read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
      kubectl --kubeconfig="$GARDEN_KUBECONFIG" -n kube-system delete secret "$token_name"
      log_info "Deleted existing bootstrap token"
    else
      log_info "Using existing bootstrap token"
      return
    fi
  fi

  # Create bootstrap token secret
  kubectl --kubeconfig="$GARDEN_KUBECONFIG" -n kube-system create secret generic "$token_name" \
    --type=bootstrap.kubernetes.io/token \
    --from-literal=description="Bootstrap token for gardenlet rebootstrap of seed ${SEED_NAME}" \
    --from-literal=token-id="$BOOTSTRAP_TOKEN_ID" \
    --from-literal=token-secret="$BOOTSTRAP_TOKEN_SECRET" \
    --from-literal=usage-bootstrap-authentication=true \
    --from-literal=usage-bootstrap-signing=true

  log_info "Bootstrap token created successfully: $token_name"
}

function get_garden_cluster_info() {
  log_info "Extracting garden cluster information..."

  # Extract server and CA from garden kubeconfig
  GARDEN_SERVER=$(kubectl --kubeconfig="$GARDEN_KUBECONFIG" config view --minify -o jsonpath='{.clusters[0].cluster.server}')
  GARDEN_CA=$(kubectl --kubeconfig="$GARDEN_KUBECONFIG" config view --minify --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

  if [[ -z "$GARDEN_SERVER" ]]; then
    log_error "Failed to extract server URL from garden kubeconfig"
    exit 1
  fi

  if [[ -z "$GARDEN_CA" ]]; then
    log_error "Failed to extract CA certificate from garden kubeconfig"
    exit 1
  fi

  log_info "Garden cluster server: $GARDEN_SERVER"
}

function create_bootstrap_kubeconfig_secret() {
  log_info "Creating bootstrap kubeconfig secret in seed cluster..."

  local bootstrap_token="${BOOTSTRAP_TOKEN_ID}.${BOOTSTRAP_TOKEN_SECRET}"
  local secret_name="gardenlet-bootstrap-kubeconfig"

  # Create bootstrap kubeconfig
  local bootstrap_kubeconfig=$(cat <<EOF
apiVersion: v1
kind: Config
current-context: gardenlet-bootstrap@default
clusters:
- cluster:
    certificate-authority-data: ${GARDEN_CA}
    server: ${GARDEN_SERVER}
  name: default
contexts:
- context:
    cluster: default
    user: gardenlet-bootstrap
  name: gardenlet-bootstrap@default
users:
- name: gardenlet-bootstrap
  user:
    token: ${bootstrap_token}
EOF
)

  # Check if secret already exists
  if kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get secret "$secret_name" &> /dev/null; then
    log_warn "Bootstrap kubeconfig secret '$secret_name' already exists in seed cluster"
    kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" delete secret "$secret_name"
    log_info "Deleted existing bootstrap kubeconfig secret"
  fi

  # Create secret with bootstrap kubeconfig
  kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" create secret generic "$secret_name" \
    --from-literal=kubeconfig="$bootstrap_kubeconfig"

  log_info "Bootstrap kubeconfig secret created successfully: $secret_name"
}

function update_gardenlet_configuration() {
  log_info "Updating gardenlet configuration..."

  # Get the gardenlet deployment
  if ! kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get deployment "$GARDENLET_DEPLOYMENT_NAME" &> /dev/null; then
    log_error "Gardenlet deployment '$GARDENLET_DEPLOYMENT_NAME' not found in namespace '$GARDENLET_NAMESPACE'"
    exit 1
  fi

  # Find the ConfigMap used by the gardenlet deployment (volume name: gardenlet-config)
  local configmap_name=$(kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get deployment "$GARDENLET_DEPLOYMENT_NAME" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="gardenlet-config")].configMap.name}')

  if [[ -z "$configmap_name" ]]; then
    log_error "Could not find ConfigMap referenced by gardenlet deployment volume 'gardenlet-config'"
    log_error "Please ensure the deployment has a volume named 'gardenlet-config' with a ConfigMap"
    exit 1
  fi

  log_info "Found ConfigMap: $configmap_name"

  # Get the current ConfigMap content and extract the config.yaml key (this is where the GardenletConfiguration is stored)
  local config_key="config\.yaml"
  local config_content=$(kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get configmap "$configmap_name" -o jsonpath="{.data['$config_key']}")

  if [[ -z "$config_content" ]]; then
    log_error "Could not find '$config_key' in ConfigMap '$configmap_name'"
    exit 1
  fi

  # Create a temporary file with the current config
  local temp_config=$(mktemp)
  echo "$config_content" > "$temp_config"

  # Update or add bootstrapKubeconfig configuration using yq
  log_info "Updating gardenlet configuration with bootstrap kubeconfig..."
  yq eval -i ".gardenClientConnection.bootstrapKubeconfig.name = \"gardenlet-bootstrap-kubeconfig\"" "$temp_config"
  yq eval -i ".gardenClientConnection.bootstrapKubeconfig.namespace = \"$GARDENLET_NAMESPACE\"" "$temp_config"

  # Generate a new ConfigMap name with timestamp to ensure uniqueness
  local timestamp=$(date +%s)
  local new_configmap_name="${configmap_name}-rebootstrap-${timestamp}"

  log_info "Creating new ConfigMap: $new_configmap_name"

  # Create the new ConfigMap with updated configuration
  kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" create configmap "$new_configmap_name" \
    --from-file="${config_key//\\/}=${temp_config}"

  # Clean up temp file
  rm -f "$temp_config"

  # Update the deployment to use the new ConfigMap
  log_info "Updating gardenlet deployment to use new ConfigMap..."

  # Get all volumes and update the gardenlet-config volume to reference the new ConfigMap
  local volumes_json
  volumes_json=$(kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get deployment "$GARDENLET_DEPLOYMENT_NAME" \
    -o yaml | yq eval '(.spec.template.spec.volumes[] | select(.name == "gardenlet-config") | .configMap.name) = "'"$new_configmap_name"'" | .spec.template.spec.volumes' -o json -)

  if [[ -z "$volumes_json" ]] || [[ "$volumes_json" == "null" ]]; then
    log_error "Failed to process volumes JSON"
    exit 1
  fi

  # Patch the deployment to reference the new ConfigMap
  kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" patch deployment "$GARDENLET_DEPLOYMENT_NAME" --type=json -p="[
    {
      \"op\": \"replace\",
      \"path\": \"/spec/template/spec/volumes\",
      \"value\": $volumes_json
    }
  ]"

  log_info "Gardenlet configuration updated successfully"
  log_info "Old ConfigMap: $configmap_name"
  log_info "New ConfigMap: $new_configmap_name"
  log_info "The old ConfigMap will be automatically removed by the garbage collector"
}

function delete_expired_kubeconfig() {
  log_info "Deleting expired kubeconfig secret..."

  local kubeconfig_secret_name="gardenlet-kubeconfig"

  if kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get secret "$kubeconfig_secret_name" &> /dev/null; then
    kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" delete secret "$kubeconfig_secret_name"
    log_info "Deleted expired kubeconfig secret: $kubeconfig_secret_name"
  else
    log_warn "Kubeconfig secret '$kubeconfig_secret_name' not found, skipping deletion"
  fi
}

function wait_for_gardenlet_rollout() {
  log_info "Waiting for gardenlet deployment rollout..."

  if kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get deployment "$GARDENLET_DEPLOYMENT_NAME" &> /dev/null; then
    log_info "The deployment will restart automatically due to the ConfigMap change"
    log_info "Waiting for rollout to complete..."
    kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" rollout status deployment "$GARDENLET_DEPLOYMENT_NAME" --timeout=5m
    log_info "Gardenlet deployment rollout completed successfully"
  else
    log_error "Gardenlet deployment '$GARDENLET_DEPLOYMENT_NAME' not found in namespace '$GARDENLET_NAMESPACE'"
    exit 1
  fi
}

function verify_bootstrap() {
  log_info "Verifying bootstrap success..."

  local kubeconfig_secret_name="gardenlet-kubeconfig"
  local bootstrap_secret_name="gardenlet-bootstrap-kubeconfig"
  local max_wait=300
  local elapsed=0
  local interval=10

  # Wait for new kubeconfig secret to be created
  log_info "Waiting for new kubeconfig secret to be created..."
  while [[ $elapsed -lt $max_wait ]]; do
    if kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get secret "$kubeconfig_secret_name" &> /dev/null; then
      log_info "✓ New kubeconfig secret created"
      break
    fi
    sleep $interval
    elapsed=$((elapsed + interval))

    # Check if bootstrap secret was deleted
    if kubectl --kubeconfig="$SEED_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get secret "$bootstrap_secret_name" &> /dev/null; then
      log_warn "⚠ Bootstrap secret still exists (it should be deleted automatically)"
    else
      log_info "✓ Bootstrap secret was deleted"
    fi
  done
  echo

  if [[ $elapsed -ge $max_wait ]]; then
    log_error "Timeout waiting for new kubeconfig secret to be created"
    log_error "Check gardenlet logs for errors:"
    log_error "  kubectl --kubeconfig=$SEED_KUBECONFIG -n $GARDENLET_NAMESPACE logs deployment/$GARDENLET_DEPLOYMENT_NAME"
    exit 1
  fi

  # Check seed status in garden cluster with retry logic
  log_info "Checking seed status in garden cluster..."

  if ! kubectl --kubeconfig="$GARDEN_KUBECONFIG" get seed "$SEED_NAME" &> /dev/null; then
    log_error "Seed resource '$SEED_NAME' not found in garden cluster"
    exit 1
  fi

  # Wait for gardenlet to become ready
  log_info "Waiting for gardenlet to report ready status..."
  local seed_max_wait=120  # 2 minutes
  local seed_elapsed=0
  local seed_interval=5

  while [[ $seed_elapsed -lt $seed_max_wait ]]; do
    local gardenlet_ready
    gardenlet_ready=$(kubectl --kubeconfig="$GARDEN_KUBECONFIG" get seed "$SEED_NAME" -o jsonpath='{.status.conditions[?(@.type=="GardenletReady")].status}' 2>/dev/null || echo "")

    if [[ "$gardenlet_ready" == "True" ]]; then
      log_info "✓ Seed is healthy and gardenlet is ready"
      break
    fi

    sleep $seed_interval
    seed_elapsed=$((seed_elapsed + seed_interval))
    echo -n "."
  done
  echo

  log_info "Deleting bootstrap token secret"
  kubectl --kubeconfig=$GARDEN_KUBECONFIG -n kube-system delete secret bootstrap-token-$BOOTSTRAP_TOKEN_ID --ignore-not-found

  if [[ $seed_elapsed -ge $seed_max_wait ]]; then
    log_warn "⚠ Timeout waiting for gardenlet to report ready status"
    log_warn "  The bootstrap may still be in progress. Check seed status manually:"
    log_warn "  kubectl --kubeconfig=$GARDEN_KUBECONFIG get seed $SEED_NAME -o yaml | yq eval .status.conditions"
  fi

  log_info ""
  log_info "=========================================="
  log_info "Bootstrap verification completed!"
  log_info "=========================================="
  log_info ""
  log_info "Next steps:"
  log_info "1. Monitor gardenlet logs:"
  log_info "   kubectl --kubeconfig=$SEED_KUBECONFIG -n $GARDENLET_NAMESPACE logs -f deployment/$GARDENLET_DEPLOYMENT_NAME"
  log_info ""
  log_info "2. Check seed conditions:"
  log_info "   kubectl --kubeconfig=$GARDEN_KUBECONFIG get seed $SEED_NAME -o yaml | yq eval .status.conditions"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --garden-kubeconfig)
      GARDEN_KUBECONFIG="$2"
      shift 2
      ;;
    --seed-kubeconfig)
      SEED_KUBECONFIG="$2"
      shift 2
      ;;
    --seed-name)
      SEED_NAME="$2"
      shift 2
      ;;
    --token-id)
      BOOTSTRAP_TOKEN_ID="$2"
      shift 2
      ;;
    --token-secret)
      BOOTSTRAP_TOKEN_SECRET="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      log_error "Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

# Main execution
log_info "=========================================="
log_info "Gardenlet Rebootstrap Script"
log_info "=========================================="
log_info ""

validate_requirements
create_bootstrap_token
get_garden_cluster_info
create_bootstrap_kubeconfig_secret
delete_expired_kubeconfig
update_gardenlet_configuration
wait_for_gardenlet_rollout
verify_bootstrap

log_info ""
log_info "=========================================="
log_info "Rebootstrap completed successfully!"
log_info "=========================================="
