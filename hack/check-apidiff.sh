#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

tmpDir=$(mktemp -d)
function cleanup_output {
  rm -rf "$tmpDir"
}
trap cleanup_output EXIT

retval=0
temp=0

# PULL_BASE_SHA env variable is set by default in prow presubmit jobs
echo "invoking: go-apidiff ${PULL_BASE_SHA:-master} --repo-path=."
go-apidiff ${PULL_BASE_SHA:-master} --repo-path=. >${tmpDir}/output.txt || true

exported_pkg=(
  gardener/gardener/extensions/
  gardener/gardener/pkg/api/
  gardener/gardener/pkg/apis/.*/v1alpha1
  gardener/gardener/pkg/apis/.*/v1beta1
  gardener/gardener/pkg/apis/extensions/validation
  gardener/gardener/pkg/chartrenderer/
  gardener/gardener/pkg/client/
  gardener/gardener/pkg/controllerutils/
  gardener/gardener/pkg/extensions/
  gardener/gardener/pkg/gardenlet/apis/config/v1alpha1
  gardener/gardener/pkg/logger/
  gardener/gardener/third_party/mock/controller-runtime/client/
  gardener/gardener/pkg/component/extensions/operatingsystemconfig/
  gardener/gardener/pkg/operator/apis/config/v1alpha1
  gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references/
  gardener/gardener/pkg/scheduler/
  gardener/gardener/pkg/utils/
  gardener/gardener/test/framework/
)

# check the changes only for the package that is in the exported_pkg list
while IFS= read -r line; do
  if [[ $line =~ ^"github.com/gardener/gardener" ]]; then
    temp=0
    for x in ${exported_pkg[*]}; do
      if [[ $line =~ $x ]]; then
        retval=1
        temp=1
        echo "$line" >>${tmpDir}/result.txt
      fi
    done
  else
    if [[ $temp -eq 1 ]]; then
      echo "$line" >>${tmpDir}/result.txt
    fi
  fi
done <"${tmpDir}/output.txt"

if [[ $retval -eq 1 ]]; then
  echo >&2 "FAIL: contains compatible/incompatible changes:"
  cat ${tmpDir}/result.txt

  cat <<EOF

The apidiff check failed – don't worry!
This check is optional and hence not required to pass before merging a PR.

The apidiff check makes changes to the go packages' API surface visible.
For example, other repositories rely on a stable API of the extensions library located in gardener/gardener/extensions.
When introducing incompatible changes in the extensions library, those dependants need to adapt to these changes when upgrading their go dependencies.
To make this process easier for your fellow developers, try to keep incompatible changes limited.
For all other cases, please check all incompatible changes listed above and add a friendly release note about them to your PR.
Even better: you could explain how to adapt to the breaking changes in your PR's description or add a short doc.

If your PR only contains compatible API changes, no action is required. You're good to go!
EOF
fi

exit $retval
