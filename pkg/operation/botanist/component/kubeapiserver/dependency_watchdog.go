// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserver

import (
	"time"

	proberapi "github.com/gardener/dependency-watchdog/api/prober"
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// NewDependencyWatchdogWeederConfiguration returns the configuration for the dependency watchdog (weeder role)
// ensuring that its dependant pods are restarted as soon as it recovers from a crash loop.
func NewDependencyWatchdogWeederConfiguration() (map[string]weederapi.DependantSelectors, error) {
	return map[string]weederapi.DependantSelectors{
		v1beta1constants.DeploymentNameKubeAPIServer: {
			PodSelectors: []*metav1.LabelSelector{
				{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      v1beta1constants.GardenRole,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{v1beta1constants.GardenRoleControlPlane},
						},
						{
							Key:      v1beta1constants.LabelRole,
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{v1beta1constants.ETCDRoleMain, v1beta1constants.LabelAPIServer},
						},
					},
				},
			},
		},
	}, nil
}

// NewDependencyWatchdogProberConfiguration returns the configuration for the dependency watchdog (probe role)
// ensuring that its dependant pods are scaled as soon a prober fails.
func NewDependencyWatchdogProberConfiguration() ([]proberapi.DependentResourceInfo, error) {
	return []proberapi.DependentResourceInfo{
		{
			Ref: &autoscalingv1.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       v1beta1constants.DeploymentNameKubeControllerManager,
				APIVersion: "apps/v1",
			},
			Optional: false,
			ScaleUpInfo: &proberapi.ScaleInfo{
				Level: 0,
			},
			ScaleDownInfo: &proberapi.ScaleInfo{
				Level: 1,
			},
		},
		{
			Ref: &autoscalingv1.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       v1beta1constants.DeploymentNameMachineControllerManager,
				APIVersion: "apps/v1",
			},
			Optional: false,
			ScaleUpInfo: &proberapi.ScaleInfo{
				Level:        1,
				InitialDelay: &metav1.Duration{Duration: 30 * time.Second},
			},
			ScaleDownInfo: &proberapi.ScaleInfo{
				Level: 0,
			},
		},
		{
			Ref: &autoscalingv1.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       v1beta1constants.DeploymentNameClusterAutoscaler,
				APIVersion: appsv1.SchemeGroupVersion.String(),
			},
			Optional: true,
			ScaleUpInfo: &proberapi.ScaleInfo{
				Level: 2,
			},
			ScaleDownInfo: &proberapi.ScaleInfo{
				Level: 0,
			},
		},
	}, nil

}
