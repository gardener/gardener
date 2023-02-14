---
title: 06 etcd-druid
---

# Integrating etcd-druid with Gardener

etcd is currently deployed by garden-controller-manager as a Statefulset. The sidecar container spec contains details pertaining to cloud-provider object-store, which is injected into the statefulset via a mutable webhook running as part of the gardener extension [story](https://github.com/gardener/gardener/blob/master/docs/extensions/controlplane-webhooks.md#what-needs-to-be-implemented-to-support-a-new-cloud-provider). This approach restricts the operations on etcd, such as scale-up and upgrade. etcd-druid will eliminate the need to hijack statefulset creation to add cloudprovider details. It has been designed to provide an intricate control over the procedure of deploying and maintaining etcd. The roadmap for etcd-druid can be found at the [gardener/etcd-druid](https://github.com/gardener/etcd-druid/issues/2) repository. 

This document explains how Gardener deploys etcd and what resources it creates for etcd-druid to deploy an etcd cluster.

## Resources Required by etcd-druid (Created by Gardener)

etcd-druid requires a:
* Secret containing credentials to access backup bucket in Cloud provider object store.
* TLS server and client secrets for etcd and backup-sidecar.
* etcd CRD resource that contains parameters pertaining to etcd, backup-sidecar, and cloud-provider object store.

When an etcd resource is created in the cluster, the druid acts on it by creating an etcd statefulset, a service, and a configmap containing the etcd bootstrap script. The secrets containing the infrastructure credentials and the TLS certificates are mounted as volumes. If no secret/information regarding backups is stated, then etcd data backups are not taken. Only data corruption checks are performed prior to starting etcd.

Garden-controller-manager, being cloud agnostic, deploys the etcd resource. This will not contain any cloud-specific information other than the cloud-provider. The extension controller that contains the cloud specific implementation to create the backup bucket will create it if needed and create a secret containing the credentials to access the bucket. The etcd backup secret name should be exposed in the BackupEntry status. Then, Gardener can read it and write it into the ETCD resource. The secret will have to be made available in the namespace the etcd statefulset will be deployed. If etcd and backup-sidecar communicates over TLS then the CA certificates, server and client certificates, and keys will also have to be made available in the namespace as well. The etcd resource will have a reference to these aforementioned secrets. etcd-druid will deploy the statefulset only if the secrets are available.

## Workflow
* etcd-druid will be deployed and etcd CRD will be created as part of the seed bootstrap.
* Garden-controller-manager creates a backupBucket extension resource. The extension controller creates the backup bucket associated with the seed.
* Garden-controller-manager creates a backupentry associated with each shoot in the seed namespace. 
* Garden-controller-manager creates an etcd resource with secretRefs and etcd information populated appropriately.
* etcd-druid acts on the etcd resource; druid creates the statefulset, the service, and the configmap.

![etcd-druid](assets/druid_integration.png)
