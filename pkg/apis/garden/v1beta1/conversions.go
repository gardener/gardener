// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1beta1

import (
	"encoding/json"
	"fmt"
	"strings"

	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func init() {
	localSchemeBuilder.Register(addConversionFuncs)
}

func Convert_v1beta1_Worker_To_garden_Worker(in *Worker, out *garden.Worker, s conversion.Scope) error {
	if err := autoConvert_v1beta1_Worker_To_garden_Worker(in, out, s); err != nil {
		return err
	}

	if in.MaxSurge == nil {
		out.MaxSurge = DefaultWorkerMaxSurge
	} else {
		out.MaxSurge = *in.MaxSurge
	}
	if in.MaxUnavailable == nil {
		out.MaxUnavailable = DefaultWorkerMaxUnavailable
	} else {
		out.MaxUnavailable = *in.MaxUnavailable
	}

	return nil
}

func Convert_garden_Worker_To_v1beta1_Worker(in *garden.Worker, out *Worker, s conversion.Scope) error {
	if err := autoConvert_garden_Worker_To_v1beta1_Worker(in, out, s); err != nil {
		return err
	}

	out.MaxSurge = &in.MaxSurge
	out.MaxUnavailable = &in.MaxUnavailable

	return nil
}

// Convert_v1beta1_MachineVersion_To_garden_MachineVersion
func Convert_v1beta1_MachineImage_To_garden_MachineImage(in *MachineImage, out *garden.MachineImage, s conversion.Scope) error {
	if err := autoConvert_v1beta1_MachineImage_To_garden_MachineImage(in, out, s); err != nil {
		return err
	}

	if len(in.Version) > 0 {
		out.Versions = make([]garden.MachineImageVersion, len(in.Versions)+1)
		out.Versions[0] = garden.MachineImageVersion{
			Version: in.Version,
		}
	} else {
		out.Versions = make([]garden.MachineImageVersion, len(in.Versions))
	}

	for index, externalVersion := range in.Versions {
		internalVersion := &garden.MachineImageVersion{}
		if err := autoConvert_v1beta1_MachineImageVersion_To_garden_MachineImageVersion(&externalVersion, internalVersion, s); err != nil {
			return err
		}
		if len(in.Version) > 0 {
			out.Versions[index+1] = *internalVersion
		} else {
			out.Versions[index] = *internalVersion
		}
	}

	return nil
}

// Convert_garden_MachineImage_To_v1beta1_MachineImage
func Convert_garden_MachineImage_To_v1beta1_MachineImage(in *garden.MachineImage, out *MachineImage, s conversion.Scope) error {
	if err := autoConvert_garden_MachineImage_To_v1beta1_MachineImage(in, out, s); err != nil {
		return err
	}

	out.Versions = make([]MachineImageVersion, len(in.Versions))
	for index, internalVersion := range in.Versions {
		externalVersion := &MachineImageVersion{}
		if err := autoConvert_garden_MachineImageVersion_To_v1beta1_MachineImageVersion(&internalVersion, externalVersion, s); err != nil {
			return err
		}
		out.Versions[index] = *externalVersion
	}

	return nil
}

func Convert_v1beta1_KubernetesConstraints_To_garden_KubernetesConstraints(in *KubernetesConstraints, out *garden.KubernetesConstraints, s conversion.Scope) error {
	out.OfferedVersions = []garden.KubernetesVersion{}
	duplicates := map[string]int{}
	for index, externalVersion := range in.Versions {
		internalVersion := &garden.KubernetesVersion{Version: externalVersion}
		if _, exists := duplicates[externalVersion]; exists {
			continue
		}
		out.OfferedVersions = append(out.OfferedVersions, *internalVersion)
		duplicates[externalVersion] = index
	}
	for _, externalVersion := range in.OfferedVersions {
		internalVersion := &garden.KubernetesVersion{}
		if err := Convert_v1beta1_KubernetesVersion_To_garden_KubernetesVersion(&externalVersion, internalVersion, s); err != nil {
			return err
		}
		if _, exists := duplicates[externalVersion.Version]; exists {
			if externalVersion.ExpirationDate == nil {
				continue
			}
			out.OfferedVersions[duplicates[externalVersion.Version]].ExpirationDate = externalVersion.ExpirationDate
			continue
		}
		out.OfferedVersions = append(out.OfferedVersions, *internalVersion)
	}
	return nil
}

func Convert_garden_KubernetesConstraints_To_v1beta1_KubernetesConstraints(in *garden.KubernetesConstraints, out *KubernetesConstraints, s conversion.Scope) error {
	if err := autoConvert_garden_KubernetesConstraints_To_v1beta1_KubernetesConstraints(in, out, s); err != nil {
		return err
	}

	for _, version := range in.OfferedVersions {
		out.Versions = append(out.Versions, version.Version)
	}

	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Shoot"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", garden.ShootSeedName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	)
}

func Convert_v1beta1_Seed_To_garden_Seed(in *Seed, out *garden.Seed, s conversion.Scope) error {
	if err := autoConvert_v1beta1_Seed_To_garden_Seed(in, out, s); err != nil {
		return err
	}

	if a := in.Annotations; a != nil {
		if v, ok := a[garden.MigrationSeedProviderType]; ok {
			out.Spec.Provider.Type = v
		}
		if v, ok := a[garden.MigrationSeedProviderRegion]; ok {
			out.Spec.Provider.Region = v
		}

		if v, ok := a[garden.MigrationSeedTaints]; ok {
			for _, key := range strings.Split(v, ",") {
				out.Spec.Taints = append(out.Spec.Taints, garden.SeedTaint{
					Key: key,
				})
			}
		}

		volumeMinimumSize, ok := a[garden.MigrationSeedVolumeMinimumSize]
		volumeProviders, ok2 := a[garden.MigrationSeedVolumeProviders]
		legacyVolumeMinimumSizeAnnotationValue, ok3 := a["persistentvolume.garden.sapcloud.io/minimumSize"]
		legacyVolumeProviderAnnotationValue, ok4 := a["persistentvolume.garden.sapcloud.io/provider"]

		if ok || ok2 || ok3 || ok4 {
			out.Spec.Volume = &garden.SeedVolume{}
		}

		if ok {
			quantity, err := resource.ParseQuantity(volumeMinimumSize)
			if err != nil {
				return err
			}
			out.Spec.Volume.MinimumSize = &quantity
		}
		if ok3 {
			quantity, err := resource.ParseQuantity(legacyVolumeMinimumSizeAnnotationValue)
			if err != nil {
				return err
			}
			out.Spec.Volume.MinimumSize = &quantity
		}

		if ok4 {
			out.Spec.Volume.Providers = append(out.Spec.Volume.Providers, garden.SeedVolumeProvider{
				Purpose: garden.SeedVolumeProviderPurposeEtcdMain,
				Name:    legacyVolumeProviderAnnotationValue,
			})
		}
		if ok2 {
			var obj []garden.SeedVolumeProvider
			if err := json.Unmarshal([]byte(volumeProviders), &obj); err != nil {
				return err
			}

			out.Spec.Volume.Providers = append(out.Spec.Volume.Providers, obj...)
		}
	}

	out.Spec.Provider.Region = in.Spec.Cloud.Region

	if p := in.Spec.Protected; p != nil && *p && !helper.TaintsHave(out.Spec.Taints, garden.SeedTaintProtected) {
		out.Spec.Taints = append(out.Spec.Taints, garden.SeedTaint{
			Key: garden.SeedTaintProtected,
		})
	}

	if v := in.Spec.Visible; v != nil && !*v && !helper.TaintsHave(out.Spec.Taints, garden.SeedTaintInvisible) {
		out.Spec.Taints = append(out.Spec.Taints, garden.SeedTaint{
			Key: garden.SeedTaintInvisible,
		})
	}

	return nil
}

func Convert_garden_Seed_To_v1beta1_Seed(in *garden.Seed, out *Seed, s conversion.Scope) error {
	if err := autoConvert_garden_Seed_To_v1beta1_Seed(in, out, s); err != nil {
		return err
	}

	if len(in.Spec.Provider.Type) > 0 || len(in.Spec.Provider.Region) > 0 || in.Spec.Volume != nil {
		old := out.Annotations
		out.Annotations = make(map[string]string, len(old)+3)
		for k, v := range old {
			out.Annotations[k] = v
		}
	}

	if len(in.Spec.Provider.Type) > 0 {
		out.Annotations[garden.MigrationSeedProviderType] = in.Spec.Provider.Type
	}

	if len(in.Spec.Provider.Region) > 0 {
		out.Annotations[garden.MigrationSeedProviderRegion] = in.Spec.Provider.Region
	}

	if v := in.Spec.Volume; v != nil {
		if v.MinimumSize != nil {
			out.Annotations[garden.MigrationSeedVolumeMinimumSize] = v.MinimumSize.String()
			out.Annotations["persistentvolume.garden.sapcloud.io/minimumSize"] = v.MinimumSize.String()
		}

		var volumeProviders []garden.SeedVolumeProvider
		for _, provider := range in.Spec.Volume.Providers {
			if provider.Purpose == garden.SeedVolumeProviderPurposeEtcdMain {
				out.Annotations["persistentvolume.garden.sapcloud.io/provider"] = provider.Name
			} else {
				volumeProviders = append(volumeProviders, provider)
			}
		}

		if len(volumeProviders) > 0 {
			data, err := json.Marshal(volumeProviders)
			if err != nil {
				return err
			}
			out.Annotations[garden.MigrationSeedVolumeProviders] = string(data)
		} else {
			delete(out.Annotations, garden.MigrationSeedVolumeProviders)
		}
	}

	var (
		trueVar   = true
		falseVar  = false
		taintKeys []string
	)

	for _, taint := range in.Spec.Taints {
		taintKeys = append(taintKeys, taint.Key)

		switch taint.Key {
		case garden.SeedTaintProtected:
			out.Spec.Protected = &trueVar
		case garden.SeedTaintInvisible:
			out.Spec.Visible = &falseVar
		}
	}

	if len(taintKeys) > 0 {
		out.Annotations[garden.MigrationSeedTaints] = strings.Join(taintKeys, ",")
	} else {
		delete(out.Annotations, garden.MigrationSeedTaints)
	}

	if out.Spec.Visible == nil {
		out.Spec.Visible = &trueVar
	}
	if out.Spec.Protected == nil {
		out.Spec.Protected = &falseVar
	}

	var (
		defaultPodCIDR             = DefaultPodNetworkCIDR
		defaultServiceCIDR         = DefaultServiceNetworkCIDR
		defaultPodCIDRAlicloud     = DefaultPodNetworkCIDRAlicloud
		defaultServiceCIDRAlicloud = DefaultServiceNetworkCIDRAlicloud
	)

	if out.Spec.Networks.ShootDefaults == nil {
		out.Spec.Networks.ShootDefaults = &ShootNetworks{}
	}

	if v, ok := out.Annotations[garden.MigrationSeedProviderType]; ok && v == "alicloud" {
		if out.Spec.Networks.ShootDefaults.Pods == nil && !gardencorev1alpha1helper.NetworksIntersect(out.Spec.Networks.Pods, defaultPodCIDRAlicloud) {
			out.Spec.Networks.ShootDefaults.Pods = &defaultPodCIDRAlicloud
		}
		if out.Spec.Networks.ShootDefaults.Services == nil && !gardencorev1alpha1helper.NetworksIntersect(out.Spec.Networks.Services, defaultServiceCIDRAlicloud) {
			out.Spec.Networks.ShootDefaults.Services = &defaultServiceCIDRAlicloud
		}
	} else {
		if out.Spec.Networks.ShootDefaults.Pods == nil && !gardencorev1alpha1helper.NetworksIntersect(out.Spec.Networks.Pods, defaultPodCIDR) {
			out.Spec.Networks.ShootDefaults.Pods = &defaultPodCIDR
		}
		if out.Spec.Networks.ShootDefaults.Services == nil && !gardencorev1alpha1helper.NetworksIntersect(out.Spec.Networks.Services, defaultServiceCIDR) {
			out.Spec.Networks.ShootDefaults.Services = &defaultServiceCIDR
		}
	}

	return nil
}

func Convert_garden_SeedSpec_To_v1beta1_SeedSpec(in *garden.SeedSpec, out *SeedSpec, s conversion.Scope) error {
	return autoConvert_garden_SeedSpec_To_v1beta1_SeedSpec(in, out, s)
}

func Convert_v1beta1_SeedSpec_To_garden_SeedSpec(in *SeedSpec, out *garden.SeedSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_SeedSpec_To_garden_SeedSpec(in, out, s)
}

func Convert_v1beta1_ProjectSpec_To_garden_ProjectSpec(in *ProjectSpec, out *garden.ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_v1beta1_ProjectSpec_To_garden_ProjectSpec(in, out, s); err != nil {
		return err
	}

	for _, member := range in.Members {
		out.ProjectMembers = append(out.ProjectMembers, garden.ProjectMember{
			Subject: member,
			Role:    garden.ProjectMemberAdmin,
		})
	}

	for _, viewer := range in.Viewers {
		out.ProjectMembers = append(out.ProjectMembers, garden.ProjectMember{
			Subject: viewer,
			Role:    garden.ProjectMemberViewer,
		})
	}

	return nil
}

func Convert_garden_ProjectSpec_To_v1beta1_ProjectSpec(in *garden.ProjectSpec, out *ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_garden_ProjectSpec_To_v1beta1_ProjectSpec(in, out, s); err != nil {
		return err
	}

	for _, member := range in.ProjectMembers {
		if member.Role == garden.ProjectMemberAdmin {
			out.Members = append(out.Members, member.Subject)
		}
		if member.Role == garden.ProjectMemberViewer {
			out.Viewers = append(out.Viewers, member.Subject)
		}
	}

	return nil
}

func Convert_v1beta1_QuotaSpec_To_garden_QuotaSpec(in *QuotaSpec, out *garden.QuotaSpec, s conversion.Scope) error {
	if err := autoConvert_v1beta1_QuotaSpec_To_garden_QuotaSpec(in, out, s); err != nil {
		return err
	}

	switch in.Scope {
	case QuotaScopeProject:
		out.Scope = corev1.ObjectReference{
			APIVersion: "core.gardener.cloud/v1alpha1",
			Kind:       "Project",
		}
	case QuotaScopeSecret:
		out.Scope = corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
		}
	}

	return nil
}

func Convert_garden_QuotaSpec_To_v1beta1_QuotaSpec(in *garden.QuotaSpec, out *QuotaSpec, s conversion.Scope) error {
	if err := autoConvert_garden_QuotaSpec_To_v1beta1_QuotaSpec(in, out, s); err != nil {
		return err
	}

	if in.Scope.APIVersion == "core.gardener.cloud/v1alpha1" && in.Scope.Kind == "Project" {
		out.Scope = QuotaScopeProject
	}
	if in.Scope.APIVersion == "v1" && in.Scope.Kind == "Secret" {
		out.Scope = QuotaScopeSecret
	}

	return nil
}
