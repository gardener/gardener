// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	"github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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

	dropInactiveVersions(namespacedCloudProfile, nil)
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

	newNamespacedCloudProfile.Status = oldNamespacedCloudProfile.Status // can only be changed by status subresource

	dropInactiveVersions(newNamespacedCloudProfile, oldNamespacedCloudProfile)
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

// WarningsOnCreate returns warnings to the client performing the creation.
func (namespacedCloudProfileStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (namespacedCloudProfileStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

// dropInactiveVersions drops expired versions that are not already present in the NamespacedCloudProfile.
func dropInactiveVersions(newProfile, oldProfile *core.NamespacedCloudProfile) {
	dropInactiveKubernetesVersions(newProfile, oldProfile)
	dropInactiveMachineImageVersions(newProfile, oldProfile)
}

// dropInactiveKubernetesVersions drops expired Kubernetes versions that are not already present in the NamespacedCloudProfile.
func dropInactiveKubernetesVersions(newProfile, oldProfile *core.NamespacedCloudProfile) {
	var existingKubernetesVersions map[string]core.ExpirableVersion
	if oldProfile != nil && oldProfile.Spec.Kubernetes != nil {
		existingKubernetesVersions = utils.CreateMapFromSlice(oldProfile.Spec.Kubernetes.Versions, func(v core.ExpirableVersion) string { return v.Version })
	}
	if newProfile.Spec.Kubernetes != nil {
		var validKubernetesVersions []core.ExpirableVersion

		for _, version := range newProfile.Spec.Kubernetes.Versions {
			if _, exists := existingKubernetesVersions[version.Version]; !exists &&
				!helper.CurrentLifecycleClassification(version).IsActive() {
				continue
			}
			validKubernetesVersions = append(validKubernetesVersions, version)
		}

		newProfile.Spec.Kubernetes.Versions = validKubernetesVersions
	}
}

// dropInactiveMachineImageVersions drops expired MachineImage versions that are not already present in the NamespacedCloudProfile.
func dropInactiveMachineImageVersions(newProfile, oldProfile *core.NamespacedCloudProfile) {
	var existingMachineImages []core.MachineImage
	if oldProfile != nil {
		existingMachineImages = oldProfile.Spec.MachineImages
	}
	existingMachineImageVersions := gardenerutils.NewCoreImagesContext(existingMachineImages)
	var validMachineImages []core.MachineImage
	for i, machineImage := range newProfile.Spec.MachineImages {
		var validMachineImageVersions []core.MachineImageVersion

		for _, version := range machineImage.Versions {
			if _, exists := existingMachineImageVersions.GetImageVersion(machineImage.Name, version.Version); !exists &&
				!helper.CurrentLifecycleClassification(version.ExpirableVersion).IsActive() {
				continue
			}
			validMachineImageVersions = append(validMachineImageVersions, version)
		}
		if len(validMachineImageVersions) > 0 || ptr.Deref(machineImage.UpdateStrategy, "") != "" {
			newProfile.Spec.MachineImages[i].Versions = validMachineImageVersions
			validMachineImages = append(validMachineImages, newProfile.Spec.MachineImages[i])
		}
	}
	newProfile.Spec.MachineImages = validMachineImages
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
