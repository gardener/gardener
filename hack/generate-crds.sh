#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

# Usage:
# generate-crds.sh [<flags>] <group> [<group> ...]
#     Generate manifests for all CRDs to the current working directory.
#     Useful for development purposes.
#
#     -p <file-name-prefix>               File name prefix for manifest files (e.g. '10-crd-')
#     -l (Optional)                       If this argument is given then the generated CRDs will have label gardener.cloud/deletion-protected: "true"
#     -k (Optional)                       If this argument is given then the generated CRDs will have annotation resources.gardener.cloud/keep-object: "true"
#     --allow-dangerous-types (Optional)  If this argument is given then the CRD generation will tolerate issues related to dangerous types.
#     --custom-package <group>=<package>  If this argument is given it supports generation for a package not listed explicitly, i.e. in another project reusing this script.
#     <group>                             List of groups to generate (generate all if unset)

if ! command -v controller-gen &> /dev/null ; then
  >&2 echo "controller-gen not available"
  exit 1
fi

output_dir="$(pwd)"
output_dir_temp="$(mktemp -d)"
add_deletion_protection_label=false
add_keep_object_annotation=false
crd_options=""
declare -A custom_packages=()

get_group_package () {
  if [[ -v custom_packages["$1"] ]]; then
    echo "${custom_packages["$1"]}"
    return
  fi

  case "$1" in
  "extensions.gardener.cloud")
    echo "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
    ;;
  "resources.gardener.cloud")
    echo "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
    ;;
  "operator.gardener.cloud")
    echo "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
    ;;
  "fluentbit.fluent.io")
    echo "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
    ;;
  "monitoring.coreos.com_v1")
    echo "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
    ;;
  "monitoring.coreos.com_v1beta1")
    echo "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1beta1"
    ;;
  "monitoring.coreos.com_v1alpha1")
    echo "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
    ;;
  "perses.dev_v1alpha1")
    echo "github.com/perses/perses-operator/api/v1alpha1"
    ;;
  "autoscaling.k8s.io")
    echo "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
    ;;
  "machine.sapcloud.io")
    echo "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
    ;;
  "cert.gardener.cloud")
    echo "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
    ;;
  "dashboard.gardener.cloud")
    echo "github.com/gardener/terminal-controller-manager/api/v1alpha1"
    ;;
  "opentelemetry.io_v1alpha1")
    echo "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
    ;;
  "opentelemetry.io_v1beta1")
    echo "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
    ;;
  *)
    >&2 echo "unknown group $1"
    return 1
  esac
}


generate_all_groups () {
  generate_group extensions.gardener.cloud
  generate_group resources.gardener.cloud
  generate_group operator.gardener.cloud
  generate_group autoscaling.k8s.io
  generate_group fluentbit.fluent.io
  generate_group monitoring.coreos.com_v1
  generate_group monitoring.coreos.com_v1beta1
  generate_group monitoring.coreos.com_v1alpha1
  generate_group perses.dev_v1alpha1
  generate_group machine.sapcloud.io
  generate_group dashboard.gardener.cloud
  generate_group opentelemetry.io
}

generate_group () {
  local group="$1"
  echo "Generating CRDs for $group group"

  local package="$(get_group_package "$group")"
  if [ -z "$package" ] ; then
    exit 1
  fi
  local package_path="$(go list -f '{{ .Dir }}' "$package")"
  if [ -z "$package_path" ] ; then
    exit 1
  fi

  generate="controller-gen crd"$crd_options" paths="$package_path" output:crd:dir="$output_dir_temp" output:stdout"

  if [[ "$group" == "autoscaling.k8s.io" ]]; then
    # See https://github.com/kubernetes/autoscaler/blame/master/vertical-pod-autoscaler/hack/generate-crd-yaml.sh#L43-L45
    generator_output="$(mktemp -d)/controller-gen.log"
    # As go list does not work with symlinks we need to manually construct the package paths to correctly
    # generate v1beta2 CRDs.
    package_path="${package_path};${package_path}beta2;"
    generate="controller-gen crd"$crd_options" paths="$package_path" output:crd:dir="$output_dir_temp" output:stdout"
    $generate &> "$generator_output" ||:
    grep -v -e 'map keys must be strings, not int' -e 'not all generators ran successfully' -e 'usage' "$generator_output" && { echo "Failed to generate CRD YAMLs."; exit 1; }
  elif [[ "$group" == "perses.dev_v1alpha1" ]]; then
    generate="controller-gen crd:ignoreUnexportedFields=true"$crd_options" paths="$package_path" output:crd:dir="$output_dir_temp" output:stdout"
    $generate
  else
    $generate
  fi

  local relevant_files=("$@")

  sanitized_group_name="${group%%_*}"

  while IFS= read -r crd; do
    crd_out="$output_dir/$file_name_prefix$(basename $crd)"
    mv "$crd" "$crd_out"
    relevant_files+=("$(basename "$crd_out")")

    if $add_deletion_protection_label; then
      if grep -q "clusters.extensions.gardener.cloud"  "$crd_out"; then
        :
      else
        sed -i '4 a\  labels:\n\    gardener.cloud/deletion-protected: "true"' "$crd_out"
      fi
    fi

    if $add_keep_object_annotation; then
      sed -i '/^  annotations:.*/a\    resources.gardener.cloud/keep-object: "true"' "$crd_out"
    fi
  done < <(ls "$output_dir_temp/$sanitized_group_name"_*.yaml)

  # garbage collection - clean all generated files for this group to account for changed prefix or removed resources
  local pattern=".*${group}_.*\.yaml"
  if [[ "$group" == "autoscaling.k8s.io" ]]; then
    pattern=".*${group}_v.*\.yaml"
  fi

  while IFS= read -r file; do
    file_name=$(basename "$file")
    delete_no_longer_needed_file=true

    for relevant_file_name in "${relevant_files[@]}"; do
      if [[ $file_name == "$relevant_file_name" ]] || [[ ! $file_name =~ $pattern ]]; then
        delete_no_longer_needed_file=false
        break
      fi
    done

    if $delete_no_longer_needed_file; then
      rm "$file"
    fi
  done < <(ls "$output_dir")
}

parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    -p)
      file_name_prefix="$2"
      shift
      shift
      ;;
    -l)
      add_deletion_protection_label=true
      shift
      ;;
    -k)
      add_keep_object_annotation=true
      shift
      ;;
    --allow-dangerous-types)
      crd_options=":allowDangerousTypes=true"
      shift
      ;;
   --custom-package)
      if [[ "$2" =~ ^[^=]+=[^=]+$ ]]; then
        IFS='=' read -r group package <<< "$2"
        custom_packages["$group"]="$package"
        shift
      else
        >&2 echo "Invalid format for --custom-package. Expected <group>=<package>"
        exit 1
      fi
      shift
      ;;
    *)
      args+=("$1")
      shift
      ;;
    esac
  done
}

parse_flags "$@"

if [ -n "$args" ]; then
  for group in "${args[@]}"; do
    generate_group "$group"
  done
else
  generate_all_groups
fi
