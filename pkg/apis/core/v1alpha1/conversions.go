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

package v1alpha1

import (
	"github.com/gardener/gardener/pkg/apis/garden"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	// Add non-generated conversion functions
	return scheme.AddConversionFuncs(
		Convert_v1alpha1_Seed_To_garden_Seed,
		Convert_garden_Seed_To_v1alpha1_Seed,
	)
}

func Convert_v1alpha1_Seed_To_garden_Seed(in *Seed, out *garden.Seed, s conversion.Scope) error {
	var (
		trueVar  = true
		falseVar = false
	)

	if err := autoConvert_v1alpha1_Seed_To_garden_Seed(in, out, s); err != nil {
		return err
	}

	if a := in.Annotations; a != nil {
		if v, ok := a[garden.MigrationSeedCloudProfile]; ok {
			out.Spec.Cloud.Profile = v
		}

		if v, ok := a[garden.MigrationSeedCloudRegion]; ok {
			out.Spec.Cloud.Region = v
		}
	}

	out.Spec.IngressDomain = in.Spec.DNS.IngressDomain
	out.Spec.Cloud.Region = in.Spec.Provider.Region

	for _, taint := range in.Spec.Taints {
		if taint.Key == SeedTaintProtected {
			out.Spec.Protected = &trueVar
		}
		if taint.Key == SeedTaintInvisible {
			out.Spec.Visible = &falseVar
		}
	}

	return nil
}

func Convert_garden_Seed_To_v1alpha1_Seed(in *garden.Seed, out *Seed, s conversion.Scope) error {
	if err := autoConvert_garden_Seed_To_v1alpha1_Seed(in, out, s); err != nil {
		return err
	}

	if len(in.Spec.Cloud.Profile) > 0 || len(in.Spec.Cloud.Region) > 0 || in.Spec.Volume != nil {
		old := out.Annotations
		out.Annotations = make(map[string]string, len(old)+2)
		for k, v := range old {
			if k != "persistentvolume.garden.sapcloud.io/provider" && k != "persistentvolume.garden.sapcloud.io/minimumSize" {
				out.Annotations[k] = v
			}
		}
	}

	if len(in.Spec.Cloud.Profile) > 0 {
		out.Annotations[garden.MigrationSeedCloudProfile] = in.Spec.Cloud.Profile
	}

	if len(in.Spec.Cloud.Region) > 0 {
		out.Annotations[garden.MigrationSeedCloudRegion] = in.Spec.Cloud.Region
	}

	if len(in.Spec.IngressDomain) > 0 {
		out.Spec.DNS = SeedDNS{
			IngressDomain: in.Spec.IngressDomain,
		}
	}

	if p := in.Spec.Protected; p != nil && *p {
		out.Spec.Taints = append(out.Spec.Taints, SeedTaint{
			Key: SeedTaintProtected,
		})
	}

	if v := in.Spec.Visible; v != nil && !*v {
		out.Spec.Taints = append(out.Spec.Taints, SeedTaint{
			Key: SeedTaintInvisible,
		})
	}

	return nil
}

func Convert_garden_SeedSpec_To_v1alpha1_SeedSpec(in *garden.SeedSpec, out *SeedSpec, s conversion.Scope) error {
	return autoConvert_garden_SeedSpec_To_v1alpha1_SeedSpec(in, out, s)
}

func Convert_v1alpha1_SeedSpec_To_garden_SeedSpec(in *SeedSpec, out *garden.SeedSpec, s conversion.Scope) error {
	return autoConvert_v1alpha1_SeedSpec_To_garden_SeedSpec(in, out, s)
}
