#!/bin/bash
set -e

echo "---> Applying Tiller for ${KYMA_VERSION}"
kubectl apply -f https://raw.githubusercontent.com/kyma-project/kyma/$KYMA_VERSION/installation/resources/tiller.yaml
