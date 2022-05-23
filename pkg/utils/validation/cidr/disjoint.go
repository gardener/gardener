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
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
	if shootNodes != nil && NetworksIntersect(*shootNodes, seedServices) {
		allErrs = append(allErrs, field.Invalid(pathNodes, *shootNodes, "shoot node network intersects with seed service network"))
	}
	if shootNodes != nil && NetworksIntersect(*shootNodes, v1beta1constants.DefaultVpnRange) {
		allErrs = append(allErrs, field.Invalid(pathNodes, *shootNodes, fmt.Sprintf("shoot node network intersects with default vpn network (%s)", v1beta1constants.DefaultVpnRange)))
	}

	if shootServices != nil {
		if NetworksIntersect(seedServices, *shootServices) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot service network intersects with seed service network"))
		}
		if NetworksIntersect(seedPods, *shootServices) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot service network intersects with seed pod network"))
		}
		if seedNodes != nil && NetworksIntersect(*seedNodes, *shootServices) {
			allErrs = append(allErrs, field.Invalid(pathServices, *seedNodes, "shoot service network intersects with seed node network"))
		}
		if NetworksIntersect(v1beta1constants.DefaultVpnRange, *shootServices) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, fmt.Sprintf("shoot service network intersects with default vpn network (%s)", v1beta1constants.DefaultVpnRange)))
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
		if seedNodes != nil && NetworksIntersect(*seedNodes, *shootPods) {
			allErrs = append(allErrs, field.Invalid(pathPods, *seedNodes, "shoot pod network intersects with seed node network"))
		}
		if NetworksIntersect(v1beta1constants.DefaultVpnRange, *shootPods) {
			allErrs = append(allErrs, field.Invalid(pathPods, *shootPods, fmt.Sprintf("shoot pod network intersects with default vpn network (%s)", v1beta1constants.DefaultVpnRange)))
		}
	} else {
		allErrs = append(allErrs, field.Required(pathPods, "pods is required"))
	}

	return allErrs
}

// ValidateShootNetworkDisjointedness validates that the given shoot network is disjoint.
func ValidateShootNetworkDisjointedness(fldPath *field.Path, shootNodes, shootPods, shootServices *string) field.ErrorList {
	var (
		allErrs = field.ErrorList{}

		pathServices = fldPath.Child("services")
		pathPods     = fldPath.Child("pods")
	)

	if shootPods != nil && shootServices != nil {
		if NetworksIntersect(*shootPods, *shootServices) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot pod network intersects with shoot service network"))
		}
		if shootNodes != nil && NetworksIntersect(*shootPods, *shootNodes) {
			allErrs = append(allErrs, field.Invalid(pathPods, *shootPods, "shoot pod network intersects with shoot node network"))
		}
		if shootNodes != nil && NetworksIntersect(*shootServices, *shootNodes) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot service network intersects with shoot node network"))
		}
	} else if shootPods != nil {
		if shootNodes != nil && NetworksIntersect(*shootPods, *shootNodes) {
			allErrs = append(allErrs, field.Invalid(pathPods, *shootPods, "shoot pod network intersects with shoot node network"))
		}
		allErrs = append(allErrs, field.Required(pathServices, "shoot service network is required"))
	} else if shootServices != nil {
		if shootNodes != nil && NetworksIntersect(*shootServices, *shootNodes) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot service network intersects with shoot node network"))
		}
		allErrs = append(allErrs, field.Required(pathPods, "shoot pod network is required"))
	} else {
		allErrs = append(allErrs, field.Required(pathServices, "shoot service network is required"))
		allErrs = append(allErrs, field.Required(pathPods, "shoot pod network is required"))
	}

	return allErrs
}

// NetworksIntersect returns true if the given network CIDRs intersect.
func NetworksIntersect(cidr1, cidr2 string) bool {
	c1 := NewCIDR(cidr1, field.NewPath(""))
	c2 := NewCIDR(cidr2, field.NewPath(""))
	return c1.ValidateOverlap(c2).ToAggregate() == nil
}
