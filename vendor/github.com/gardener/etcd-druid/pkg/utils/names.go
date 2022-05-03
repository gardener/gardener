// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"fmt"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
)

// GetPeerServiceName returns the peer service name of ETCD cluster reachable by members within the ETCD cluster
func GetPeerServiceName(etcd *druidv1alpha1.Etcd) string {
	return fmt.Sprintf("%s-peer", etcd.Name)
}

// GetClientServiceName returns the client service name of ETCD cluster reachable by external client
func GetClientServiceName(etcd *druidv1alpha1.Etcd) string {
	return fmt.Sprintf("%s-client", etcd.Name)
}

// GetServiceAccountName returns the service account name
func GetServiceAccountName(etcd *druidv1alpha1.Etcd) string {
	return etcd.Name
}

// GetConfigmapName returns the name of the configmap based on the given `etcd` object.
func GetConfigmapName(etcd *druidv1alpha1.Etcd) string {
	return fmt.Sprintf("etcd-bootstrap-%s", string(etcd.UID[:6]))
}

// GetCronJobName returns the legacy compaction cron job name
func GetCronJobName(etcd *druidv1alpha1.Etcd) string {
	return fmt.Sprintf("%s-compact-backup", etcd.Name)
}

// GetJobName returns the compaction job name
func GetJobName(etcd *druidv1alpha1.Etcd) string {
	return fmt.Sprintf("%s-compact-job", string(etcd.UID[:6]))
}

// GetOrdinalPodName returns the ETCD pod name based on the order
func GetOrdinalPodName(etcd *druidv1alpha1.Etcd, order int) string {
	return fmt.Sprintf("%s-%d", etcd.Name, order)
}
