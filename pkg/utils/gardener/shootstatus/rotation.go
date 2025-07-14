// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstatus

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// StartRotationSSHKeypair start the ssh keypair rotation by mutating rotation status.
func StartRotationSSHKeypair(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateShootSSHKeypairRotation(shoot, func(rotation *gardencorev1beta1.ShootSSHKeypairRotation) {
		rotation.LastInitiationTime = now
	})
}

// StartRotationObservability start the observability passwords rotation by mutating rotation status.
func StartRotationObservability(shoot *gardencorev1beta1.Shoot, now *metav1.Time) {
	v1beta1helper.MutateObservabilityRotation(shoot, func(rotation *gardencorev1beta1.ObservabilityRotation) {
		rotation.LastInitiationTime = now
	})
}
