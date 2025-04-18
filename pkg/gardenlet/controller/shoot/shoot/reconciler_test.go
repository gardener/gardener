// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx           = context.TODO()
		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		namespace  *corev1.Namespace
		shoot      *gardencorev1beta1.Shoot
		reconciler *Reconciler
		fakeClock  *testclock.FakeClock
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "garden-local"},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot",
				Namespace: "namespace",
			},
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{Name: "worker1", UpdateStrategy: ptr.To(gardencorev1beta1.AutoInPlaceUpdate)},
						{Name: "worker2", UpdateStrategy: ptr.To(gardencorev1beta1.ManualInPlaceUpdate)},
						{Name: "worker3", UpdateStrategy: ptr.To(gardencorev1beta1.AutoRollingUpdate)},
						{Name: "worker4", UpdateStrategy: ptr.To(gardencorev1beta1.AutoInPlaceUpdate)},
						{Name: "worker5", UpdateStrategy: ptr.To(gardencorev1beta1.ManualInPlaceUpdate)},
					},
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				Credentials: &gardencorev1beta1.ShootCredentials{
					Rotation: &gardencorev1beta1.ShootCredentialsRotation{
						CertificateAuthorities: &gardencorev1beta1.CARotation{
							PendingWorkersRollouts: []gardencorev1beta1.PendingWorkersRollout{
								{Name: "worker1"},
								{Name: "worker3"},
								{Name: "worker5"},
							},
						},
						ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
							PendingWorkersRollouts: []gardencorev1beta1.PendingWorkersRollout{
								{Name: "worker2"},
								{Name: "worker3"},
								{Name: "worker4"},
							},
						},
					},
				},
				InPlaceUpdates: &gardencorev1beta1.InPlaceUpdatesStatus{
					PendingWorkerUpdates: &gardencorev1beta1.PendingWorkerUpdates{
						AutoInPlaceUpdate:   []string{"worker1", "worker4"},
						ManualInPlaceUpdate: []string{"worker2", "worker5"},
					},
				},
			},
		}
	})

	Describe("#removeNonExistentPoolsFromPendingWorkersRollouts", func() {
		It("should remove non-existent pools from pending workers rollouts", func() {
			shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{
				{Name: "worker3", UpdateStrategy: ptr.To(gardencorev1beta1.AutoRollingUpdate)},
				{Name: "worker4", UpdateStrategy: ptr.To(gardencorev1beta1.AutoInPlaceUpdate)},
				{Name: "worker5", UpdateStrategy: ptr.To(gardencorev1beta1.ManualInPlaceUpdate)},
			}

			removeNonExistentPoolsFromPendingWorkersRollouts(shoot, false)

			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts).To(ConsistOf(
				gardencorev1beta1.PendingWorkersRollout{Name: "worker3"},
				gardencorev1beta1.PendingWorkersRollout{Name: "worker5"},
			))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts).To(ConsistOf(
				gardencorev1beta1.PendingWorkersRollout{Name: "worker3"},
				gardencorev1beta1.PendingWorkersRollout{Name: "worker4"},
			))

			Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).To(ConsistOf("worker4"))
			Expect(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).To(ConsistOf("worker5"))
		})

		It("should remove all worker pools from pending workers rollouts if hibernation is enabled", func() {
			removeNonExistentPoolsFromPendingWorkersRollouts(shoot, true)

			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts).To(BeEmpty())
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts).To(BeEmpty())
			Expect(shoot.Status.InPlaceUpdates).To(BeNil())
		})
	})

	Describe("#patchShootStatusOperationSuccess", func() {
		BeforeEach(func() {
			gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).Build()

			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			seedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build()

			fakeDateAndTime, _ := time.Parse(time.DateTime, "2024-05-14 19:59:39")
			fakeClock = testclock.NewFakeClock(fakeDateAndTime)

			reconciler = &Reconciler{
				GardenClient:  gardenClient,
				SeedClientSet: seedClientSet,
				Clock:         fakeClock,
			}

			Expect(gardenClient.Create(ctx, namespace)).To(Succeed())
			Expect(gardenClient.Create(ctx, shoot)).To(Succeed())

			DeferCleanup(func() {
				Expect(gardenClient.Delete(ctx, shoot)).To(Succeed())
				Expect(gardenClient.Delete(ctx, namespace)).To(Succeed())
			})
		})

		It("should not set the rotation status to Prepared if current status is Preparing and manual in-place update pending workers are present", func() {
			shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase = gardencorev1beta1.RotationPreparing
			shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase = gardencorev1beta1.RotationPreparing
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, nil, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPreparing))
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).To(BeNil())
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationPreparing))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime).To(BeNil())
		})

		It("should set the rotation status to Prepared if current status is Preparing and manual in-place update pending workers are empty", func() {
			shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase = gardencorev1beta1.RotationPreparing
			shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase = gardencorev1beta1.RotationPreparing
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate = nil
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, nil, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime.UTC()).To(Equal(fakeClock.Now()))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime.UTC()).To(Equal(fakeClock.Now()))
		})

		It("should not set the rotation status to Prepared if current status is WaitingForWorkersRollout and manual in-place update pending workers are present", func() {
			shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts = nil
			shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts = nil
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, nil, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).To(BeNil())
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime).To(BeNil())
		})

		It("should not set the rotation status to Prepared if current status is WaitingForWorkersRollout and certificateAuthorities.PendingWorkersRollouts are present", func() {
			shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate = nil
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, nil, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime).To(BeNil())
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationWaitingForWorkersRollout))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime).To(BeNil())
		})

		It("should set the rotation status to Prepared if current status is WaitingForWorkersRollout and manual in-place update pending workers and certificateAuthorities.PendingWorkersRollouts are empty", func() {
			shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase = gardencorev1beta1.RotationWaitingForWorkersRollout
			shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts = nil
			shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts = nil
			shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate = nil
			Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

			Expect(reconciler.patchShootStatusOperationSuccess(ctx, shoot, nil, gardencorev1beta1.LastOperationTypeReconcile)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
			Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationFinishedTime.UTC()).To(Equal(fakeClock.Now()))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
			Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime.UTC()).To(Equal(fakeClock.Now()))
		})
	})
})
