#!/bin/bash
set -e

echo "---> Validate requirements"
SHOOT_DOMAIN="$(kubectl -n kube-system get configmap shoot-info -o jsonpath='{.data.domain}')"

EXPECTED_MAJOR=$(echo ${K8S_VERSION} | cut -d '.' -f 2)
EXPECTED_MINOR=$(echo ${K8S_VERSION} | cut -d '.' -f 3)

SHOOT_K8S_VERSION=$(kubectl -n kube-system get configmap shoot-info -o jsonpath='{.data.kubernetesVersion}')
SHOOT_MAJOR=$(echo ${SHOOT_K8S_VERSION} | cut -d '.' -f 2)
SHOOT_MINOR=$(echo ${SHOOT_K8S_VERSION} | cut -d '.' -f 3)

EXPECTED_EXTENSIONS=($(echo ${GARDENER_EXTENSIONS} | tr "," "\n"))
SHOOT_EXTENSIONS=$(kubectl -n kube-system get configmap shoot-info -o jsonpath='{.data.extensions}')

if [[ $SHOOT_MAJOR -gt EXPECTED_MAJOR ]]; then
  echo "Unsupported Kubernetes version $SHOOT_K8S_VERSION, should be less or equal $K8S_VERSION"
  exit 1
fi

for i in "${EXPECTED_EXTENSIONS[@]}"
do
  if [[ $SHOOT_EXTENSIONS != *"${i}"* ]]; then
    echo "Required Gardener extension $i not found in installed extensions $SHOOT_EXTENSIONS"
    exit 1
  fi
done

if [[ -z $SHOOT_DOMAIN ]]; then
  echo "Cannot gather domain name from shoot-info configmap"
  exit 1
fi
echo "---> All requirements fine"
