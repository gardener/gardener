#!/usr/bin/env bash
#
# Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

echo "> Check Helm charts"

if [[ -d "$1" ]]; then
  echo "Checking for chart symlink errors"
  BROKEN_SYMLINKS=$(find -L $1 -type l)
  if [[ "$BROKEN_SYMLINKS" ]]; then
    echo "Found broken symlinks:"
    echo "$BROKEN_SYMLINKS"
    exit 1
  fi
  echo "Checking whether all charts can be rendered"
  for chart_dir in $(find charts -type d -exec test -f '{}'/Chart.yaml \;  -print -prune | sort); do
    [ -f "$chart_dir/values-test.yaml" ] && values_files="-f $chart_dir/values-test.yaml" || unset values_files
    helm template $values_files "$chart_dir" 1> /dev/null
  done
fi

echo "All checks successful"
