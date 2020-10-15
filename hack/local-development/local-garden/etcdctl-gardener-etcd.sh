#!/usr/bin/env bash

docker exec -i --env ETCDCTL_API=3 g-etcd etcdctl \
 --endpoints http://localhost:22379 \
 $@
