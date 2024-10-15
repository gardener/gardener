// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprofile

import (
	"context"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/validation"
)

type cloudProfileStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy defines the storage strategy for CloudProfiles.
var Strategy = cloudProfileStrategy{api.Scheme, names.SimpleNameGenerator}

func (cloudProfileStrategy) NamespaceScoped() bool {
	return false
}

func (cloudProfileStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	cloudProfile := obj.(*core.CloudProfile)

	dropExpiredVersions(cloudProfile)
}

func (cloudProfileStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	cloudProfile := obj.(*core.CloudProfile)
	return validation.ValidateCloudProfile(cloudProfile)
}

func (cloudProfileStrategy) Canonicalize(obj runtime.Object) {
	cloudProfile := obj.(*core.CloudProfile)

	translateLegacyAccessRestrictionLabels(cloudProfile)
}

func (cloudProfileStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (cloudProfileStrategy) PrepareForUpdate(_ context.Context, newObj, oldObj runtime.Object) {
	_ = oldObj.(*core.CloudProfile)
	_ = newObj.(*core.CloudProfile)
}

func (cloudProfileStrategy) AllowUnconditionalUpdate() bool {
	return true
}

func (cloudProfileStrategy) ValidateUpdate(_ context.Context, newObj, oldObj runtime.Object) field.ErrorList {
	oldProfile, newProfile := oldObj.(*core.CloudProfile), newObj.(*core.CloudProfile)
	return validation.ValidateCloudProfileUpdate(newProfile, oldProfile)
}

// WarningsOnCreate returns warnings to the client performing a create.
func (cloudProfileStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns warnings to the client performing the update.
func (cloudProfileStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func dropExpiredVersions(cloudProfile *core.CloudProfile) {
	var validKubernetesVersions []core.ExpirableVersion

	for _, version := range cloudProfile.Spec.Kubernetes.Versions {
		if version.ExpirationDate != nil && version.ExpirationDate.Time.Before(time.Now()) {
			continue
		}
		validKubernetesVersions = append(validKubernetesVersions, version)
	}

	cloudProfile.Spec.Kubernetes.Versions = validKubernetesVersions

	for i, machineImage := range cloudProfile.Spec.MachineImages {
		var validMachineImageVersions []core.MachineImageVersion

		for _, version := range machineImage.Versions {
			if version.ExpirationDate != nil && version.ExpirationDate.Time.Before(time.Now()) {
				continue
			}
			validMachineImageVersions = append(validMachineImageVersions, version)
		}

		cloudProfile.Spec.MachineImages[i].Versions = validMachineImageVersions
	}
}

func translateLegacyAccessRestrictionLabels(cloudProfile *core.CloudProfile) {
	for i, region := range cloudProfile.Spec.Regions {
		if region.Labels["seed.gardener.cloud/eu-access"] == "true" {
			if !slices.ContainsFunc(region.AccessRestrictions, func(accessRestriction core.AccessRestriction) bool {
				return accessRestriction.Name == "eu-access-only"
			}) {
				cloudProfile.Spec.Regions[i].AccessRestrictions = append(cloudProfile.Spec.Regions[i].AccessRestrictions, core.AccessRestriction{Name: "eu-access-only"})
			}
		}
	}
}
