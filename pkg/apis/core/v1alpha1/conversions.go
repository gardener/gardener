// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

//nolint:revive
package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/apis/core"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(
		SchemeGroupVersion.WithKind("ControllerInstallation"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", core.RegistrationRefName, core.SeedRefName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Shoot"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", core.ShootSeedName, core.ShootCloudProfileName, core.ShootStatusSeedName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	// Add non-generated conversion functions
	if err := scheme.AddConversionFunc((*Seed)(nil), (*core.Seed)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_Seed_To_core_Seed(a.(*Seed), b.(*core.Seed), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*SeedSpec)(nil), (*core.SeedSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_SeedSpec_To_core_SeedSpec(a.(*SeedSpec), b.(*core.SeedSpec), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*SeedNetworks)(nil), (*core.SeedNetworks)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_SeedNetworks_To_core_SeedNetworks(a.(*SeedNetworks), b.(*core.SeedNetworks), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.Seed)(nil), (*Seed)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_Seed_To_v1alpha1_Seed(a.(*core.Seed), b.(*Seed), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.SeedSpec)(nil), (*SeedSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_SeedSpec_To_v1alpha1_SeedSpec(a.(*core.SeedSpec), b.(*SeedSpec), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.SeedNetworks)(nil), (*SeedNetworks)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_SeedNetworks_To_v1alpha1_SeedNetworks(a.(*core.SeedNetworks), b.(*SeedNetworks), scope)
	}); err != nil {
		return err
	}

	return nil
}

func Convert_v1alpha1_Seed_To_core_Seed(in *Seed, out *core.Seed, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_Seed_To_core_Seed(in, out, s); err != nil {
		return err
	}

	out.Spec.Networks.BlockCIDRs = in.Spec.BlockCIDRs

	return nil
}

func Convert_core_Seed_To_v1alpha1_Seed(in *core.Seed, out *Seed, s conversion.Scope) error {
	if err := autoConvert_core_Seed_To_v1alpha1_Seed(in, out, s); err != nil {
		return err
	}

	out.Spec.BlockCIDRs = in.Spec.Networks.BlockCIDRs

	return nil
}

func Convert_core_SeedSpec_To_v1alpha1_SeedSpec(in *core.SeedSpec, out *SeedSpec, s conversion.Scope) error {
	return autoConvert_core_SeedSpec_To_v1alpha1_SeedSpec(in, out, s)
}

func Convert_v1alpha1_SeedSpec_To_core_SeedSpec(in *SeedSpec, out *core.SeedSpec, s conversion.Scope) error {
	return autoConvert_v1alpha1_SeedSpec_To_core_SeedSpec(in, out, s)
}

func Convert_core_SeedNetworks_To_v1alpha1_SeedNetworks(in *core.SeedNetworks, out *SeedNetworks, s conversion.Scope) error {
	return autoConvert_core_SeedNetworks_To_v1alpha1_SeedNetworks(in, out, s)
}

func Convert_v1alpha1_SeedNetworks_To_core_SeedNetworks(in *SeedNetworks, out *core.SeedNetworks, s conversion.Scope) error {
	return autoConvert_v1alpha1_SeedNetworks_To_core_SeedNetworks(in, out, s)
}

func Convert_core_SeedStatus_To_v1alpha1_SeedStatus(in *core.SeedStatus, out *SeedStatus, s conversion.Scope) error {
	return autoConvert_core_SeedStatus_To_v1alpha1_SeedStatus(in, out, s)
}

func removeRoleFromRoles(roles []string, role string) []string {
	var newRoles []string
	for _, r := range roles {
		if r != role {
			newRoles = append(newRoles, r)
		}
	}
	return newRoles
}
