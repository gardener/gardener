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
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateNetworkDisjointedness validates that the given <seedNetworks> and <k8sNetworks> are disjoint.
func ValidateNetworkDisjointedness(seedNetworks gardenv1beta1.SeedNetworks, k8sNetworks gardencorev1alpha1.K8SNetworks, fldPath *field.Path) field.ErrorList {
	var (
		allErrs = field.ErrorList{}

		pathNodes    = fldPath.Child("nodes")
		pathServices = fldPath.Child("services")
		pathPods     = fldPath.Child("pods")
	)

	if nodes := k8sNetworks.Nodes; nodes != nil {
		if gardencorev1alpha1helper.NetworksIntersect(seedNetworks.Nodes, *nodes) {
			allErrs = append(allErrs, field.Invalid(pathNodes, *nodes, "shoot node network intersects with seed node network"))
		}
	} else {
		allErrs = append(allErrs, field.Required(pathNodes, "nodes is required"))
	}

	if services := k8sNetworks.Services; services != nil {
		if gardencorev1alpha1helper.NetworksIntersect(seedNetworks.Services, *services) {
			allErrs = append(allErrs, field.Invalid(pathServices, *services, "shoot service network intersects with seed service network"))
		}
	} else if seedNetworks.ShootDefaults == nil || seedNetworks.ShootDefaults.Services == nil {
		allErrs = append(allErrs, field.Required(pathServices, "services is required"))
	}

	if pods := k8sNetworks.Pods; pods != nil {
		if gardencorev1alpha1helper.NetworksIntersect(seedNetworks.Pods, *pods) {
			allErrs = append(allErrs, field.Invalid(pathPods, *pods, "shoot pod network intersects with seed pod network"))
		}
	} else if seedNetworks.ShootDefaults == nil || seedNetworks.ShootDefaults.Pods == nil {
		allErrs = append(allErrs, field.Required(pathPods, "pods is required"))
	}

	return allErrs
}
