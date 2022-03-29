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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"

	restarterapi "github.com/gardener/dependency-watchdog/pkg/restarter/api"
	scalerapi "github.com/gardener/dependency-watchdog/pkg/scaler/api"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// DependencyWatchdogEndpointConfiguration returns the configuration for the dependency watchdog (endpoint role)
// ensuring that its dependant pods are restarted as soon as it recovers from a crash loop.
func DependencyWatchdogEndpointConfiguration() (map[string]restarterapi.Service, error) {
	return map[string]restarterapi.Service{
		v1beta1constants.DeploymentNameKubeAPIServer: {
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
								Operator: metav1.LabelSelectorOpNotIn,
								Values:   []string{v1beta1constants.ETCDRoleMain, v1beta1constants.LabelAPIServer},
							},
						},
					},
				},
			},
		},
	}, nil
}

// DependencyWatchdogProbeConfiguration returns the configuration for the dependency watchdog (probe role)
// ensuring that its dependant pods are scaled as soon a probe fails.
func DependencyWatchdogProbeConfiguration() ([]scalerapi.ProbeDependants, error) {
	return []scalerapi.ProbeDependants{{
		Name: "shoot-" + v1beta1constants.DeploymentNameKubeAPIServer,
		Probe: &scalerapi.ProbeConfig{
			External:      &scalerapi.ProbeDetails{KubeconfigSecretName: dependencywatchdog.ExternalProbeSecretName},
			Internal:      &scalerapi.ProbeDetails{KubeconfigSecretName: dependencywatchdog.InternalProbeSecretName},
			PeriodSeconds: pointer.Int32(30),
		},
		DependantScales: []*scalerapi.DependantScaleDetails{
			{
				ScaleRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       v1beta1constants.DeploymentNameKubeControllerManager,
				},
				ScaleUpDelaySeconds: pointer.Int32(120),
			},
			{
				ScaleRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       v1beta1constants.DeploymentNameMachineControllerManager,
				},
				ScaleUpDelaySeconds: pointer.Int32(60),
				ScaleRefDependsOn: []autoscalingv1.CrossVersionObjectReference{
					{
						APIVersion: appsv1.SchemeGroupVersion.String(),
						Kind:       "Deployment",
						Name:       v1beta1constants.DeploymentNameKubeControllerManager,
					},
				},
			},
			{
				ScaleRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       v1beta1constants.DeploymentNameClusterAutoscaler,
				},
				ScaleRefDependsOn: []autoscalingv1.CrossVersionObjectReference{
					{
						APIVersion: appsv1.SchemeGroupVersion.String(),
						Kind:       "Deployment",
						Name:       v1beta1constants.DeploymentNameMachineControllerManager,
					},
				},
			},
		},
	}}, nil
}
