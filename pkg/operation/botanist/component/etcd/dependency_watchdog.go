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

package etcd

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	restarterapi "github.com/gardener/dependency-watchdog/pkg/restarter/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DependencyWatchdogEndpointConfiguration returns the configuration for the dependency watchdog ensuring that its dependant
// pods are restarted as soon as it recovers from a crash loop.
func DependencyWatchdogEndpointConfiguration(role string) (map[string]restarterapi.Service, error) {
	return map[string]restarterapi.Service{
		ServiceName(role): {
			Dependants: []restarterapi.DependantPods{
				{
					Name: v1beta1constants.GardenRoleControlPlane,
					Selector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      v1beta1constants.GardenRole,
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{v1beta1constants.GardenRoleControlPlane},
							},
							{
								Key:      v1beta1constants.LabelRole,
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{v1beta1constants.LabelAPIServer},
							},
						},
					},
				},
			},
		},
	}, nil
}
