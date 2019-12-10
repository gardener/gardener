#!/bin/bash

until [[ $STATUS == "Installed" ]]; do
    STATUS="$(kubectl -n default get installation/kyma-installation -o jsonpath='{.status.state}')"
    DESCRIPTION="$(kubectl -n default get installation/kyma-installation -o jsonpath='{.status.description}')"
    echo "---> Waiting for Kyma-Installer. Status: $STATUS, Description: $DESCRIPTION"
    sleep $DELAY
done
