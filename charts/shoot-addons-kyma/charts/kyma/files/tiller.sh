#!/bin/bash
set -e
if [ -z "$(kubectl -n kyma-installer get job -l app=kyma-initializer,version!=${KYMA_VERSION} --ignore-not-found)" ]; then
    echo "---> Apply Tiller for ${KYMA_VERSION}"
    kubectl apply -f "https://raw.githubusercontent.com/kyma-project/kyma/${KYMA_VERSION}/installation/resources/tiller.yaml"
else
    echo "---> Skip Tiller installation as there is already a Kyma installation"
fi
