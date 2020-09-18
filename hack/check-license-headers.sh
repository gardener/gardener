#!/bin/bash
#
# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# Checks if license headers comply with REUSE rules
# and are present in all source code files.
# Some files are excluded since they were copied from Kubernetes
# and therefore have a different license header.

function exclude {
  grep -v \
    -e "^charts/" \
    -e "^docs/" \
    -e "^LICENSES/" \
    -e "^.reuse/" \
    -e "^vendor/" \
    -e "^CODEOWNERS$" \
    -e "^go.mod$" \
    -e "^go.sum$" \
    -e "^pkg/chartrenderer/sorter.go$" \
    -e "^pkg/controllermanager/controller/certificatesigningrequest/csr_autoapprove_control.go$" \
    -e "^pkg/gardenlet/bootstrap/certificate/certificate_util.go$" \
    -e "^pkg/operation/botanist/matchers/rule_matcher.go$" \
    -e "^test/framework/cleanup.go$" \
    -e "^VERSION$" \
    -e "^.*ignore$" \
    -e ".conf$" \
    -e ".crt$" \
    -e ".json$" \
    -e ".key$" \
    -e ".md$" \
    -e ".png$" \
    -e ".pub$" \
    -e ".report$" \
    -e ".svg$" \
    -e ".tpl$" \
    -e ".txt$" \
    -e ".yaml$" < /dev/stdin
}

function check {
  git grep -EL "$1" | exclude
}

fail=false

echo "checking copyright headers ..." >>/dev/stderr
copyright_missing=$(check "[/#* ]*SPDX-FileCopyrightText: 20[0-9][0-9] SAP SE or an SAP affiliate company and Gardener contributors$")
if [ -z "$copyright_missing" ]
then
  echo "copyright headers are ok"; >>/dev/stderr
else
  echo "copyright header missing in:" >>/dev/stderr
  echo "$copyright_missing"
  fail=true
fi

echo "checking license headers ..." >>/dev/stderr
license_missing=$(check "[/#* ]*SPDX-License-Identifier: Apache-2.0$")
if [ -z "$license_missing" ]
then
  echo "license headers are ok"; >>/dev/stderr
else
  echo "license header missing in:" >>/dev/stderr
  echo "$license_missing"
  fail=true
fi

if [[ "$fail" ]]
then
  exit 1
fi