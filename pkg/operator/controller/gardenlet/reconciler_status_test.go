// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	mockgardenletdeployer "github.com/gardener/gardener/pkg/controller/gardenletdeployer/mock"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
)

var _ = Describe("Gardenlet reconciler kstatus conditions", func() {
	var (
		ctx           context.Context
		log           logr.Logger
		virtualClient client.Client
		reconciler    *Reconciler
		gardenlet     *seedmanagementv1alpha1.Gardenlet
		fakeClock     *testclock.FakeClock
		ctrl          *gomock.Controller
		mockActuator  *mockgardenletdeployer.MockInterface
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logr.Discard()
		logf.IntoContext(ctx, log)

		fakeClock = testclock.NewFakeClock(time.Now())

		ctrl = gomock.NewController(GinkgoT())
		mockActuator = mockgardenletdeployer.NewMockInterface(ctrl)

		gardenlet = &seedmanagementv1alpha1.Gardenlet{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-gardenlet",
				Namespace:  "garden",
				Generation: 5,
			},
			Spec: seedmanagementv1alpha1.GardenletSpec{
				Config: runtime.RawExtension{Raw: []byte("{}")},
			},
		}

		virtualClient = fakeclient.NewClientBuilder().
			WithScheme(operatorclient.VirtualScheme).
			WithStatusSubresource(&seedmanagementv1alpha1.Gardenlet{}).
			Build()

		reconciler = &Reconciler{
			VirtualClient:               virtualClient,
			DefaultGardenClusterAddress: "https://garden.local",
			Clock:                       fakeClock,
		}

		Expect(virtualClient.Create(ctx, gardenlet)).To(Succeed())
	})

	Describe("reconciliation start", func() {
		It("sets Reconciling=True and bumps ObservedGeneration atomically", func() {
			var recordedStatuses []seedmanagementv1alpha1.GardenletStatus
			recordingClient := fakeclient.NewClientBuilder().
				WithScheme(operatorclient.VirtualScheme).
				WithStatusSubresource(&seedmanagementv1alpha1.Gardenlet{}).
				WithInterceptorFuncs(interceptor.Funcs{
					SubResourcePatch: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
						if subResourceName == "status" {
							if g, ok := obj.(*seedmanagementv1alpha1.Gardenlet); ok {
								recordedStatuses = append(recordedStatuses, *g.Status.DeepCopy())
							}
						}
						return c.SubResource(subResourceName).Patch(ctx, obj, patch, opts...)
					},
				}).
				Build()

			gardenletCopy := gardenlet.DeepCopy()
			gardenletCopy.ResourceVersion = ""
			Expect(recordingClient.Create(ctx, gardenletCopy)).To(Succeed())
			reconciler.VirtualClient = recordingClient

			mockActuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ logr.Logger, _ client.Object, conditions []gardencorev1beta1.Condition, _ *seedmanagementv1alpha1.GardenletDeployment, _ *runtime.RawExtension, _ seedmanagementv1alpha1.Bootstrap, _ bool) ([]gardencorev1beta1.Condition, error) {
					return conditions, nil
				})

			_, err := reconciler.reconcile(ctx, log, gardenlet, mockActuator)
			Expect(err).NotTo(HaveOccurred())

			Expect(recordedStatuses).NotTo(BeEmpty())
			initialStatus := recordedStatuses[0]

			By("ObservedGeneration must equal Generation at start")
			Expect(initialStatus.ObservedGeneration).To(Equal(gardenlet.Generation))

			By("Reconciling must be True at start")
			Expect(initialStatus.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ConditionReconciling),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))

			By("Stalled must be absent at start")
			Expect(initialStatus.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionStalled),
			})))
		})

		It("clears a pre-existing Stalled condition on start", func() {
			patch := client.MergeFrom(gardenlet.DeepCopy())
			gardenlet.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               gardencorev1beta1.ConditionStalled,
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "Error",
					Message:            "previous error",
					LastTransitionTime: metav1.NewTime(fakeClock.Now()),
					LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
				},
			}
			Expect(virtualClient.Status().Patch(ctx, gardenlet, patch)).To(Succeed())

			mockActuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ logr.Logger, _ client.Object, conditions []gardencorev1beta1.Condition, _ *seedmanagementv1alpha1.GardenletDeployment, _ *runtime.RawExtension, _ seedmanagementv1alpha1.Bootstrap, _ bool) ([]gardencorev1beta1.Condition, error) {
					return conditions, nil
				})

			_, err := reconciler.reconcile(ctx, log, gardenlet, mockActuator)
			Expect(err).NotTo(HaveOccurred())

			updated := &seedmanagementv1alpha1.Gardenlet{}
			Expect(virtualClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), updated)).To(Succeed())

			By("Reconciling must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be True as it is still waiting for Seed registration")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(gardencorev1beta1.ConditionStalled),
				"Status":  Equal(gardencorev1beta1.ConditionTrue),
				"Message": Equal("Waiting for seed registration"),
			})))
		})
	})

	Describe("on reconciliation success (seed already registered)", func() {
		BeforeEach(func() {
			Expect(virtualClient.Create(ctx, &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{Name: gardenlet.Name},
			})).To(Succeed())
		})

		It("removes both Reconciling and Stalled conditions", func() {
			result, err := reconciler.reconcile(ctx, log, gardenlet, mockActuator)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			updated := &seedmanagementv1alpha1.Gardenlet{}
			Expect(virtualClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), updated)).To(Succeed())

			By("ObservedGeneration must equal Generation")
			Expect(updated.Status.ObservedGeneration).To(Equal(gardenlet.Generation))

			By("Reconciling must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionStalled),
			})))

			By("SeedRegistered must be True")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(seedmanagementv1alpha1.SeedRegistered),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))
		})

		It("deletes kubeconfig secret and removes spec ref on success", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardenlet-kubeconfig",
					Namespace: gardenlet.Namespace,
				},
			}
			Expect(virtualClient.Create(ctx, secret)).To(Succeed())

			patch := client.MergeFrom(gardenlet.DeepCopy())
			gardenlet.Spec.KubeconfigSecretRef = &corev1.LocalObjectReference{Name: secret.Name}
			Expect(virtualClient.Patch(ctx, gardenlet, patch)).To(Succeed())

			result, err := reconciler.reconcile(ctx, log, gardenlet, mockActuator)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			updated := &seedmanagementv1alpha1.Gardenlet{}
			Expect(virtualClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), updated)).To(Succeed())

			By("Kubeconfig secret ref must be cleared from spec")
			Expect(updated.Spec.KubeconfigSecretRef).To(BeNil())

			By("Reconciling and Stalled must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionStalled),
			})))
		})
	})

	Describe("on reconciliation failure when cleaning up kubeconfig secret", func() {
		BeforeEach(func() {
			Expect(virtualClient.Create(ctx, &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{Name: gardenlet.Name},
			})).To(Succeed())

			patch := client.MergeFrom(gardenlet.DeepCopy())
			gardenlet.Spec.KubeconfigSecretRef = &corev1.LocalObjectReference{Name: "gardenlet-kubeconfig"}
			Expect(virtualClient.Patch(ctx, gardenlet, patch)).To(Succeed())

			errClient := fakeclient.NewClientBuilder().
				WithScheme(operatorclient.VirtualScheme).
				WithStatusSubresource(&seedmanagementv1alpha1.Gardenlet{}).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.DeleteOption) error {
						if _, ok := obj.(*corev1.Secret); ok {
							return errors.New("cannot delete secret")
						}
						return nil
					},
				}).
				Build()

			Expect(errClient.Create(ctx, &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: gardenlet.Name}})).To(Succeed())
			gardenletCopy := gardenlet.DeepCopy()
			gardenletCopy.ResourceVersion = ""
			Expect(errClient.Create(ctx, gardenletCopy)).To(Succeed())
			reconciler.VirtualClient = errClient
		})

		It("removes Reconciling and sets Stalled=True on secret deletion error", func() {
			_, err := reconciler.reconcile(ctx, log, gardenlet, mockActuator)
			Expect(err).NotTo(HaveOccurred())

			updated := &seedmanagementv1alpha1.Gardenlet{}
			Expect(reconciler.VirtualClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), updated)).To(Succeed())

			By("ObservedGeneration must equal Generation")
			Expect(updated.Status.ObservedGeneration).To(Equal(gardenlet.Generation))

			By("Reconciling must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be True with ReconcileError reason")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(gardencorev1beta1.ConditionStalled),
				"Status":  Equal(gardencorev1beta1.ConditionTrue),
				"Reason":  Equal(gardencorev1beta1.EventReconcileError),
				"Message": ContainSubstring("could not delete kubeconfig secret"),
			})))

			By("SeedRegistered must be False")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(seedmanagementv1alpha1.SeedRegistered),
				"Status": Equal(gardencorev1beta1.ConditionFalse),
			})))
		})
	})

	Describe("on reconciliation failure when setting default garden cluster address", func() {
		It("removes Reconciling and sets Stalled=True when gardenlet config is invalid", func() {
			patch := client.MergeFrom(gardenlet.DeepCopy())
			gardenlet.Spec.Config = runtime.RawExtension{Raw: []byte(`{"apiVersion":"invalid.gardener.cloud/v1","kind":"Invalid"}`)}
			Expect(virtualClient.Patch(ctx, gardenlet, patch)).To(Succeed())

			_, err := reconciler.reconcile(ctx, log, gardenlet, mockActuator)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to set default garden cluster address"))

			updated := &seedmanagementv1alpha1.Gardenlet{}
			Expect(virtualClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), updated)).To(Succeed())

			By("ObservedGeneration must equal Generation")
			Expect(updated.Status.ObservedGeneration).To(Equal(gardenlet.Generation))

			By("Reconciling must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be True with ReconcileError reason")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(gardencorev1beta1.ConditionStalled),
				"Status":  Equal(gardencorev1beta1.ConditionTrue),
				"Reason":  Equal(gardencorev1beta1.EventReconcileError),
				"Message": ContainSubstring("failed to decode gardenlet configuration"),
			})))
		})
	})

	Describe("on reconciliation failure during actuator deployment", func() {
		It("removes Reconciling and sets Stalled=True when actuator fails", func() {
			mockActuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, errors.New("failed to deploy gardenlet chart"))

			_, err := reconciler.reconcile(ctx, log, gardenlet, mockActuator)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not reconcile Gardenlet"))

			updated := &seedmanagementv1alpha1.Gardenlet{}
			Expect(virtualClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), updated)).To(Succeed())

			By("ObservedGeneration must equal Generation")
			Expect(updated.Status.ObservedGeneration).To(Equal(gardenlet.Generation))

			By("Reconciling must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be True with ReconcileError reason")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(gardencorev1beta1.ConditionStalled),
				"Status":  Equal(gardencorev1beta1.ConditionTrue),
				"Reason":  Equal(gardencorev1beta1.EventReconcileError),
				"Message": Equal("failed to deploy gardenlet chart"),
			})))
		})
	})

	Describe("on successful gardenlet deployment waiting for seed registration", func() {
		It("sets Stalled and removes Reconciling while waiting for seed registration", func() {
			mockActuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, _ logr.Logger, _ client.Object, conditions []gardencorev1beta1.Condition, _ *seedmanagementv1alpha1.GardenletDeployment, _ *runtime.RawExtension, _ seedmanagementv1alpha1.Bootstrap, _ bool) ([]gardencorev1beta1.Condition, error) {
					return conditions, nil
				})

			result, err := reconciler.reconcile(ctx, log, gardenlet, mockActuator)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(RequeueDurationSeedIsNotYetRegistered))

			updated := &seedmanagementv1alpha1.Gardenlet{}
			Expect(virtualClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), updated)).To(Succeed())

			By("ObservedGeneration must equal Generation")
			Expect(updated.Status.ObservedGeneration).To(Equal(gardenlet.Generation))

			By("Reconciling must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be True")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ConditionStalled),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))

			By("SeedRegistered must be Progressing")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(seedmanagementv1alpha1.SeedRegistered),
				"Status":  Equal(gardencorev1beta1.ConditionProgressing),
				"Message": Equal("Waiting for seed registration"),
			})))
		})
	})

	Describe("kstatus condition isolation: stale Stalled is replaced on next error", func() {
		It("overwrites a stale Stalled message with the new error on repeated failures", func() {
			By("Pre-seed Stalled condition with old error")
			patch := client.MergeFrom(gardenlet.DeepCopy())
			gardenlet.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               gardencorev1beta1.ConditionStalled,
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "Error",
					Message:            "old error from previous reconcile",
					LastTransitionTime: metav1.NewTime(fakeClock.Now()),
					LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
				},
			}
			Expect(virtualClient.Status().Patch(ctx, gardenlet, patch)).To(Succeed())

			By("Encounter a new error during reconciliation")
			mockActuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, errors.New("new error from actuator"))

			_, err := reconciler.reconcile(ctx, log, gardenlet, mockActuator)
			Expect(err).To(HaveOccurred())

			updated := &seedmanagementv1alpha1.Gardenlet{}
			Expect(virtualClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), updated)).To(Succeed())

			stalledCond := findGardenletCondition(updated.Status.Conditions, gardencorev1beta1.ConditionStalled)
			Expect(stalledCond).NotTo(BeNil())
			Expect(stalledCond.Message).To(ContainSubstring("new error from actuator"),
				"Stalled must have the new error message, not the old one")
			Expect(stalledCond.Message).NotTo(Equal("old error from previous reconcile"))
		})
	})

	Describe("Reconcile", func() {
		It("reconciles via Reconcile request when Seed already exists", func() {
			Expect(virtualClient.Create(ctx, &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{Name: gardenlet.Name},
			})).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(gardenlet),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			updated := &seedmanagementv1alpha1.Gardenlet{}
			Expect(virtualClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), updated)).To(Succeed())

			By("ObservedGeneration must equal Generation")
			Expect(updated.Status.ObservedGeneration).To(Equal(gardenlet.Generation))

			By("Reconciling and Stalled must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionStalled),
			})))

			By("SeedRegistered must be True")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(seedmanagementv1alpha1.SeedRegistered),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))
		})

		It("stops reconciling without error if object is not found", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: "non-existing", Namespace: "garden"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})
})

func findGardenletCondition(conditions []gardencorev1beta1.Condition, condType gardencorev1beta1.ConditionType) *gardencorev1beta1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}
