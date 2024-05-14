#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

# IMAGE is set from skaffold, and looks like "localhost:5001/europe-docker_pkg_dev_gardener-project_releases_gardener_extensions_provider-local:v1.95.0-dev-2f85d6a4e-dirty"
# IMG looks the same and comes from requires.alias in the skaffold Config.

tag=${IMAGE##*:}
repository=${IMAGE%:*}
name=${repository##*/}
registry=${repository%/*}

tmp=$(mktemp -d)
trap 'rm -rf $tmp' EXIT

cp -r ./charts/gardener/provider-local "$tmp"
chartDir="$tmp/provider-local"

# set image - that way the pushed helm chart always contains the same image (name + tag) as the helm chart itself
yq -i ".image |= \"$IMG\"" "$chartDir/values.yaml"
# overrides the chart name. Skaffold replaces every "/" with "_" and expects an artifact to be pushed to that path.
# However, `helm push` always appends "/<chartName>" to the registry path conflicting with this. This is hacky, since
# Charts might use {{ .Chart.name }} in their templates, which will now look very different, and is e.g. no longer a
# valid dns name.
yq -i ".name |= \"$name\"" "$chartDir/Chart.yaml"
helm package "$chartDir" -d "$chartDir" --version "$tag"

helm push "$chartDir/$name-$tag.tgz" "oci://$registry"
