#!/bin/bash -eu

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
