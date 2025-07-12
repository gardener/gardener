// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstatus_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstatus"
)

var _ = Describe("StartRotation", func() {
	var (
		shoot *gardencorev1beta1.Shoot
		now   metav1.Time
	)

	BeforeEach(func() {
		now = metav1.Now()
		shoot = &gardencorev1beta1.Shoot{
			Status: gardencorev1beta1.ShootStatus{
				Credentials: &gardencorev1beta1.ShootCredentials{
					Rotation: &gardencorev1beta1.ShootCredentialsRotation{},
				},
			},
		}
	})

	Describe("StartRotationETCDEncryptionKey", func() {
		It("should set the phase to Preparing and update the initiation time", func() {
			expectedShoot := shoot.DeepCopy()
			expectedShoot.Status.Credentials.Rotation.ETCDEncryptionKey = &gardencorev1beta1.ETCDEncryptionKeyRotation{
				Phase:                       gardencorev1beta1.RotationPreparing,
				LastInitiationTime:          &now,
				LastInitiationFinishedTime:  nil,
				LastCompletionTriggeredTime: nil,
			}
			shootstatus.StartRotationETCDEncryptionKey(shoot, &now)
			Expect(shoot).To(Equal(expectedShoot))
		})
	})

	Describe("StartRotationSSHKeypair", func() {
		It("should set the phase to Preparing and update the initiation time", func() {
			expectedShoot := shoot.DeepCopy()
			expectedShoot.Status.Credentials.Rotation.SSHKeypair = &gardencorev1beta1.ShootSSHKeypairRotation{
				LastInitiationTime: &now,
			}
			shootstatus.StartRotationSSHKeypair(shoot, &now)
			Expect(shoot).To(Equal(expectedShoot))
		})
	})

	Describe("StartRotationObservability", func() {
		It("should set the phase to Preparing and update the initiation time", func() {
			expectedShoot := shoot.DeepCopy()
			expectedShoot.Status.Credentials.Rotation.Observability = &gardencorev1beta1.ObservabilityRotation{
				LastInitiationTime: &now,
			}
			shootstatus.StartRotationObservability(shoot, &now)
			Expect(shoot).To(Equal(expectedShoot))
		})
	})
})
