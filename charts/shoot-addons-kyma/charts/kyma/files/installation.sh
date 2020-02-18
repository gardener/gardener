#!/bin/bash
set -e

if [ -z "$(kubectl -n kyma-installer get job -l app=kyma-initializer,version!=${KYMA_VERSION} --ignore-not-found)" ]; then
    echo "---> Apply Kyma-Installer for ${KYMA_VERSION}"
    kubectl apply -f "https://raw.githubusercontent.com/kyma-project/kyma/${KYMA_VERSION}/installation/resources/installer.yaml"

    echo "---> Apply Kyma installation CR for ${KYMA_VERSION}"
    kubectl apply -f "https://raw.githubusercontent.com/kyma-project/kyma/${KYMA_VERSION}/installation/resources/installer-cr-cluster.yaml.tpl"
else
    echo "---> Skip Kyma installation as there is already a Kyma installation"
fi
