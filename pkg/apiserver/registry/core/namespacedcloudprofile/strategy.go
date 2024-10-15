// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile

import (
	"context"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

type namespacedCloudProfileStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for NamespacedCloudProfiles.
var Strategy = namespacedCloudProfileStrategy{api.Scheme, names.SimpleNameGenerator}

func (namespacedCloudProfileStrategy) NamespaceScoped() bool {
	return true
}

func (namespacedCloudProfileStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	namespacedCloudProfile := obj.(*core.NamespacedCloudProfile)

	dropExpiredVersions(namespacedCloudProfile)
	namespacedCloudProfile.Generation = 1
	namespacedCloudProfile.Status = core.NamespacedCloudProfileStatus{}
}

func (namespacedCloudProfileStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	namespacedCloudProfile := obj.(*core.NamespacedCloudProfile)
	return validation.ValidateNamespacedCloudProfile(namespacedCloudProfile)
}

func (namespacedCloudProfileStrategy) Canonicalize(_ runtime.Object) {
}

func (namespacedCloudProfileStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (namespacedCloudProfileStrategy) PrepareForUpdate(_ context.Context, newObj, oldObj runtime.Object) {
	oldNamespacedCloudProfile := oldObj.(*core.NamespacedCloudProfile)
	newNamespacedCloudProfile := newObj.(*core.NamespacedCloudProfile)

	if mustIncreaseGeneration(oldNamespacedCloudProfile, newNamespacedCloudProfile) {
		newNamespacedCloudProfile.Generation = oldNamespacedCloudProfile.Generation + 1
	}
}

func mustIncreaseGeneration(oldNamespacedCloudProfile, newNamespacedCloudProfile *core.NamespacedCloudProfile) bool {
	// The NamespacedCloudProfile specification changes.
	if !apiequality.Semantic.DeepEqual(oldNamespacedCloudProfile.Spec, newNamespacedCloudProfile.Spec) {
		return true
	}

	// The deletion timestamp was set.
	if oldNamespacedCloudProfile.DeletionTimestamp == nil && newNamespacedCloudProfile.DeletionTimestamp != nil {
		return true
	}

	return false
}

func (namespacedCloudProfileStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (namespacedCloudProfileStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldProfile, newProfile := oldObj.(*core.NamespacedCloudProfile), newObj.(*core.NamespacedCloudProfile)
	return validation.ValidateNamespacedCloudProfileUpdate(newProfile, oldProfile)
}

// WarningsOnCreate returns warnings to the client performing the create.
func (namespacedCloudProfileStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (namespacedCloudProfileStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func dropExpiredVersions(namespacedCloudProfile *core.NamespacedCloudProfile) {
	if namespacedCloudProfile.Spec.Kubernetes != nil {
		var validKubernetesVersions []core.ExpirableVersion

		for _, version := range namespacedCloudProfile.Spec.Kubernetes.Versions {
			if version.ExpirationDate != nil && version.ExpirationDate.Time.Before(time.Now()) {
				continue
			}
			validKubernetesVersions = append(validKubernetesVersions, version)
		}

		namespacedCloudProfile.Spec.Kubernetes.Versions = validKubernetesVersions
	}

	validMachineImages := []core.MachineImage{}
	for i, machineImage := range namespacedCloudProfile.Spec.MachineImages {
		var validMachineImageVersions []core.MachineImageVersion

		for _, version := range machineImage.Versions {
			if version.ExpirationDate != nil && version.ExpirationDate.Time.Before(time.Now()) {
				continue
			}
			validMachineImageVersions = append(validMachineImageVersions, version)
		}
		if len(validMachineImageVersions) > 0 {
			namespacedCloudProfile.Spec.MachineImages[i].Versions = validMachineImageVersions
			validMachineImages = append(validMachineImages, namespacedCloudProfile.Spec.MachineImages[i])
		}
	}
	namespacedCloudProfile.Spec.MachineImages = validMachineImages
}

type namespacedCloudProfileStatusStrategy struct {
	namespacedCloudProfileStrategy
}

// StatusStrategy defines the storage strategy for the status subresource of NamespacedCloudProfiles.
var StatusStrategy = namespacedCloudProfileStatusStrategy{Strategy}

func (namespacedCloudProfileStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newNamespacedCloudProfile := obj.(*core.NamespacedCloudProfile)
	oldNamespacedCloudProfile := old.(*core.NamespacedCloudProfile)
	newNamespacedCloudProfile.Spec = oldNamespacedCloudProfile.Spec
}

func (namespacedCloudProfileStatusStrategy) ValidateUpdate(_ context.Context, _, _ runtime.Object) field.ErrorList {
	return field.ErrorList{}
}
