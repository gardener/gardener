// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	"net"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ClusterAutoscalerRequired returns whether the given worker pool configuration indicates that a cluster-autoscaler
// is needed.
func ClusterAutoscalerRequired(pools []extensionsv1alpha1.WorkerPool) bool {
	for _, pool := range pools {
		if pool.Maximum > pool.Minimum {
			return true
		}
	}
	return false
}

// GetDNSRecordType returns the appropriate DNS record type (A or CNAME) for the given address.
func GetDNSRecordType(address string) extensionsv1alpha1.DNSRecordType {
	if ip := net.ParseIP(address); ip != nil && ip.To4() != nil {
		return extensionsv1alpha1.DNSRecordTypeA
	}
	return extensionsv1alpha1.DNSRecordTypeCNAME
}

// GetDNSRecordTTL returns the value of the given ttl, or 120 if nil.
func GetDNSRecordTTL(ttl *int64) int64 {
	if ttl != nil {
		return *ttl
	}
	return 120
}
