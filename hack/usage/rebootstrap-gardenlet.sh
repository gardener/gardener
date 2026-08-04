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
SHOOT_KUBECONFIG="${SHOOT_KUBECONFIG:-}"
SHOOT_NAMESPACE="${SHOOT_NAMESPACE:-}"
SHOOT_NAME="${SHOOT_NAME:-}"
BOOTSTRAP_TOKEN_ID=""
BOOTSTRAP_TOKEN_SECRET=""
GARDENLET_NAMESPACE=""
GARDENLET_DEPLOYMENT_NAME="gardenlet"
BOOTSTRAP_KUBECONFIG_SECRET_NAME=""
MODE="" # "seed" or "shoot"

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

Required Options (seed gardenlet):
  --garden-kubeconfig PATH    Path to garden cluster kubeconfig
  --seed-kubeconfig PATH      Path to seed cluster kubeconfig
  --seed-name NAME            Name of the seed

Required Options (shoot gardenlet):
  --garden-kubeconfig PATH    Path to garden cluster kubeconfig
  --shoot-kubeconfig PATH     Path to shoot cluster kubeconfig
  --shoot-namespace NAMESPACE Namespace of the shoot in the garden cluster
  --shoot-name NAME           Name of the shoot

Note: --seed-* and --shoot-* options are mutually exclusive.

Optional:
  --token-id ID               Bootstrap token ID (6 characters, random if not provided)
  --token-secret SECRET       Bootstrap token secret (16 characters, random if not provided)
  -h, --help                  Show this help message

Examples:
  $0 --garden-kubeconfig ~/.kube/garden.yaml --seed-kubeconfig ~/.kube/seed.yaml --seed-name my-seed
  $0 --garden-kubeconfig ~/.kube/garden.yaml --shoot-kubeconfig ~/.kube/shoot.yaml --shoot-namespace garden --shoot-name my-shoot

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

  # Determine mode and validate mode-specific options
  local has_seed=false
  local has_shoot=false
  [[ -n "$SEED_KUBECONFIG" || -n "$SEED_NAME" ]] && has_seed=true
  [[ -n "$SHOOT_KUBECONFIG" || -n "$SHOOT_NAMESPACE" || -n "$SHOOT_NAME" ]] && has_shoot=true

  if [[ "$has_seed" == "true" && "$has_shoot" == "true" ]]; then
    log_error "--seed-* and --shoot-* options are mutually exclusive."
    exit 1
  fi

  if [[ "$has_seed" == "false" && "$has_shoot" == "false" ]]; then
    log_error "Either --seed-kubeconfig/--seed-name or --shoot-kubeconfig/--shoot-namespace/--shoot-name must be provided."
    exit 1
  fi

  if [[ "$has_seed" == "true" ]]; then
    MODE="seed"

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

    GARDENLET_NAMESPACE="garden"
    BOOTSTRAP_KUBECONFIG_SECRET_NAME="gardenlet-bootstrap-kubeconfig"
    TARGET_KUBECONFIG="$SEED_KUBECONFIG"
  else
    MODE="shoot"

    if [[ -z "$SHOOT_KUBECONFIG" ]]; then
      log_error "Shoot kubeconfig is required. Use --shoot-kubeconfig option."
      exit 1
    fi

    if [[ ! -f "$SHOOT_KUBECONFIG" ]]; then
      log_error "Shoot kubeconfig file not found: $SHOOT_KUBECONFIG"
      exit 1
    fi

    if [[ -z "$SHOOT_NAMESPACE" ]]; then
      log_error "Shoot namespace is required. Use --shoot-namespace option."
      exit 1
    fi

    if [[ -z "$SHOOT_NAME" ]]; then
      log_error "Shoot name is required. Use --shoot-name option."
      exit 1
    fi

    GARDENLET_NAMESPACE="kube-system"
    BOOTSTRAP_KUBECONFIG_SECRET_NAME="gardenlet-kubeconfig-bootstrap"
    TARGET_KUBECONFIG="$SHOOT_KUBECONFIG"
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

  log_info "All requirements validated successfully (mode: $MODE)"
}

function create_bootstrap_token() {
  log_info "Creating bootstrap token in garden cluster..."

  if [[ -z "$BOOTSTRAP_TOKEN_ID" ]]; then
    BOOTSTRAP_TOKEN_ID=$(generate_random_string 6)
    log_info "Generated token ID: $BOOTSTRAP_TOKEN_ID"
  fi

  if [[ -z "$BOOTSTRAP_TOKEN_SECRET" ]]; then
    BOOTSTRAP_TOKEN_SECRET=$(generate_random_string 16)
    log_info "Generated token secret: $BOOTSTRAP_TOKEN_SECRET"
  fi

  local token_name="bootstrap-token-${BOOTSTRAP_TOKEN_ID}"
  local description
  if [[ "$MODE" == "seed" ]]; then
    description="Bootstrap token for gardenlet rebootstrap of seed ${SEED_NAME}"
  else
    description="Used for connecting the self-hosted Shoot ${SHOOT_NAMESPACE}/${SHOOT_NAME}"
  fi

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

  kubectl --kubeconfig="$GARDEN_KUBECONFIG" -n kube-system create secret generic "$token_name" \
    --type=bootstrap.kubernetes.io/token \
    --from-literal=description="$description" \
    --from-literal=token-id="$BOOTSTRAP_TOKEN_ID" \
    --from-literal=token-secret="$BOOTSTRAP_TOKEN_SECRET" \
    --from-literal=usage-bootstrap-authentication=true \
    --from-literal=usage-bootstrap-signing=true

  log_info "Bootstrap token created successfully: $token_name"
}

function get_garden_cluster_info() {
  log_info "Extracting garden cluster information..."

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
  log_info "Creating bootstrap kubeconfig secret in target cluster..."

  local bootstrap_token="${BOOTSTRAP_TOKEN_ID}.${BOOTSTRAP_TOKEN_SECRET}"

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

  if kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get secret "$BOOTSTRAP_KUBECONFIG_SECRET_NAME" &> /dev/null; then
    log_warn "Bootstrap kubeconfig secret '$BOOTSTRAP_KUBECONFIG_SECRET_NAME' already exists in target cluster"
    kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" delete secret "$BOOTSTRAP_KUBECONFIG_SECRET_NAME"
    log_info "Deleted existing bootstrap kubeconfig secret"
  fi

  kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" create secret generic "$BOOTSTRAP_KUBECONFIG_SECRET_NAME" \
    --from-literal=kubeconfig="$bootstrap_kubeconfig"

  log_info "Bootstrap kubeconfig secret created successfully: $BOOTSTRAP_KUBECONFIG_SECRET_NAME"
}

function update_gardenlet_configuration() {
  log_info "Updating gardenlet configuration..."

  if ! kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get deployment "$GARDENLET_DEPLOYMENT_NAME" &> /dev/null; then
    log_error "Gardenlet deployment '$GARDENLET_DEPLOYMENT_NAME' not found in namespace '$GARDENLET_NAMESPACE'"
    exit 1
  fi

  local configmap_name=$(kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get deployment "$GARDENLET_DEPLOYMENT_NAME" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="gardenlet-config")].configMap.name}')

  if [[ -z "$configmap_name" ]]; then
    log_error "Could not find ConfigMap referenced by gardenlet deployment volume 'gardenlet-config'"
    log_error "Please ensure the deployment has a volume named 'gardenlet-config' with a ConfigMap"
    exit 1
  fi

  log_info "Found ConfigMap: $configmap_name"

  local config_key="config\.yaml"
  local config_content=$(kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get configmap "$configmap_name" -o jsonpath="{.data['$config_key']}")

  if [[ -z "$config_content" ]]; then
    log_error "Could not find '$config_key' in ConfigMap '$configmap_name'"
    exit 1
  fi

  local temp_config=$(mktemp)
  echo "$config_content" > "$temp_config"

  log_info "Updating gardenlet configuration with bootstrap kubeconfig..."
  yq eval -i ".gardenClientConnection.bootstrapKubeconfig.name = \"$BOOTSTRAP_KUBECONFIG_SECRET_NAME\"" "$temp_config"
  yq eval -i ".gardenClientConnection.bootstrapKubeconfig.namespace = \"$GARDENLET_NAMESPACE\"" "$temp_config"

  local timestamp=$(date +%s)
  local new_configmap_name="${configmap_name}-rebootstrap-${timestamp}"

  log_info "Creating new ConfigMap: $new_configmap_name"

  kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" create configmap "$new_configmap_name" \
    --from-file="${config_key//\\/}=${temp_config}"

  rm -f "$temp_config"

  log_info "Updating gardenlet deployment to use new ConfigMap..."

  local volumes_json
  volumes_json=$(kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get deployment "$GARDENLET_DEPLOYMENT_NAME" \
    -o yaml | yq eval '(.spec.template.spec.volumes[] | select(.name == "gardenlet-config") | .configMap.name) = "'"$new_configmap_name"'" | .spec.template.spec.volumes' -o json -)

  if [[ -z "$volumes_json" ]] || [[ "$volumes_json" == "null" ]]; then
    log_error "Failed to process volumes JSON"
    exit 1
  fi

  kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" patch deployment "$GARDENLET_DEPLOYMENT_NAME" --type=json -p="[
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

  if kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get secret "$kubeconfig_secret_name" &> /dev/null; then
    kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" delete secret "$kubeconfig_secret_name"
    log_info "Deleted expired kubeconfig secret: $kubeconfig_secret_name"
  else
    log_warn "Kubeconfig secret '$kubeconfig_secret_name' not found, skipping deletion"
  fi
}

function wait_for_gardenlet_rollout() {
  log_info "Waiting for gardenlet deployment rollout..."

  if kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get deployment "$GARDENLET_DEPLOYMENT_NAME" &> /dev/null; then
    log_info "The deployment will restart automatically due to the ConfigMap change"
    log_info "Waiting for rollout to complete..."
    kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" rollout status deployment "$GARDENLET_DEPLOYMENT_NAME" --timeout=5m
    log_info "Gardenlet deployment rollout completed successfully"
  else
    log_error "Gardenlet deployment '$GARDENLET_DEPLOYMENT_NAME' not found in namespace '$GARDENLET_NAMESPACE'"
    exit 1
  fi
}

function verify_bootstrap() {
  log_info "Verifying bootstrap success..."

  local kubeconfig_secret_name="gardenlet-kubeconfig"
  local max_wait=300
  local elapsed=0
  local interval=10

  log_info "Waiting for new kubeconfig secret to be created..."
  while [[ $elapsed -lt $max_wait ]]; do
    if kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get secret "$kubeconfig_secret_name" &> /dev/null; then
      log_info "✓ New kubeconfig secret created"
      break
    fi
    sleep $interval
    elapsed=$((elapsed + interval))

    if kubectl --kubeconfig="$TARGET_KUBECONFIG" -n "$GARDENLET_NAMESPACE" get secret "$BOOTSTRAP_KUBECONFIG_SECRET_NAME" &> /dev/null; then
      log_warn "⚠ Bootstrap secret still exists (it should be deleted automatically)"
    else
      log_info "✓ Bootstrap secret was deleted"
    fi
  done
  echo

  if [[ $elapsed -ge $max_wait ]]; then
    log_error "Timeout waiting for new kubeconfig secret to be created"
    log_error "Check gardenlet logs for errors:"
    log_error "  kubectl --kubeconfig=$TARGET_KUBECONFIG -n $GARDENLET_NAMESPACE logs deployment/$GARDENLET_DEPLOYMENT_NAME"
    exit 1
  fi

  local seed_max_wait=120
  local seed_elapsed=0
  local seed_interval=5

  if [[ "$MODE" == "seed" ]]; then
    log_info "Checking seed status in garden cluster..."

    if ! kubectl --kubeconfig="$GARDEN_KUBECONFIG" get seed "$SEED_NAME" &> /dev/null; then
      log_error "Seed resource '$SEED_NAME' not found in garden cluster"
      exit 1
    fi

    log_info "Waiting for gardenlet to report ready status..."
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
  else
    log_info "Checking shoot status in garden cluster..."

    if ! kubectl --kubeconfig="$GARDEN_KUBECONFIG" -n "$SHOOT_NAMESPACE" get shoot "$SHOOT_NAME" &> /dev/null; then
      log_error "Shoot resource '$SHOOT_NAMESPACE/$SHOOT_NAME' not found in garden cluster"
      exit 1
    fi

    log_info "Waiting for gardenlet to report ready status..."
    while [[ $seed_elapsed -lt $seed_max_wait ]]; do
      local gardenlet_ready
      gardenlet_ready=$(kubectl --kubeconfig="$GARDEN_KUBECONFIG" -n "$SHOOT_NAMESPACE" get shoot "$SHOOT_NAME" -o jsonpath='{.status.conditions[?(@.type=="GardenletReady")].status}' 2>/dev/null || echo "")

      if [[ "$gardenlet_ready" == "True" ]]; then
        log_info "✓ Shoot gardenlet is ready"
        break
      fi

      sleep $seed_interval
      seed_elapsed=$((seed_elapsed + seed_interval))
      echo -n "."
    done
    echo
  fi

  log_info "Deleting bootstrap token secret"
  kubectl --kubeconfig=$GARDEN_KUBECONFIG -n kube-system delete secret bootstrap-token-$BOOTSTRAP_TOKEN_ID --ignore-not-found

  if [[ $seed_elapsed -ge $seed_max_wait ]]; then
    log_warn "⚠ Timeout waiting for gardenlet to report ready status"
    if [[ "$MODE" == "seed" ]]; then
      log_warn "  The bootstrap may still be in progress. Check seed status manually:"
      log_warn "  kubectl --kubeconfig=$GARDEN_KUBECONFIG get seed $SEED_NAME -o yaml | yq eval .status.conditions"
    else
      log_warn "  The bootstrap may still be in progress. Check shoot status manually:"
      log_warn "  kubectl --kubeconfig=$GARDEN_KUBECONFIG -n $SHOOT_NAMESPACE get shoot $SHOOT_NAME -o yaml | yq eval .status.conditions"
    fi
  fi

  log_info ""
  log_info "=========================================="
  log_info "Bootstrap verification completed!"
  log_info "=========================================="
  log_info ""
  log_info "Next steps:"
  log_info "1. Monitor gardenlet logs:"
  log_info "   kubectl --kubeconfig=$TARGET_KUBECONFIG -n $GARDENLET_NAMESPACE logs -f deployment/$GARDENLET_DEPLOYMENT_NAME"
  log_info ""
  if [[ "$MODE" == "seed" ]]; then
    log_info "2. Check seed conditions:"
    log_info "   kubectl --kubeconfig=$GARDEN_KUBECONFIG get seed $SEED_NAME -o yaml | yq eval .status.conditions"
  else
    log_info "2. Check shoot conditions:"
    log_info "   kubectl --kubeconfig=$GARDEN_KUBECONFIG -n $SHOOT_NAMESPACE get shoot $SHOOT_NAME -o yaml | yq eval .status.conditions"
  fi
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
    --shoot-kubeconfig)
      SHOOT_KUBECONFIG="$2"
      shift 2
      ;;
    --shoot-namespace)
      SHOOT_NAMESPACE="$2"
      shift 2
      ;;
    --shoot-name)
      SHOOT_NAME="$2"
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
