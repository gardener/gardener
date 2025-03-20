// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizer_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/state/finalizer"
	"github.com/gardener/gardener/pkg/controllerutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ShootState Finalizer Reconciler", func() {
	const (
		defaultShootName = "shoot-1"
		defaultNamespace = "namespace-1"
		defaultUuid      = "uuid-1"
	)
	var (
		ctx context.Context
		c   client.Client

		shoot      *gardencorev1beta1.Shoot
		reconciler *finalizer.Reconciler

		lastOpMigrateProcessing func() *gardencorev1beta1.LastOperation
		lastOpRestoreProcessing func() *gardencorev1beta1.LastOperation
		lastOpRestoreSucceeded  func() *gardencorev1beta1.LastOperation

		defaultShootWith               func(*gardencorev1beta1.LastOperation) *gardencorev1beta1.Shoot
		defaultShootState              func() *gardencorev1beta1.ShootState
		defaultShootStateWithFinalizer func()
		defaultReconciliationRequest   func() reconcile.Request
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		reconciler = &finalizer.Reconciler{
			Client: c,
		}

		lastOpMigrateProcessing = func() *gardencorev1beta1.LastOperation {
			op := &gardencorev1beta1.LastOperation{
				Type:     gardencorev1beta1.LastOperationTypeMigrate,
				State:    gardencorev1beta1.LastOperationStateProcessing,
				Progress: 10,
			}
			return op
		}

		lastOpRestoreProcessing = func() *gardencorev1beta1.LastOperation {
			op := &gardencorev1beta1.LastOperation{
				Type:     gardencorev1beta1.LastOperationTypeRestore,
				State:    gardencorev1beta1.LastOperationStateProcessing,
				Progress: 10,
			}
			return op
		}

		lastOpRestoreSucceeded = func() *gardencorev1beta1.LastOperation {
			op := &gardencorev1beta1.LastOperation{
				Type:     gardencorev1beta1.LastOperationTypeRestore,
				State:    gardencorev1beta1.LastOperationStateSucceeded,
				Progress: 100,
			}
			return op
		}

		defaultShootWith = func(op *gardencorev1beta1.LastOperation) *gardencorev1beta1.Shoot {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultShootName,
					Namespace: defaultNamespace,
					UID:       defaultUuid,
				},
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: op,
				},
			}
			return shoot
		}

		defaultShootState = func() *gardencorev1beta1.ShootState {
			shootState := &gardencorev1beta1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultShootName,
					Namespace: defaultNamespace,
					UID:       defaultUuid,
				},
			}
			return shootState
		}

		defaultShootStateWithFinalizer = func() {
			By("Create new default ShootState")
			shootState := defaultShootState()
			Expect(c.Create(ctx, shootState)).To(Succeed())

			By("Add finalizer to the default ShootState")
			err := controllerutils.AddFinalizers(ctx, c, shootState, finalizer.FinalizerName)
			Expect(err).NotTo(HaveOccurred())

			By("Verify that finalizer is added")
			shootStateWithFinalizer := &gardencorev1beta1.ShootState{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), shootStateWithFinalizer)).To(Succeed())
			Expect(shootStateWithFinalizer.GetFinalizers()).To(ConsistOf(finalizer.FinalizerName))
		}

		defaultReconciliationRequest = func() reconcile.Request {
			request := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      defaultShootName,
					Namespace: defaultNamespace,
				},
			}
			return request
		}

	})

	Context("when Shoot last operation has `Migrate` type", func() {
		BeforeEach(func() {
			shoot = defaultShootWith(lastOpMigrateProcessing())
			Expect(c.Create(ctx, shoot)).To(Succeed())
		})

		It("should add finalizer if not present", func() {
			By("Create new default ShootState")
			shootState := defaultShootState()
			Expect(c.Create(ctx, shootState)).To(Succeed())

			By("Run reconciliation")
			reconciliationRequest := defaultReconciliationRequest()
			_, err := reconciler.Reconcile(ctx, reconciliationRequest)
			Expect(err).NotTo(HaveOccurred())

			By("Verify that finalizer is present")
			actualShootState := &gardencorev1beta1.ShootState{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), actualShootState)).To(Succeed())
			Expect(actualShootState.GetFinalizers()).To(ConsistOf(finalizer.FinalizerName))
		})

		It("should not add/duplicate finalizer if already present", func() {
			defaultShootStateWithFinalizer()

			By("Run reconciliation")
			reconciliationRequest := defaultReconciliationRequest()
			_, err := reconciler.Reconcile(ctx, reconciliationRequest)
			Expect(err).NotTo(HaveOccurred())

			By("Verify that only one finalizer is present")
			actualShootState := &gardencorev1beta1.ShootState{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), actualShootState)).To(Succeed())
			Expect(actualShootState.GetFinalizers()).To(HaveLen(1))
			Expect(actualShootState.GetFinalizers()).To(ConsistOf(finalizer.FinalizerName))

		})
	})

	Context("when Shoot last operation has `Restore` type", func() {
		Context("with state `Succeeded`", func() {
			BeforeEach(func() {
				shoot = defaultShootWith(lastOpRestoreSucceeded())
				Expect(c.Create(ctx, shoot)).To(Succeed())
			})

			It("should remove finalizer if present", func() {
				defaultShootStateWithFinalizer()

				By("Run reconciliation")
				reconciliationRequest := defaultReconciliationRequest()
				_, err := reconciler.Reconcile(ctx, reconciliationRequest)
				Expect(err).NotTo(HaveOccurred())

				By("Verify that finalizer is removed")
				actualShootState := &gardencorev1beta1.ShootState{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), actualShootState)).To(Succeed())
				Expect(actualShootState.GetFinalizers()).NotTo(ConsistOf(finalizer.FinalizerName))
			})

			It("should not fail if finalizer is not present", func() {
				By("Create new default ShootState")
				shootState := defaultShootState()
				Expect(c.Create(ctx, shootState)).To(Succeed())

				By("Run reconciliation")
				reconciliationRequest := defaultReconciliationRequest()
				_, err := reconciler.Reconcile(ctx, reconciliationRequest)
				Expect(err).NotTo(HaveOccurred())

				By("Verify that finalizer is not present")
				actualShootState := &gardencorev1beta1.ShootState{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), actualShootState)).To(Succeed())
				Expect(actualShootState.GetFinalizers()).NotTo(ConsistOf(finalizer.FinalizerName))
			})
		})

		Context("with state other than `Succeeded`", func() {
			BeforeEach(func() {
				shoot = defaultShootWith(lastOpRestoreProcessing())
				Expect(c.Create(ctx, shoot)).To(Succeed())
			})

			It("should not remove finalizer", func() {
				defaultShootStateWithFinalizer()

				By("Run reconciliation")
				reconciliationRequest := defaultReconciliationRequest()
				_, err := reconciler.Reconcile(ctx, reconciliationRequest)
				Expect(err).NotTo(HaveOccurred())

				By("Verify that finalizer is present")
				actualShootState := &gardencorev1beta1.ShootState{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), actualShootState)).To(Succeed())
				Expect(actualShootState.GetFinalizers()).To(ConsistOf(finalizer.FinalizerName))
			})
		})
	})
})
