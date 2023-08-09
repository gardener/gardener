// Copyright 2023 SAP SE or an SAP affiliate company
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"context"
	"strconv"

	"github.com/gardener/etcd-druid/pkg/common"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/go-logr/logr"
	coordinationv1 "k8s.io/api/coordination/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsPeerURLTLSEnabled checks if the TLS has been enabled for all existing members of an etcd cluster identified by etcdName and in the provided namespace.
func IsPeerURLTLSEnabled(ctx context.Context, cli client.Client, namespace, etcdName string, logger logr.Logger) (bool, error) {
	var tlsEnabledValues []bool
	labels := GetMemberLeaseLabels(etcdName)
	leaseList := &coordinationv1.LeaseList{}
	if err := cli.List(ctx, leaseList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return false, err
	}
	for _, lease := range leaseList.Items {
		tlsEnabled := parseAndGetTLSEnabledValue(lease, logger)
		if tlsEnabled != nil {
			tlsEnabledValues = append(tlsEnabledValues, *tlsEnabled)
		}
	}
	tlsEnabled := true
	for _, v := range tlsEnabledValues {
		tlsEnabled = tlsEnabled && v
	}
	return tlsEnabled, nil
}

// PurposeMemberLease is a constant used as a purpose for etcd member lease objects.
const PurposeMemberLease = "etcd-member-lease"

// GetMemberLeaseLabels creates a map of default labels for member lease.
func GetMemberLeaseLabels(etcdName string) map[string]string {
	return map[string]string{
		common.GardenerOwnedBy:           etcdName,
		v1beta1constants.GardenerPurpose: PurposeMemberLease,
	}
}

func parseAndGetTLSEnabledValue(lease coordinationv1.Lease, logger logr.Logger) *bool {
	const peerURLTLSEnabledKey = "member.etcd.gardener.cloud/tls-enabled"
	if lease.Annotations != nil {
		if tlsEnabledStr, ok := lease.Annotations[peerURLTLSEnabledKey]; ok {
			tlsEnabled, err := strconv.ParseBool(tlsEnabledStr)
			if err != nil {
				logger.Error(err, "tls-enabled value is not a valid boolean", "namespace", lease.Namespace, "leaseName", lease.Name)
				return nil
			}
			return &tlsEnabled
		}
		logger.V(4).Info("tls-enabled annotation not present for lease.", "namespace", lease.Namespace, "leaseName", lease.Name)
	}
	return nil
}
