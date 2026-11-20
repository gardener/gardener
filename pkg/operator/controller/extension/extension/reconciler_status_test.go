// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/version"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension/admission"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension/controllerregistration"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension/runtime"
)

// noopRuntime is a stub runtime.Interface that always succeeds.
type noopRuntime struct {
	deleteErr error
}

func (noopRuntime) Reconcile(_ context.Context, _ logr.Logger, _ *operatorv1alpha1.Extension) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}
func (n noopRuntime) Delete(_ context.Context, _ logr.Logger, _ *operatorv1alpha1.Extension) (reconcile.Result, error) {
	return reconcile.Result{}, n.deleteErr
}

// noopAdmission is a stub admission.Interface that always succeeds.
type noopAdmission struct{}

func (noopAdmission) Reconcile(_ context.Context, _ logr.Logger, _ kubernetes.Interface, _ string, _ *operatorv1alpha1.Extension) error {
	return nil
}
func (noopAdmission) Delete(_ context.Context, _ logr.Logger, _ *operatorv1alpha1.Extension) error {
	return nil
}

// noopControllerRegistration is a stub controllerregistration.Interface that always succeeds.
type noopControllerRegistration struct{}

func (noopControllerRegistration) Reconcile(_ context.Context, _ logr.Logger, _ *operatorv1alpha1.Extension) error {
	return nil
}
func (noopControllerRegistration) Delete(_ context.Context, _ logr.Logger, _ *operatorv1alpha1.Extension) error {
	return nil
}

// Verify the stubs satisfy their interfaces at compile time.
var (
	_ runtime.Interface                = noopRuntime{}
	_ admission.Interface              = noopAdmission{}
	_ controllerregistration.Interface = noopControllerRegistration{}
)

var _ = Describe("Extension reconciler kstatus conditions", func() {
	var (
		ctx           context.Context
		runtimeClient client.Client
		reconciler    *Reconciler
		extension     *operatorv1alpha1.Extension
		garden        *operatorv1alpha1.Garden
		fakeClock     *testclock.FakeClock
		statusPatches []operatorv1alpha1.ExtensionStatus
	)

	BeforeEach(func() {
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())

		fakeClock = testclock.NewFakeClock(time.Now())
		statusPatches = nil

		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-extension",
				Generation: 3,
			},
		}

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-garden",
			},
		}

		runtimeClient = fakeclient.NewClientBuilder().
			WithScheme(operatorclient.RuntimeScheme).
			WithStatusSubresource(&operatorv1alpha1.Extension{}, &operatorv1alpha1.Garden{}).
			WithInterceptorFuncs(interceptor.Funcs{
				SubResourcePatch: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
					if subResourceName == "status" {
						if ext, ok := obj.(*operatorv1alpha1.Extension); ok {
							statusPatches = append(statusPatches, *ext.Status.DeepCopy())
						}
					}
					return c.SubResource(subResourceName).Patch(ctx, obj, patch, opts...)
				},
			}).
			Build()

		reconciler = &Reconciler{
			RuntimeClientSet: fakekubernetes.NewClientSetBuilder().
				WithClient(runtimeClient).
				Build(),
			Clock:                  fakeClock,
			admission:              noopAdmission{},
			controllerRegistration: noopControllerRegistration{},
			runtime:                noopRuntime{},
		}

		Expect(runtimeClient.Create(ctx, extension)).To(Succeed())
		Expect(runtimeClient.Create(ctx, garden)).To(Succeed())
	})

	Describe("on reconciliation when garden is not yet reconciled", func() {
		It("should set Stalled=True condition", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: extension.Name},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(RequeueGardenResourceNotReady))

			updated := &operatorv1alpha1.Extension{}
			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(extension), updated)).To(Succeed())

			By("ObservedGeneration must equal Generation")
			Expect(updated.Status.ObservedGeneration).To(Equal(extension.Generation))

			By("Reconciling must be absent (replaced by Stalled at error)")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be True")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(gardencorev1beta1.ConditionStalled),
				"Status":  Equal(gardencorev1beta1.ConditionTrue),
				"Reason":  Equal(ReasonReconcileFailed),
				"Message": ContainSubstring("garden is not yet successfully reconciled or not up-to-date"),
			})))
		})
	})

	Describe("on deletion", func() {
		It("sets Reconciling while deleting and removes kstatus conditions after success", func() {
			_, err := reconciler.delete(ctx, logr.Discard(), extension, NewConditions(fakeClock, extension.Status))
			Expect(err).NotTo(HaveOccurred())

			Expect(statusPatches).To(HaveLen(2))
			Expect(statusPatches[0].Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ConditionReconciling),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))

			updated := &operatorv1alpha1.Extension{}
			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(extension), updated)).To(Succeed())
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Or(Equal(gardencorev1beta1.ConditionReconciling), Equal(gardencorev1beta1.ConditionStalled)),
			})))
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(operatorv1alpha1.ExtensionInstalled),
				"Status": Equal(gardencorev1beta1.ConditionFalse),
				"Reason": Equal(ReasonDeleteSuccessful),
			})))
		})

		It("sets Stalled with the deletion error", func() {
			reconciler.runtime = noopRuntime{deleteErr: errors.New("runtime deletion failed")}

			_, err := reconciler.delete(ctx, logr.Discard(), extension, NewConditions(fakeClock, extension.Status))
			Expect(err).To(MatchError(ContainSubstring("runtime deletion failed")))

			updated := &operatorv1alpha1.Extension{}
			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(extension), updated)).To(Succeed())
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(gardencorev1beta1.ConditionStalled),
				"Status":  Equal(gardencorev1beta1.ConditionTrue),
				"Reason":  Equal(ReasonDeleteFailed),
				"Message": ContainSubstring("runtime deletion failed"),
			})))
		})
	})

	Describe("on reconciliation when garden is fully reconciled", func() {
		BeforeEach(func() {
			patch := client.MergeFrom(garden.DeepCopy())
			garden.Annotations = map[string]string{
				v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName: "kubeconfig-secret",
			}
			Expect(runtimeClient.Patch(ctx, garden, patch)).To(Succeed())

			statusPatch := client.MergeFrom(garden.DeepCopy())
			garden.Status = operatorv1alpha1.GardenStatus{
				Gardener: &gardencorev1beta1.Gardener{Version: version.Get().GitVersion},
				LastOperation: &gardencorev1beta1.LastOperation{
					Type:     gardencorev1beta1.LastOperationTypeReconcile,
					State:    gardencorev1beta1.LastOperationStateSucceeded,
					Progress: 100,
				},
			}
			Expect(runtimeClient.Status().Patch(ctx, garden, statusPatch)).To(Succeed())

			fakeVirtualClientSet := fakekubernetes.NewClientSetBuilder().
				WithClient(fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()).
				Build()
			reconciler.GardenClientMap = fakeclientmap.NewClientMapBuilder().
				WithClientSetForKey(keys.ForGarden(garden), fakeVirtualClientSet).
				Build()
		})

		It("should remove both Reconciling and Stalled conditions after successfully reconciling", func() {
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: extension.Name},
			})
			Expect(err).NotTo(HaveOccurred())

			updated := &operatorv1alpha1.Extension{}
			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(extension), updated)).To(Succeed())

			By("Reconciling must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be absent")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionStalled),
			})))

			By("Installed must be True")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(operatorv1alpha1.ExtensionInstalled),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))
		})
	})

	Describe("kstatus condition isolation: stale Stalled is replaced on next error", func() {
		It("a stale Stalled condition is replaced when a new error occurs", func() {
			patch := client.MergeFrom(extension.DeepCopy())
			extension.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               gardencorev1beta1.ConditionStalled,
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "Error",
					Message:            "old error from previous reconcile",
					LastTransitionTime: metav1.NewTime(fakeClock.Now()),
					LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
				},
			}
			Expect(runtimeClient.Status().Patch(ctx, extension, patch)).To(Succeed())

			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: extension.Name},
			})

			updated := &operatorv1alpha1.Extension{}
			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(extension), updated)).To(Succeed())

			var stalledCond *gardencorev1beta1.Condition
			for i := range updated.Status.Conditions {
				if updated.Status.Conditions[i].Type == gardencorev1beta1.ConditionStalled {
					stalledCond = &updated.Status.Conditions[i]
					break
				}
			}
			Expect(stalledCond).NotTo(BeNil())
			Expect(stalledCond.Message).NotTo(Equal("old error from previous reconcile"),
				"Stalled must have been replaced with the new error message")
		})
	})

	Describe("when previously Stalled=True", func() {
		It("removes the Stalled condition and sets Reconciling=True upon a subsequent reconciliation run", func() {
			patch := client.MergeFrom(extension.DeepCopy())
			extension.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               gardencorev1beta1.ConditionStalled,
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "Error",
					Message:            "previous error",
					LastTransitionTime: metav1.NewTime(fakeClock.Now()),
					LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
				},
			}
			Expect(runtimeClient.Status().Patch(ctx, extension, patch)).To(Succeed())

			statusPatches = nil

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: extension.Name},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(statusPatches).NotTo(BeEmpty())
			initialStatus := statusPatches[0]

			By("ObservedGeneration must equal Generation")
			Expect(initialStatus.ObservedGeneration).To(Equal(extension.Generation))

			By("Stalled must be absent upon reconciliation run")
			Expect(initialStatus.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionStalled),
			})))

			By("Reconciling must be True upon reconciliation run")
			Expect(initialStatus.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ConditionReconciling),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))
		})
	})
})
