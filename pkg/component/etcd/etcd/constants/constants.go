// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

import (
	"fmt"
)

// ServiceName returns the service name for an etcd for the given role.
func ServiceName(role string) string {
	return fmt.Sprintf("etcd-%s-client", role)
}

const (
	// ContainerNameEtcd is the name of the etcd container in the etcd pod.
	ContainerNameEtcd = "etcd"
	// ContainerNameBackupRestore is the name of the backup-restore sidecar container in the etcd pod.
	ContainerNameBackupRestore = "backup-restore"
	// LabelAppValue is the value of a label whose key is 'app'.
	LabelAppValue = "etcd-statefulset"
	// ServicePortNameEtcdPeer is the name prefix used for etcd peer ports on the Istio ingress gateway.
	ServicePortNameEtcdPeer = "tls-etcd-peer"
	// ServicePortNameEtcdClient is the name used for the etcd client port on the Istio ingress gateway.
	ServicePortNameEtcdClient = "tls-etcd-client"
)

var (
	// PortEtcdClient is the port exposed by etcd for client communication.
	PortEtcdClient int32 = 2379
	// PortEtcdPeer is the port exposed by etcd for server-to-server communication.
	PortEtcdPeer int32 = 2380
	// PortEtcdPeerExternal is the base port on the Istio ingress gateway for etcd peer traffic.
	// Member ordinal i is exposed on PortEtcdPeerExternal+i.
	PortEtcdPeerExternal int32 = 12380
	// PortEtcdClientExternal is the port on the Istio ingress gateway for etcd client traffic.
	PortEtcdClientExternal int32 = 12379

	// PortBackupRestore is the client port exposed by the backup-restore sidecar container.
	PortBackupRestore int32 = 8080
	// PortEtcdWrapper is the port exposed by etcd-wrapper.
	PortEtcdWrapper int32 = 9095

	// StaticPodPortEtcdEventsClient is the port exposed by etcd-events for client communication when it runs as static
	// pod.
	StaticPodPortEtcdEventsClient int32 = 2382
	// StaticPodPortEtcdEventsPeer is the port exposed by etcd-events for server-to-server communication when it runs as
	// static pod.
	StaticPodPortEtcdEventsPeer int32 = 2383
	// StaticPodPortEtcdEventsBackupRestore is the client port exposed by the backup-restore sidecar container when it
	// runs as static pod.
	StaticPodPortEtcdEventsBackupRestore int32 = 8081
	// StaticPodPortEtcdEventsWrapper is the port exposed by the etcd-wrapper container in etcd-events when it runs as
	// static pod.
	StaticPodPortEtcdEventsWrapper int32 = 9096

	// VolumeNameServerTLS is the name of the volume in the ETCD pod spec used for the server TLS.
	VolumeNameServerTLS = "etcd-server-tls"
	// VolumeNamePeerTLS is the name of the volume in the ETCD pod spec used for the peer TLS.
	VolumeNamePeerTLS = "etcd-peer-server-tls"

	// HAReplicaCount is the number of replicas for a highly available etcd cluster.
	HAReplicaCount int32 = 3
)
