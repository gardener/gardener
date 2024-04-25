// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//nolint:revive
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/conversion"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func Convert_v1beta1_SeedTemplate_To_core_SeedTemplate(in *gardencorev1beta1.SeedTemplate, out *gardencore.SeedTemplate, s conversion.Scope) error {
	return gardencorev1beta1.Convert_v1beta1_SeedTemplate_To_core_SeedTemplate(in, out, s)
}

func Convert_core_SeedTemplate_To_v1beta1_SeedTemplate(in *gardencore.SeedTemplate, out *gardencorev1beta1.SeedTemplate, s conversion.Scope) error {
	return gardencorev1beta1.Convert_core_SeedTemplate_To_v1beta1_SeedTemplate(in, out, s)
}
