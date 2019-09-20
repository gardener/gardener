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

package utils

import (
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateNetworkDisjointedness validates that the given <seedNetworks> and <k8sNetworks> are disjoint.
func ValidateNetworkDisjointedness(seedNetworks gardencorev1alpha1.SeedNetworks, shootNodes string, shootPods, shootServices *string, fldPath *field.Path) field.ErrorList {
	var (
		allErrs = field.ErrorList{}

		pathNodes    = fldPath.Child("nodes")
		pathServices = fldPath.Child("services")
		pathPods     = fldPath.Child("pods")
	)

	if shootNodes != "" {
		if utils.NetworksIntersect(seedNetworks.Nodes, shootNodes) {
			allErrs = append(allErrs, field.Invalid(pathNodes, shootNodes, "shoot node network intersects with seed node network"))
		}
	} else {
		allErrs = append(allErrs, field.Required(pathNodes, "nodes is required"))
	}

	if shootServices != nil {
		if utils.NetworksIntersect(seedNetworks.Services, *shootServices) {
			allErrs = append(allErrs, field.Invalid(pathServices, *shootServices, "shoot service network intersects with seed service network"))
		}
	} else if seedNetworks.ShootDefaults == nil || seedNetworks.ShootDefaults.Services == nil {
		allErrs = append(allErrs, field.Required(pathServices, "services is required"))
	}

	if shootPods != nil {
		if utils.NetworksIntersect(seedNetworks.Pods, *shootPods) {
			allErrs = append(allErrs, field.Invalid(pathPods, *shootPods, "shoot pod network intersects with seed pod network"))
		}
	} else if seedNetworks.ShootDefaults == nil || seedNetworks.ShootDefaults.Pods == nil {
		allErrs = append(allErrs, field.Required(pathPods, "pods is required"))
	}

	return allErrs
}
