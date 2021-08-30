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
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_ManagedSeed sets default values for ManagedSeed objects.
func SetDefaults_ManagedSeed(obj *ManagedSeed) {
	switch {
	case obj.Spec.SeedTemplate != nil:
		setDefaultsSeedSpec(&obj.Spec.SeedTemplate.Spec, obj.Name, obj.Namespace, true)
	case obj.Spec.Gardenlet != nil:
		setDefaultsGardenlet(obj.Spec.Gardenlet, obj.Name, obj.Namespace)
	}
}

// SetDefaults_GardenletDeployment sets default values for GardenletDeployment objects.
func SetDefaults_GardenletDeployment(obj *GardenletDeployment) {
	// Set default replica count
	if obj.ReplicaCount == nil {
		obj.ReplicaCount = pointer.Int32(1)
	}

	// Set default revision history limit
	if obj.RevisionHistoryLimit == nil {
		obj.RevisionHistoryLimit = pointer.Int32(10)
	}

	// Set default image
	if obj.Image == nil {
		obj.Image = &Image{}
	}

	// Set default VPA
	if obj.VPA == nil {
		obj.VPA = pointer.Bool(true)
	}
}

// SetDefaults_Image sets default values for Image objects.
func SetDefaults_Image(obj *Image) {
	// Set default pull policy
	if obj.PullPolicy == nil {
		var pullPolicy corev1.PullPolicy
		if obj.Tag != nil && *obj.Tag == "latest" {
			pullPolicy = corev1.PullAlways
		} else {
			pullPolicy = corev1.PullIfNotPresent
		}
		obj.PullPolicy = &pullPolicy
	}
}

func setDefaultsGardenlet(obj *Gardenlet, name, namespace string) {
	// Set deployment defaults
	if obj.Deployment == nil {
		obj.Deployment = &GardenletDeployment{}
	}

	// Decode gardenlet config to an external version
	// Without defaults, since we don't want to set gardenlet config defaults in the resource at this point
	gardenletConfig, err := encoding.DecodeGardenletConfiguration(&obj.Config, false)
	if err != nil {
		return
	}

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

	// Set gardenlet config defaults
	setDefaultsGardenletConfiguration(gardenletConfig, name, namespace)

	// Set gardenlet config back to obj.Config
	// Encoding back to bytes is not needed, it will be done by the custom conversion code
	obj.Config = runtime.RawExtension{Object: gardenletConfig}

	// Set default garden connection bootstrap
	if obj.Bootstrap == nil {
		gardenConnectionBootstrap := BootstrapToken
		obj.Bootstrap = &gardenConnectionBootstrap
	}

	// Set default merge with parent
	if obj.MergeWithParent == nil {
		obj.MergeWithParent = pointer.Bool(true)
	}
}

func setDefaultsGardenletConfiguration(obj *configv1alpha1.GardenletConfiguration, name, namespace string) {
	// Initialize resources
	if obj.Resources == nil {
		obj.Resources = &configv1alpha1.ResourcesConfiguration{}
	}

	// Set resources defaults
	setDefaultsResources(obj.Resources)

	// Initialize seed config
	if obj.SeedConfig == nil {
		obj.SeedConfig = &configv1alpha1.SeedConfig{}
	}

	// Set seed spec defaults
	setDefaultsSeedSpec(&obj.SeedConfig.SeedTemplate.Spec, name, namespace, false)
}

func setDefaultsResources(obj *configv1alpha1.ResourcesConfiguration) {
	if _, ok := obj.Capacity[gardencorev1beta1.ResourceShoots]; !ok {
		if obj.Capacity == nil {
			obj.Capacity = make(corev1.ResourceList)
		}
		obj.Capacity[gardencorev1beta1.ResourceShoots] = resource.MustParse("250")
	}
}

func setDefaultsSeedSpec(spec *gardencorev1beta1.SeedSpec, name, namespace string, withSecretRef bool) {
	if spec.SecretRef == nil && withSecretRef {
		spec.SecretRef = &corev1.SecretReference{
			Name:      fmt.Sprintf("seed-%s", name),
			Namespace: namespace,
		}
	}
	if spec.Backup != nil && spec.Backup.SecretRef == (corev1.SecretReference{}) {
		spec.Backup.SecretRef = corev1.SecretReference{
			Name:      fmt.Sprintf("backup-%s", name),
			Namespace: namespace,
		}
	}
}
