// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprofile

import (
	"context"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/utils/ptr"
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
	DefaultBasedOnCapabilitiesDefinition(cloudProfile)

	dropExpiredVersions(cloudProfile)
}

func (cloudProfileStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	cloudProfile := obj.(*core.CloudProfile)
	return validation.ValidateCloudProfile(cloudProfile)
}

func (cloudProfileStrategy) Canonicalize(obj runtime.Object) {
	cloudProfile := obj.(*core.CloudProfile)

	syncLegacyAccessRestrictionLabelWithNewField(cloudProfile)
}

func (cloudProfileStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (cloudProfileStrategy) PrepareForUpdate(_ context.Context, newObj, oldObj runtime.Object) {
	oldCloudProfile := oldObj.(*core.CloudProfile)
	newCloudProfile := newObj.(*core.CloudProfile)
	DefaultBasedOnCapabilitiesDefinition(newCloudProfile)

	syncLegacyAccessRestrictionLabelWithNewFieldOnUpdate(newCloudProfile, oldCloudProfile)
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

// TODO(Roncossek): Remove this function after Architecture(s) field is removed from MachineType and MachineImageVersion
func DefaultBasedOnCapabilitiesDefinition(in *core.CloudProfile) {
	// with CapabilitiesDefinition no defaulting for Architecture is required
	// as the capabilities.architecture field is used instead
	if in.Spec.CapabilitiesDefinition != nil {
		return
	}

	for i := range in.Spec.MachineImages {
		machineImage := &in.Spec.MachineImages[i]

		for j := range machineImage.Versions {
			b := &machineImage.Versions[j]
			if len(b.Architectures) == 0 {
				b.Architectures = []string{v1beta1constants.ArchitectureAMD64}
			}
		}

	}

	for i := range in.Spec.MachineTypes {
		machineType := &in.Spec.MachineTypes[i]
		if machineType.Architecture == nil {
			machineType.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)
		}
	}

}

// TODO(rfranzke): Remove everything below this line and the legacy access restriction label after
// https://github.com/gardener/dashboard/issues/2120 has been merged and ~6 months have passed to make sure all clients
// have adapted to the new fields in the specifications, and are rolled out.
func syncLegacyAccessRestrictionLabelWithNewField(cloudProfile *core.CloudProfile) {
	for i, region := range cloudProfile.Spec.Regions {
		if region.Labels["seed.gardener.cloud/eu-access"] == "true" {
			if !slices.ContainsFunc(region.AccessRestrictions, func(accessRestriction core.AccessRestriction) bool {
				return accessRestriction.Name == "eu-access-only"
			}) {
				cloudProfile.Spec.Regions[i].AccessRestrictions = append(cloudProfile.Spec.Regions[i].AccessRestrictions, core.AccessRestriction{Name: "eu-access-only"})
				continue
			}
		}

		if slices.ContainsFunc(region.AccessRestrictions, func(accessRestriction core.AccessRestriction) bool {
			return accessRestriction.Name == "eu-access-only"
		}) {
			if region.Labels == nil {
				cloudProfile.Spec.Regions[i].Labels = make(map[string]string)
			}
			cloudProfile.Spec.Regions[i].Labels["seed.gardener.cloud/eu-access"] = "true"
		}
	}
}

func syncLegacyAccessRestrictionLabelWithNewFieldOnUpdate(cloudProfile, oldCloudProfile *core.CloudProfile) {
	removeAccessRestriction := func(accessRestrictions []core.AccessRestriction, name string) []core.AccessRestriction {
		var updatedAccessRestrictions []core.AccessRestriction
		for _, accessRestriction := range accessRestrictions {
			if accessRestriction.Name != name {
				updatedAccessRestrictions = append(updatedAccessRestrictions, accessRestriction)
			}
		}
		return updatedAccessRestrictions
	}

	for _, oldRegion := range oldCloudProfile.Spec.Regions {
		i := slices.IndexFunc(cloudProfile.Spec.Regions, func(currentRegion core.Region) bool {
			return currentRegion.Name == oldRegion.Name
		})
		if i == -1 {
			continue
		}

		if oldRegion.Labels["seed.gardener.cloud/eu-access"] == "true" &&
			cloudProfile.Spec.Regions[i].Labels["seed.gardener.cloud/eu-access"] != "true" {
			cloudProfile.Spec.Regions[i].AccessRestrictions = removeAccessRestriction(cloudProfile.Spec.Regions[i].AccessRestrictions, "eu-access-only")
		}
	}
}
