// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot/helper"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("ComputeOperationType", func() {
	var shoot *gardencorev1beta1.Shoot

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: ptr.To("seed"),
			},
			Status: gardencorev1beta1.ShootStatus{
				SeedName:      ptr.To("seed"),
				LastOperation: &gardencorev1beta1.LastOperation{},
			},
		}
	})

	It("should return Create if last operation is not set", func() {
		shoot.Status.LastOperation = nil
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeCreate))
	})

	It("should return Create if last operation is Create Error", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeCreate
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeCreate))
	})

	It("should return Reconcile if last operation is Create Succeeded", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeCreate
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
	})

	It("should return Reconcile if last operation is Restore Succeeded", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeRestore
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
	})

	It("should return Reconcile if last operation is Reconcile Succeeded", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
	})

	It("should return Reconcile if last operation is Reconcile Error", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
	})

	It("should return Reconcile if last operation is Reconcile Aborted", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateAborted
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
	})

	It("should return Migrate if spec.seedName and status.seedName differ", func() {
		shoot.Spec.SeedName = ptr.To("other")
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeMigrate))
	})

	It("should return Migrate if last operation is Migrate Error", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeMigrate
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeMigrate))
	})

	It("should return Restore if last operation is Migrate Succeeded", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeMigrate
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
	})

	It("should return Restore if last operation is Migrate Aborted", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeMigrate
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateAborted
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
	})

	It("should return Restore if last operation is Restore Error", func() {
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeRestore
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
	})

	It("should return Delete if deletionTimestamp is set", func() {
		shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeDelete))
	})

	It("should return Delete if last operation is Delete Error", func() {
		shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
		shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
		Expect(ComputeOperationType(shoot)).To(Equal(gardencorev1beta1.LastOperationTypeDelete))
	})
})

var _ = Describe("GetEtcdDeployTimeout", func() {
	var (
		s              *shoot.Shoot
		defaultTimeout time.Duration
	)

	BeforeEach(func() {
		s = &shoot.Shoot{}
		s.SetInfo(&gardencorev1beta1.Shoot{})
		defaultTimeout = 30 * time.Second
	})

	It("shoot is not marked to have HA control plane", func() {
		Expect(GetEtcdDeployTimeout(s, defaultTimeout)).To(Equal(defaultTimeout))
	})

	It("shoot spec has empty ControlPlane", func() {
		s.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{}
		Expect(GetEtcdDeployTimeout(s, defaultTimeout)).To(Equal(defaultTimeout))
	})

	It("shoot is marked as multi-zonal", func() {
		s.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
			HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeNode}},
		}
		Expect(GetEtcdDeployTimeout(s, defaultTimeout)).To(Equal(etcd.DefaultTimeout))
	})
})

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
			StartRotationETCDEncryptionKey(shoot, &now)
			Expect(shoot).To(Equal(expectedShoot))
		})
	})

	Describe("StartRotationSSHKeypair", func() {
		It("should set the phase to Preparing and update the initiation time", func() {
			expectedShoot := shoot.DeepCopy()
			expectedShoot.Status.Credentials.Rotation.SSHKeypair = &gardencorev1beta1.ShootSSHKeypairRotation{
				LastInitiationTime: &now,
			}
			StartRotationSSHKeypair(shoot, &now)
			Expect(shoot).To(Equal(expectedShoot))
		})
	})

	Describe("StartRotationObservability", func() {
		It("should set the phase to Preparing and update the initiation time", func() {
			expectedShoot := shoot.DeepCopy()
			expectedShoot.Status.Credentials.Rotation.Observability = &gardencorev1beta1.ObservabilityRotation{
				LastInitiationTime: &now,
			}
			StartRotationObservability(shoot, &now)
			Expect(shoot).To(Equal(expectedShoot))
		})
	})
})
