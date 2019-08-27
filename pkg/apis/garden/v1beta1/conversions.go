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

	"github.com/gardener/gardener/pkg/apis/garden"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func init() {
	localSchemeBuilder.Register(addConversionFuncs)
}

func Convert_v1beta1_Worker_To_garden_Worker(in *Worker, out *garden.Worker, s conversion.Scope) error {
	autoConvert_v1beta1_Worker_To_garden_Worker(in, out, s)
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
	autoConvert_garden_Worker_To_v1beta1_Worker(in, out, s)
	out.MaxSurge = &in.MaxSurge
	out.MaxUnavailable = &in.MaxUnavailable
	return nil
}

// Convert_v1beta1_MachineVersion_To_garden_MachineVersion
func Convert_v1beta1_MachineImage_To_garden_MachineImage(in *MachineImage, out *garden.MachineImage, s conversion.Scope) error {
	autoConvert_v1beta1_MachineImage_To_garden_MachineImage(in, out, s)
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
	autoConvert_garden_MachineImage_To_v1beta1_MachineImage(in, out, s)
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
	autoConvert_garden_KubernetesConstraints_To_v1beta1_KubernetesConstraints(in, out, s)
	for _, version := range in.OfferedVersions {
		out.Versions = append(out.Versions, version.Version)
	}
	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Shoot"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name",
				"metadata.namespace",
				garden.ShootSeedName:
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

	return nil
}

func Convert_garden_SeedSpec_To_v1beta1_SeedSpec(in *garden.SeedSpec, out *SeedSpec, s conversion.Scope) error {
	return autoConvert_garden_SeedSpec_To_v1beta1_SeedSpec(in, out, s)
}
