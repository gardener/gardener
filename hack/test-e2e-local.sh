#!/usr/bin/env bash

create_failed=yes
delete_failed=yes

trap shoot_deletion EXIT

function shoot_deletion {
  go test -mod=vendor -timeout=15m ./test/system/shoot_deletion \
    --v -ginkgo.v -ginkgo.progress \
    -kubecfg=$KUBECONFIG \
    -project-namespace=garden-local \
    -shoot-name=e2e-local \
    -skip-accessing-shoot=${SKIP_ACCESSING_SHOOT:-true}

  if [ $? = 0 ] ; then
    delete_failed=no
  fi

  if [ $create_failed = yes ] || [ $delete_failed = yes ] ; then
    exit 1
  fi
}

go test -mod=vendor -timeout=15m ./test/system/shoot_creation \
  --v -ginkgo.v -ginkgo.progress \
  -kubecfg=$KUBECONFIG \
  -project-namespace=garden-local \
  -shoot-name=e2e-local \
  -annotations=shoot.gardener.cloud/infrastructure-cleanup-wait-period-seconds=0 \
  -k8s-version=1.23.1 \
  -cloud-profile=local \
  -seed=local \
  -region=local \
  -secret-binding=local \
  -provider-type=local \
  -networking-type=local \
  -workers-config-filepath=<(cat <<EOF
- name: local
  machine:
    type: local
  cri:
    name: containerd
  maximum: 1
  minimum: 1
  maxSurge: 1
  maxUnavailable: 0
EOF
) \
  -shoot-template-path=<(cat <<EOF
apiVersion: core.gardener.cloud/v1beta1
kind: Shoot
EOF
) \
  -skip-accessing-shoot=${SKIP_ACCESSING_SHOOT:-true}

if [ $? = 0 ] ; then
  create_failed=no
fi
