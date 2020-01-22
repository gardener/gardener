#!/bin/bash
set -e

echo "---> Applying Kyma-Installer for ${KYMA_VERSION}"
kubectl apply -f https://raw.githubusercontent.com/kyma-project/kyma/$KYMA_VERSION/installation/resources/installer.yaml

echo "---> Apply Kyma installation CR for ${KYMA_VERSION}"
kubectl -n default apply -f https://raw.githubusercontent.com/kyma-project/kyma/$KYMA_VERSION/installation/resources/installer-cr-cluster.yaml.tpl
