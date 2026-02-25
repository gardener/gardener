// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerinstallation"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerinstallation/shoot"
)

var _ = Describe("Add", func() {
	var reconciler *controllerinstallation.Reconciler

	BeforeEach(func() {
		reconciler = &controllerinstallation.Reconciler{}
	})

	Context("Mappers", func() {
		var (
			ctx        = context.TODO()
			log        logr.Logger
			fakeClient client.Client
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			reconciler.Client = fakeClient
		})

		Describe("#MapToAllSelfHostedShoots", func() {
			It("should return empty when no shoots exist", func() {
				Expect(MapToAllSelfHostedShoots(log, reconciler)(ctx, nil)).To(BeEmpty())
			})

			It("should return empty because shoots listed as PartialObjectMetadata cannot be type-asserted to Shoot for self-hosted filtering", func() {
				shoot1 := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: "garden"}}
				shoot2 := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "shoot2", Namespace: "garden"}}

				Expect(fakeClient.Create(ctx, shoot1)).To(Succeed())
				Expect(fakeClient.Create(ctx, shoot2)).To(Succeed())

				Expect(MapToAllSelfHostedShoots(log, reconciler)(ctx, nil)).To(BeEmpty())
			})
		})

		Describe("#MapBackupBucketToShoot", func() {
			var (
				backupBucket   *gardencorev1beta1.BackupBucket
				shootName      = "shoot"
				shootNamespace = "garden"
			)

			BeforeEach(func() {
				backupBucket = &gardencorev1beta1.BackupBucket{
					Spec: gardencorev1beta1.BackupBucketSpec{
						ShootRef: &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace},
					},
				}
			})

			It("should return nil when shoot ref is not set", func() {
				backupBucket.Spec.ShootRef = nil
				Expect(MapBackupBucketToShoot(ctx, backupBucket)).To(BeEmpty())
			})

			It("should map to the shoot", func() {
				Expect(MapBackupBucketToShoot(ctx, backupBucket)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: shootName, Namespace: shootNamespace}},
				))
			})
		})

		Describe("#MapBackupEntryToShoot", func() {
			var (
				backupEntry    *gardencorev1beta1.BackupEntry
				shootName      = "shoot"
				shootNamespace = "garden"
			)

			BeforeEach(func() {
				backupEntry = &gardencorev1beta1.BackupEntry{
					Spec: gardencorev1beta1.BackupEntrySpec{
						ShootRef: &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace},
					},
				}
			})

			It("should return nil when shoot ref is not set", func() {
				backupEntry.Spec.ShootRef = nil
				Expect(MapBackupEntryToShoot(ctx, backupEntry)).To(BeEmpty())
			})

			It("should map to the shoot", func() {
				Expect(MapBackupEntryToShoot(ctx, backupEntry)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: shootName, Namespace: shootNamespace}},
				))
			})
		})

		Describe("#MapControllerInstallationToShoot", func() {
			var (
				controllerInstallation *gardencorev1beta1.ControllerInstallation
				shootName              = "shoot"
				shootNamespace         = "garden"
			)

			BeforeEach(func() {
				controllerInstallation = &gardencorev1beta1.ControllerInstallation{
					Spec: gardencorev1beta1.ControllerInstallationSpec{
						ShootRef: &corev1.ObjectReference{Name: shootName, Namespace: shootNamespace},
					},
				}
			})

			It("should return nil when shoot ref is not set", func() {
				controllerInstallation.Spec.ShootRef = nil
				Expect(MapControllerInstallationToShoot(ctx, controllerInstallation)).To(BeEmpty())
			})

			It("should map to the shoot", func() {
				Expect(MapControllerInstallationToShoot(ctx, controllerInstallation)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: shootName, Namespace: shootNamespace}},
				))
			})
		})

		Describe("#MapControllerDeploymentToAllSelfHostedShoots", func() {
			var (
				deploymentName = "deployment"

				controllerDeployment   *gardencorev1.ControllerDeployment
				controllerRegistration *gardencorev1beta1.ControllerRegistration
			)

			BeforeEach(func() {
				controllerDeployment = &gardencorev1.ControllerDeployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName}}
				controllerRegistration = &gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "registration-"},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
							DeploymentRefs: []gardencorev1beta1.DeploymentRef{{Name: deploymentName}},
						},
					},
				}
			})

			It("should return nil because there is no ControllerRegistration referencing the deployment", func() {
				Expect(MapControllerDeploymentToAllSelfHostedShoots(log, reconciler)(ctx, controllerDeployment)).To(BeEmpty())
			})

			It("should return empty because even when a ControllerRegistration references the deployment, shoots listed as PartialObjectMetadata cannot pass the self-hosted filter", func() {
				shoot1 := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "shoot1", Namespace: "garden"}}
				shoot2 := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "shoot2", Namespace: "garden"}}

				Expect(fakeClient.Create(ctx, shoot1)).To(Succeed())
				Expect(fakeClient.Create(ctx, shoot2)).To(Succeed())
				Expect(fakeClient.Create(ctx, controllerRegistration)).To(Succeed())

				Expect(MapControllerDeploymentToAllSelfHostedShoots(log, reconciler)(ctx, controllerDeployment)).To(BeEmpty())
			})
		})
	})
})
