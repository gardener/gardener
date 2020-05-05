// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cidr

import (
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateNetworkDisjointedness validates that the given <seedNetworks> and <k8sNetworks> are disjoint.
func ValidateNetworkDisjointedness(fldPath *field.Path, shootNodes, shootPods, shootServices, seedNodes *string, seedPods, seedServices string) field.ErrorList {
	var (
		allErrs = field.ErrorList{}

		pathNodes    = fldPath.Child("nodes")
		pathServices = fldPath.Child("services")
		pathPods     = fldPath.Child("pods")
	)

	if shootNodes != nil && seedNodes != nil && NetworksIntersect(*shootNodes, *seedNodes) {
		allErrs = append(allErrs, field.Invalid(pathNodes, *shootNodes, "shoot node network intersects with seed node network"))
	}

	if shootServices != nil {
		if NetworksIntersect(seedServices, *shootServices) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot service network intersects with seed service network"))
		}
		if NetworksIntersect(seedPods, *shootServices) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot service network intersects with seed pod network"))
		}
	} else {
		allErrs = append(allErrs, field.Required(pathServices, "services is required"))
	}

	if shootPods != nil {
		if NetworksIntersect(seedPods, *shootPods) {
			allErrs = append(allErrs, field.Invalid(pathPods, *shootPods, "shoot pod network intersects with seed pod network"))
		}
		if NetworksIntersect(seedServices, *shootPods) {
			allErrs = append(allErrs, field.Invalid(pathPods, *shootPods, "shoot pod network intersects with seed service network"))
		}
	} else {
		allErrs = append(allErrs, field.Required(pathPods, "pods is required"))
	}

	return allErrs
}

// NetworksIntersect returns true if the given network CIDRs intersect.
func NetworksIntersect(cidr1, cidr2 string) bool {
	_, net1, err1 := net.ParseCIDR(cidr1)
	_, net2, err2 := net.ParseCIDR(cidr2)
	return err1 != nil || err2 != nil || net2.Contains(net1.IP) || net1.Contains(net2.IP)
}
