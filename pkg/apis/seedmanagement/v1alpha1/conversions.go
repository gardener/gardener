// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//nolint:revive
package v1alpha1

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(
		SchemeGroupVersion.WithKind("ManagedSeed"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", seedmanagement.ManagedSeedShootName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	return nil
}

func Convert_v1alpha1_GardenletConfig_To_seedmanagement_GardenletConfig(in *GardenletConfig, out *seedmanagement.GardenletConfig, s conversion.Scope) error {
	if in.Config.Object == nil {
		cfg, err := encoding.DecodeGardenletConfigurationFromBytes(in.Config.Raw, false)
		if err != nil {
			return err
		}

		in.Config.Object = cfg
	}
	return autoConvert_v1alpha1_GardenletConfig_To_seedmanagement_GardenletConfig(in, out, s)
}

func Convert_seedmanagement_GardenletConfig_To_v1alpha1_GardenletConfig(in *seedmanagement.GardenletConfig, out *GardenletConfig, s conversion.Scope) error {
	if err := autoConvert_seedmanagement_GardenletConfig_To_v1alpha1_GardenletConfig(in, out, s); err != nil {
		return err
	}
	if out.Config.Raw == nil && out.Config.Object != nil {
		cfg, ok := out.Config.Object.(*gardenletconfigv1alpha1.GardenletConfiguration)
		if !ok {
			return errors.New("unknown gardenlet config object type")
		}

		raw, err := encoding.EncodeGardenletConfigurationToBytes(cfg)
		if err != nil {
			return err
		}

		out.Config.Raw = raw
	}
	return nil
}

func Convert_v1alpha1_GardenletSpec_To_seedmanagement_GardenletSpec(in *GardenletSpec, out *seedmanagement.GardenletSpec, s conversion.Scope) error {
	if in.Config.Object == nil {
		cfg, err := encoding.DecodeGardenletConfigurationFromBytes(in.Config.Raw, false)
		if err != nil {
			return err
		}

		in.Config.Object = cfg
	}
	return autoConvert_v1alpha1_GardenletSpec_To_seedmanagement_GardenletSpec(in, out, s)
}

func Convert_seedmanagement_GardenletSpec_To_v1alpha1_GardenletSpec(in *seedmanagement.GardenletSpec, out *GardenletSpec, s conversion.Scope) error {
	if err := autoConvert_seedmanagement_GardenletSpec_To_v1alpha1_GardenletSpec(in, out, s); err != nil {
		return err
	}
	if out.Config.Raw == nil && out.Config.Object != nil {
		cfg, ok := out.Config.Object.(*gardenletconfigv1alpha1.GardenletConfiguration)
		if !ok {
			return errors.New("unknown gardenlet config object type")
		}

		raw, err := encoding.EncodeGardenletConfigurationToBytes(cfg)
		if err != nil {
			return err
		}

		out.Config.Raw = raw
	}
	return nil
}

func Convert_v1_OCIRepository_To_core_OCIRepository(in *gardencorev1.OCIRepository, out *gardencore.OCIRepository, s conversion.Scope) error {
	return gardencorev1.Convert_v1_OCIRepository_To_core_OCIRepository(in, out, s)
}

func Convert_core_OCIRepository_To_v1_OCIRepository(in *gardencore.OCIRepository, out *gardencorev1.OCIRepository, s conversion.Scope) error {
	return gardencorev1.Convert_core_OCIRepository_To_v1_OCIRepository(in, out, s)
}

func Convert_v1beta1_SeedTemplate_To_core_SeedTemplate(in *gardencorev1beta1.SeedTemplate, out *gardencore.SeedTemplate, s conversion.Scope) error {
	return gardencorev1beta1.Convert_v1beta1_SeedTemplate_To_core_SeedTemplate(in, out, s)
}

func Convert_core_SeedTemplate_To_v1beta1_SeedTemplate(in *gardencore.SeedTemplate, out *gardencorev1beta1.SeedTemplate, s conversion.Scope) error {
	return gardencorev1beta1.Convert_core_SeedTemplate_To_v1beta1_SeedTemplate(in, out, s)
}

func Convert_v1beta1_ShootTemplate_To_core_ShootTemplate(in *gardencorev1beta1.ShootTemplate, out *gardencore.ShootTemplate, s conversion.Scope) error {
	return gardencorev1beta1.Convert_v1beta1_ShootTemplate_To_core_ShootTemplate(in, out, s)
}

func Convert_core_ShootTemplate_To_v1beta1_ShootTemplate(in *gardencore.ShootTemplate, out *gardencorev1beta1.ShootTemplate, s conversion.Scope) error {
	return gardencorev1beta1.Convert_core_ShootTemplate_To_v1beta1_ShootTemplate(in, out, s)
}
