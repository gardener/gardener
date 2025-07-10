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

var (
	// PortEtcdClient is the port exposed by etcd for client communication.
	PortEtcdClient int32 = 2379
	// PortEtcdPeer is the port exposed by etcd for server-to-server communication.
	PortEtcdPeer int32 = 2380
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
)
