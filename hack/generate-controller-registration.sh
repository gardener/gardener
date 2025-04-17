#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

function usage {
    cat <<EOM
Usage:
generate-controller-registration [options] <name> <chart-dir> <version> <dest> <kind-and-type> [kinds-and-types ...]

    -h, --help        Display this help and exit.
    -m, --auto-enable-modes[=shoot,seed]
                      Sets the auto-enable mode for the controller registration. Possible values are:
                      'shoot' and 'seed'
    -e, --pod-security-enforce[=pod-security-standard]
                      Sets 'security.gardener.cloud/pod-security-enforce' annotation in the
                      controller registration. Defaults to 'baseline'.
    -i, --inject-garden-kubeconfig
                      Sets 'injectGardenKubeconfig: true' for the controller deployment.
    <name>            Name of the controller registration to generate.
    <chart-dir>       Location of the chart directory.
    <version>         Version to use for the Helm chart and the tag in the ControllerDeployment.
    <dest>            The destination file to write the registration YAML to.
    <kind-and-type>   A tuple of kind and type of the controller registration to generate.
                      Separated by ':'.
                      Example: OperatingSystemConfig:foobar
    <kinds-and-types> Further tuples of kind and type of the controller registration to generate.
                      Separated by ':'.
EOM
    exit 0
}

POD_SECURITY_ENFORCE="baseline"
INJECT_GARDEN_KUBECONFIG=false
AUTO_ENABLE_MODES=""

while :; do
  case $1 in
    -h|--help)
      usage
      ;;
    -m|--auto-enable-modes)
      AUTO_ENABLE_MODES=$2
      shift
      ;;
    --auto-enable-modes=*)
      AUTO_ENABLE_MODES=${1#*=}
      ;;
    -e|--pod-security-enforce)
      POD_SECURITY_ENFORCE=$2
      shift
      ;;
    --pod-security-enforce=*)
      POD_SECURITY_ENFORCE=${1#*=}
      ;;
    -i|--inject-garden-kubeconfig)
      INJECT_GARDEN_KUBECONFIG=true
      ;;
    --)
      shift
      break
      ;;
    *)
      break
  esac
  shift
done

IFS=', ' read -r -a AUTO_ENABLE_MODES_ARRAY <<< "$AUTO_ENABLE_MODES"
for mode in "${AUTO_ENABLE_MODES_ARRAY[@]}"; do
  case $mode in
    shoot|seed)
      ;;
    *)
      echo "Invalid auto-enable mode: $mode"
      usage
      ;;
  esac
done

NAME="$1"
CHART_DIR="$2"
VERSION="$3"
DEST="$4"
KIND_AND_TYPE="$5"

( [[ -z "$NAME" ]] || [[ -z "$CHART_DIR" ]] || [[ -z "$DEST" ]] || [[ -z "$KIND_AND_TYPE" ]]) && usage

KINDS_AND_TYPES=("$KIND_AND_TYPE" "${@:6}")

# The following code is to make `helm package` idempotent: Usually, everytime `helm package` is invoked,
# it produces a different `.tgz` due to modification timestamps and some special shasums of gzip. We
# resolve this by unarchiving the `.tgz`, compressing it again with a constant `mtime` and no gzip
# checksums.
temp_dir="$(mktemp -d)"
temp_helm_home="$(mktemp -d)"
temp_extract_dir="$(mktemp -d)"
function cleanup {
    rm -rf "$temp_dir"
    rm -rf "$temp_helm_home"
    rm -rf "$temp_extract_dir"
}
trap cleanup EXIT ERR INT TERM

export HELM_HOME="$temp_helm_home"
[ "$(helm version --client --template "{{.Version}}" | head -c2 | tail -c1)" = "3" ] || helm init --client-only > /dev/null 2>&1
helm package "$CHART_DIR" --destination "$temp_dir" > /dev/null
tar -xzm -C "$temp_extract_dir" -f "$temp_dir"/*
chart="$(tar --sort=name -c --owner=root:0 --group=root:0 --mtime='UTC 2019-01-01' -C "$temp_extract_dir" "$(basename "$temp_extract_dir"/*)" | gzip -n | base64 | tr -d '\n')"

mkdir -p "$(dirname "$DEST")"

cat <<EOM > "$DEST"
---
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: $NAME
helm:
  rawChart: $chart
  values:
EOM

if [ -n "$(yq '.image.repository' "$CHART_DIR"/values.yaml)" ] ; then
  # image value specifies repository and tag separately, output the image stanza with the given version as tag value
  cat <<EOM >> "$DEST"
    image:
      tag: $VERSION
EOM
else
  # image value specifies a fully-qualified image reference, output the default image plus the given version as tag
  default_image="$(yq '.image' "$CHART_DIR"/values.yaml)"
  if [ -n "$VERSION" ] ; then
    # if a version is given, replace the default tag
    default_image="${default_image%%:*}:$VERSION"
  fi

  cat <<EOM >> "$DEST"
    image: $default_image
EOM
fi

if [ "${INJECT_GARDEN_KUBECONFIG}" = true ]; then
  cat <<EOM >> "$DEST"
injectGardenKubeconfig: true
EOM
fi

cat <<EOM >> "$DEST"
---
apiVersion: core.gardener.cloud/v1beta1
kind: ControllerRegistration
metadata:
  name: $NAME
  annotations:
    security.gardener.cloud/pod-security-enforce: $POD_SECURITY_ENFORCE
spec:
  deployment:
    deploymentRefs:
    - name: $NAME
  resources:
EOM

MODE_STRING=""
if [[ -n $AUTO_ENABLE_MODES ]]; then
  MODE_STRING=$(printf "autoEnable: [%s]" "${AUTO_ENABLE_MODES}")
fi

for kind_and_type in "${KINDS_AND_TYPES[@]}"; do
  KIND="$(echo "$kind_and_type" | cut -d ':' -f 1)"
  TYPE="$(echo "$kind_and_type" | cut -d ':' -f 2)"
  cat <<EOM >> "$DEST"
  - kind: $KIND
    type: $TYPE
    $MODE_STRING
EOM
done

echo "Successfully generated controller registration at $DEST"
