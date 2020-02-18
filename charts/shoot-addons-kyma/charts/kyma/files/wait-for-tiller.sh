#!/bin/bash

until [[ $(kubectl get pod -l name=tiller -n kube-system -o jsonpath='{.items[*].status.containerStatuses[0].ready}') == "true" ]]
do
    echo "---> Tiller is not ready yet"
    sleep "${DELAY}"
done

echo "---> Tiller is ready"
