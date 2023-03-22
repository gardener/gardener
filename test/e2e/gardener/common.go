// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/test/framework"
)

// DefaultGardenConfig returns a GardenerConfig framework object with default values for the e2e tests.
func DefaultGardenConfig(projectNamespace string) *framework.GardenerConfig {
	return &framework.GardenerConfig{
		CommonConfig: &framework.CommonConfig{
			DisableStateDump: true,
		},
		ProjectNamespace:   projectNamespace,
		GardenerKubeconfig: os.Getenv("KUBECONFIG"),
	}
}

// getShootControlPlane returns a ControlPlane object based on env variable SHOOT_FAILURE_TOLERANCE_TYPE value
func getShootControlPlane() *gardencorev1beta1.ControlPlane {
	var failureToleranceType gardencorev1beta1.FailureToleranceType
	switch os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE") {
	case "zone":
		failureToleranceType = gardencorev1beta1.FailureToleranceTypeZone
	case "node":
		failureToleranceType = gardencorev1beta1.FailureToleranceTypeNode
	default:
		return nil
	}

	return &gardencorev1beta1.ControlPlane{
		HighAvailability: &gardencorev1beta1.HighAvailability{
			FailureTolerance: gardencorev1beta1.FailureTolerance{
				Type: failureToleranceType,
			},
		},
	}
}

// DefaultShoot returns a Shoot object with default values for the e2e tests.
func DefaultShoot(name string) *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				v1beta1constants.AnnotationShootInfrastructureCleanupWaitPeriodSeconds: "0",
				v1beta1constants.AnnotationShootCloudConfigExecutionMaxDelaySeconds:    "0",
			},
		},
		Spec: gardencorev1beta1.ShootSpec{
			ControlPlane:      getShootControlPlane(),
			Region:            "local",
			SecretBindingName: "local",
			CloudProfileName:  "local",
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version:                     "1.26.0",
				EnableStaticTokenKubeconfig: pointer.Bool(false),
				Kubelet: &gardencorev1beta1.KubeletConfig{
					SerializeImagePulls: pointer.Bool(false),
					RegistryPullQPS:     pointer.Int32(10),
					RegistryBurst:       pointer.Int32(20),
				},
				KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
			},
			Networking: gardencorev1beta1.Networking{
				Type: "calico",
				// TODO(scheererj): Drop this once v1.32 has been released and https://github.com/gardener/gardener-extension-networking-calico/pull/250 is available as release
				ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"apiVersion":"calico.networking.extensions.gardener.cloud/v1alpha1","kind":"NetworkConfig"}`)},
			},
			Provider: gardencorev1beta1.Provider{
				Type: "local",
				Workers: []gardencorev1beta1.Worker{{
					Name: "local",
					Machine: gardencorev1beta1.Machine{
						Type: "local",
					},
					CRI: &gardencorev1beta1.CRI{
						Name: gardencorev1beta1.CRINameContainerD,
					},
					Labels: map[string]string{
						"foo": "bar",
					},
					Minimum: 1,
					Maximum: 1,
				}},
			},
			Extensions: []gardencorev1beta1.Extension{
				{
					Type: "local-ext-seed",
				},
				{
					Type: "local-ext-shoot",
				},
			},
		},
	}
}
