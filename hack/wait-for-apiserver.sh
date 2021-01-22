#!/usr/bin/env bash

# wait until gardner apiserver has sucessfully connected and shoot resources
# are available

echo -n "waiting for gardener apiserver to register: "
SUCCESS=0
for i in $(seq 60); do
    echo -n "."
    kubectl get shoot 2>/dev/null >&2 && SUCCESS=1 && break
    sleep 1
done
if [ $SUCCESS -eq 0 ]; then
    echo -e "\nGardener apiserver did not start in time. Aborting"
    exit 1
fi
