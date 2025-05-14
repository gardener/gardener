// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// SetDefaults_ManagedSeed sets default values for ManagedSeed objects.
func SetDefaults_ManagedSeed(obj *ManagedSeed) {
	setDefaultsGardenlet(&obj.Spec.Gardenlet)
}

// SetDefaults_GardenletDeployment sets default values for GardenletDeployment objects.
func SetDefaults_GardenletDeployment(obj *GardenletDeployment) {
	// Set default replica count
	if obj.ReplicaCount == nil {
		obj.ReplicaCount = ptr.To[int32](2)
	}

	// Set default revision history limit
	if obj.RevisionHistoryLimit == nil {
		obj.RevisionHistoryLimit = ptr.To[int32](2)
	}

	// Set default image
	if obj.Image == nil {
		obj.Image = &Image{}
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

func setDefaultsGardenlet(obj *GardenletConfig) {
	// Set deployment defaults
	if obj.Deployment == nil {
		obj.Deployment = &GardenletDeployment{}
	}

	setDefaultsGardenletConfig(&obj.Config)

	// Set default garden connection bootstrap
	if obj.Bootstrap == nil {
		gardenConnectionBootstrap := BootstrapToken
		obj.Bootstrap = &gardenConnectionBootstrap
	}

	// Set default merge with parent
	if obj.MergeWithParent == nil {
		obj.MergeWithParent = ptr.To(true)
	}
}

func setDefaultsGardenletConfig(config *runtime.RawExtension) {
	if config == nil {
		return
	}

	// Decode gardenlet config to an external version
	// Without defaults, since we don't want to set gardenlet config defaults in the resource at this point
	gardenletConfig, err := encoding.DecodeGardenletConfiguration(config, false)
	if err != nil {
		return
	}

	// If the gardenlet config was decoded without errors to nil,
	// initialize it with an empty config
	if gardenletConfig == nil {
		gardenletConfig = &gardenletconfigv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
		}
	}

	// Set gardenlet config defaults
	setDefaultsGardenletConfiguration(gardenletConfig)

	// Set gardenlet config back to obj.Config
	// Encoding back to bytes is not needed, it will be done by the custom conversion code
	*config = runtime.RawExtension{Object: gardenletConfig}
}

func setDefaultsGardenletConfiguration(obj *gardenletconfigv1alpha1.GardenletConfiguration) {
	// Initialize resources
	if obj.Resources == nil {
		obj.Resources = &gardenletconfigv1alpha1.ResourcesConfiguration{}
	}

	// Set resources defaults
	setDefaultsResources(obj.Resources)

	// Initialize seed config
	if obj.SeedConfig == nil {
		obj.SeedConfig = &gardenletconfigv1alpha1.SeedConfig{}
	}
}

func setDefaultsResources(obj *gardenletconfigv1alpha1.ResourcesConfiguration) {
	if _, ok := obj.Capacity[gardencorev1beta1.ResourceShoots]; !ok {
		if obj.Capacity == nil {
			obj.Capacity = make(corev1.ResourceList)
		}
		obj.Capacity[gardencorev1beta1.ResourceShoots] = resource.MustParse("250")
	}
}
