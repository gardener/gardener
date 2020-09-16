// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// VerifySeedReadiness verifies whether the seed is ready.
func VerifySeedReadiness(seed *gardencorev1beta1.Seed) bool {
	if cond := gardencorev1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped); cond == nil || cond.Status != gardencorev1beta1.ConditionTrue {
		return false
	}
	if cond := gardencorev1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedGardenletReady); cond == nil || cond.Status != gardencorev1beta1.ConditionTrue {
		return false
	}
	return true
}
