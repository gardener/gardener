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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_Imports sets defaults for the configuration of the Gardenlet Landscaper.
func SetDefaults_Imports(obj *Imports) {
	// Decode gardenlet config to an external version
	// Without defaults, since we don't want to set gardenlet config defaults in the resource at this point
	gardenletConfig, err := encoding.DecodeGardenletConfiguration(&obj.ComponentConfiguration, false)
	if err != nil {
		return
	}

	SetDefaultsComponentConfiguration(gardenletConfig)

	if obj.DeploymentConfiguration == nil {
		obj.DeploymentConfiguration = &seedmanagementv1alpha1.GardenletDeployment{}
	}

	SetDefaultsDeploymentConfiguration(obj.DeploymentConfiguration)

	// Set gardenlet config back to obj.Config
	// Encoding back to bytes is not needed, it will be done by the custom conversion code
	obj.ComponentConfiguration = runtime.RawExtension{Object: gardenletConfig}

}

// SetDefaultsComponentConfiguration sets defaults for the Gardenlet component configuration for the Landscaper imports
func SetDefaultsComponentConfiguration(gardenletConfig *configv1alpha1.GardenletConfiguration) {
	// If the gardenlet config was decoded without errors to nil,
	// initialize it with an empty config
	if gardenletConfig == nil {
		gardenletConfig = &configv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: configv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
		}
	}

	if gardenletConfig.GardenClientConnection == nil {
		gardenletConfig.GardenClientConnection = &configv1alpha1.GardenClientConnection{
			// set to not default to the RecommendedDefaultClientConnectionConfiguration with lower QPS and Burst
			ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   100,
				Burst: 130,
			},
		}
	}

	if gardenletConfig.GardenClientConnection.BootstrapKubeconfig == nil {
		gardenletConfig.GardenClientConnection.BootstrapKubeconfig = &corev1.SecretReference{
			Name:      managedseed.GardenletDefaultKubeconfigBootstrapSecretName,
			Namespace: v1beta1constants.GardenNamespace,
		}
	}

	if gardenletConfig.GardenClientConnection.KubeconfigSecret == nil {
		gardenletConfig.GardenClientConnection.KubeconfigSecret = &corev1.SecretReference{
			Name:      managedseed.GardenletDefaultKubeconfigSecretName,
			Namespace: v1beta1constants.GardenNamespace,
		}
	}
}

// SetDefaultsDeploymentConfiguration sets default values for DeploymentConfiguration objects.
func SetDefaultsDeploymentConfiguration(obj *seedmanagementv1alpha1.GardenletDeployment) {
	if obj == nil {
		obj = &seedmanagementv1alpha1.GardenletDeployment{}
	}

	// Set default replica count
	if obj.ReplicaCount == nil {
		obj.ReplicaCount = pointer.Int32(1)
	}

	// Set default revision history limit
	if obj.RevisionHistoryLimit == nil {
		obj.RevisionHistoryLimit = pointer.Int32(10)
	}

	if obj.VPA == nil {
		obj.VPA = pointer.Bool(false)
	}
}
