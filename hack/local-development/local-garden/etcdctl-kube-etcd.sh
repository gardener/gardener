#!/usr/bin/env bash

docker exec -i --env ETCDCTL_API=3 etcd etcdctl \
 --endpoints https://localhost:12379 \
 --key /keys/kube-etcd.key --cert /certs/kube-etcd.crt --cacert /certs/ca.crt \
 $@
