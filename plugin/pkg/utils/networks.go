// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"net"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/garden"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateNetworkDisjointedness validates that the given <seedNetworks> and <k8sNetworks> are disjoint.
func ValidateNetworkDisjointedness(seedNetworks garden.SeedNetworks, k8sNetworks gardencore.K8SNetworks, fldPath *field.Path) field.ErrorList {
	var (
		allErrs = field.ErrorList{}

		pathNodes    = fldPath.Child("nodes")
		pathServices = fldPath.Child("services")
		pathPods     = fldPath.Child("pods")
	)

	if nodes := k8sNetworks.Nodes; nodes != nil {
		if networksIntersect(seedNetworks.Nodes, *nodes) {
			allErrs = append(allErrs, field.Invalid(pathNodes, *nodes, "shoot node network intersects with seed node network"))
		}
	} else {
		allErrs = append(allErrs, field.Required(pathNodes, "nodes is required"))
	}

	if services := k8sNetworks.Services; services != nil {
		if networksIntersect(seedNetworks.Services, *services) {
			allErrs = append(allErrs, field.Invalid(pathServices, *services, "shoot service network intersects with seed node network"))
		}
	} else {
		allErrs = append(allErrs, field.Required(pathServices, "services is required"))
	}

	if pods := k8sNetworks.Pods; pods != nil {
		if networksIntersect(seedNetworks.Pods, *pods) {
			allErrs = append(allErrs, field.Invalid(pathPods, *pods, "shoot pod network intersects with seed node network"))
		}
	} else {
		allErrs = append(allErrs, field.Required(pathPods, "pods is required"))
	}

	return allErrs
}

func networksIntersect(cidr1, cidr2 gardencore.CIDR) bool {
	_, net1, err1 := net.ParseCIDR(string(cidr1))
	_, net2, err2 := net.ParseCIDR(string(cidr2))
	return err1 != nil || err2 != nil || net2.Contains(net1.IP) || net1.Contains(net2.IP)
}
