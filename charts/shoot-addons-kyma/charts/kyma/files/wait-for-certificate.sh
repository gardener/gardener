#!/bin/bash

until kubectl get cm owndomain-overrides -n kyma-installer
do
    echo "---> Certificate is not ready yet"
    sleep $DELAY
done
