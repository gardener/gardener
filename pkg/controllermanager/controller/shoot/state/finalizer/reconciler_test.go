// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/state/finalizer"
	"github.com/gardener/gardener/pkg/controllerutils"
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
		shootState *gardencorev1beta1.ShootState
		reconciler *finalizer.Reconciler

		lastOpMigrateProcessing   func() *gardencorev1beta1.LastOperation
		lastOpRestoreProcessing   func() *gardencorev1beta1.LastOperation
		lastOpRestoreSucceeded    func() *gardencorev1beta1.LastOperation
		lastOpReconcileProcessing func() *gardencorev1beta1.LastOperation

		defaultShootWith             func(*gardencorev1beta1.LastOperation) *gardencorev1beta1.Shoot
		defaultReconciliationRequest func() reconcile.Request

		createDefaultShootState              func() *gardencorev1beta1.ShootState
		createDefaultShootStateWithFinalizer func() *gardencorev1beta1.ShootState
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

		lastOpReconcileProcessing = func() *gardencorev1beta1.LastOperation {
			op := &gardencorev1beta1.LastOperation{
				Type:     gardencorev1beta1.LastOperationTypeReconcile,
				State:    gardencorev1beta1.LastOperationStateProcessing,
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

		createDefaultShootState = func() *gardencorev1beta1.ShootState {
			shootState = &gardencorev1beta1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultShootName,
					Namespace: defaultNamespace,
					UID:       defaultUuid,
				},
			}

			By("Create new default ShootState")
			Expect(c.Create(ctx, shootState)).To(Succeed())

			return shootState
		}

		createDefaultShootStateWithFinalizer = func() *gardencorev1beta1.ShootState {
			createDefaultShootState()

			By("Add finalizer to the default ShootState")
			err := controllerutils.AddFinalizers(ctx, c, shootState, finalizer.FinalizerName)
			Expect(err).NotTo(HaveOccurred())

			By("Verify that finalizer is added")
			shootStateWithFinalizer := &gardencorev1beta1.ShootState{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), shootStateWithFinalizer)).To(Succeed())
			Expect(shootStateWithFinalizer.GetFinalizers()).To(ConsistOf(finalizer.FinalizerName))

			return shootStateWithFinalizer
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
			createDefaultShootState()

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
			createDefaultShootStateWithFinalizer()

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
				createDefaultShootStateWithFinalizer()

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
				createDefaultShootState()

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

			It("should add finalizer if not exists", func() {
				createDefaultShootState()

				By("Run reconciliation")
				reconciliationRequest := defaultReconciliationRequest()
				_, err := reconciler.Reconcile(ctx, reconciliationRequest)
				Expect(err).NotTo(HaveOccurred())

				By("Verify that finalizer is present")
				actualShootState := &gardencorev1beta1.ShootState{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(shoot), actualShootState)).To(Succeed())
				Expect(actualShootState.GetFinalizers()).To(ConsistOf(finalizer.FinalizerName))
			})

			It("should not remove finalizer", func() {
				createDefaultShootStateWithFinalizer()

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

	Context("when Shoot last operation has `Reconcile` type", func() {
		BeforeEach(func() {
			shoot = defaultShootWith(lastOpReconcileProcessing())
			Expect(c.Create(ctx, shoot)).To(Succeed())
		})

		It("should remove finalizer regardless of the last operation state", func() {
			createDefaultShootStateWithFinalizer()

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
})
