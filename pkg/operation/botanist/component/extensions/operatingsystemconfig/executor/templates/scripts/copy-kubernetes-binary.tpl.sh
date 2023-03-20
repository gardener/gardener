#!/bin/bash -eu
# Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


BINARY="$1"

PATH_HYPERKUBE_DOWNLOADS="{{ .pathHyperkubeDownloads }}"
PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE="{{ .pathLastDownloadedHyperkubeImage }}"
PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY=""

if [[ "$BINARY" == "kubelet" ]]; then
  PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY="{{ .pathHyperKubeImageUsedForLastCopyKubelet }}"
elif [[ "$BINARY" == "kubectl" ]]; then
  PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY="{{ .pathHyperKubeImageUsedForLastCopyKubectl }}"
else
  echo "$BINARY cannot be handled. Only 'kubelet' and 'kubectl' are valid arguments."
  exit 1
fi

LAST_DOWNLOADED_HYPERKUBE_IMAGE=""
if [[ -f "$PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE" ]]; then
  LAST_DOWNLOADED_HYPERKUBE_IMAGE="$(cat "$PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE")"
fi

HYPERKUBE_IMAGE_USED_FOR_LAST_COPY=""
if [[ -f "$PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY" ]]; then
  HYPERKUBE_IMAGE_USED_FOR_LAST_COPY="$(cat "$PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY")"
fi

echo "Checking whether to copy new $BINARY binary from hyperkube image to {{ .pathBinaries }} folder..."
if [[ "$HYPERKUBE_IMAGE_USED_FOR_LAST_COPY" != "$LAST_DOWNLOADED_HYPERKUBE_IMAGE" ]]; then
  echo "$BINARY binary in {{ .pathBinaries }} is outdated (image used for last copy: $HYPERKUBE_IMAGE_USED_FOR_LAST_COPY). Need to update it to $LAST_DOWNLOADED_HYPERKUBE_IMAGE".
  rm -f "{{ .pathBinaries }}/$BINARY" &&
    cp "$PATH_HYPERKUBE_DOWNLOADS/$BINARY" "{{ .pathBinaries }}" &&
    echo "$LAST_DOWNLOADED_HYPERKUBE_IMAGE" > "$PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY"
else
  echo "No need to copy $BINARY binary from a new hyperkube image because binary found in $PATH_HYPERKUBE_DOWNLOADS is up-to-date."
fi
