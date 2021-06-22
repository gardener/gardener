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

	scalerapi "github.com/gardener/dependency-watchdog/pkg/scaler/api"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/utils/pointer"
)

const (
	// DependencyWatchdogExternalProbeSecretName is the name of the kubecfg secret with internal DNS for external access.
	DependencyWatchdogExternalProbeSecretName = "dependency-watchdog-external-probe"
	// DependencyWatchdogInternalProbeSecretName is the name of the kubecfg secret with cluster IP access.
	DependencyWatchdogInternalProbeSecretName = "dependency-watchdog-internal-probe"
)

// DependencyWatchdogProbeConfiguration returns the configuration for the dependency watchdog (probe role)
// ensuring that its dependant pods are scaled as soon a probe fails.
func DependencyWatchdogProbeConfiguration() ([]scalerapi.ProbeDependants, error) {
	return []scalerapi.ProbeDependants{{
		Name: "shoot-" + v1beta1constants.DeploymentNameKubeAPIServer,
		Probe: &scalerapi.ProbeConfig{
			External:      &scalerapi.ProbeDetails{KubeconfigSecretName: DependencyWatchdogExternalProbeSecretName},
			Internal:      &scalerapi.ProbeDetails{KubeconfigSecretName: DependencyWatchdogInternalProbeSecretName},
			PeriodSeconds: pointer.Int32(30),
		},
		DependantScales: []*scalerapi.DependantScaleDetails{{
			ScaleRef: autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       v1beta1constants.DeploymentNameKubeControllerManager,
			},
		}},
	}}, nil
}
