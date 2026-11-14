// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/operator/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	gardencontroller "github.com/gardener/gardener/pkg/operator/controller/garden/garden"
)

var _ = Describe("Garden reconciler kstatus conditions", func() {
	var (
		ctx           context.Context
		runtimeClient client.Client
		reconciler    *gardencontroller.Reconciler
		garden        *operatorv1alpha1.Garden
		fakeClock     *testclock.FakeClock
		statusUpdates []operatorv1alpha1.GardenStatus
	)

	BeforeEach(func() {
		ctx = context.Background()
		logf.IntoContext(ctx, logr.Discard())

		fakeClock = testclock.NewFakeClock(time.Now())
		statusUpdates = nil

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-garden",
				Generation: 5,
			},
			Spec: operatorv1alpha1.GardenSpec{
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     []string{"10.1.0.0/16"},
						Services: []string{"10.2.0.0/16"},
					},
					Ingress: operatorv1alpha1.Ingress{
						Domains:    []operatorv1alpha1.DNSDomain{{Name: "ingress.example.com"}},
						Controller: gardencorev1beta1.IngressController{Kind: "nginx"},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "virtual.example.com"}},
					},
					Gardener: operatorv1alpha1.Gardener{ClusterIdentity: "test"},
					Kubernetes: operatorv1alpha1.Kubernetes{
						// A valid version is required so that updateStatusOperationStart succeeds.
						Version: "1.31.0",
					},
					Maintenance: operatorv1alpha1.Maintenance{
						TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
					Networking: operatorv1alpha1.Networking{
						Services: []string{"100.64.0.0/13"},
					},
				},
			},
		}

		runtimeClient = fakeclient.NewClientBuilder().
			WithScheme(operatorclient.RuntimeScheme).
			WithStatusSubresource(&operatorv1alpha1.Garden{}).
			WithInterceptorFuncs(interceptor.Funcs{
				SubResourceUpdate: func(ctx context.Context, c client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					if subResourceName == "status" {
						if g, ok := obj.(*operatorv1alpha1.Garden); ok {
							statusUpdates = append(statusUpdates, *g.Status.DeepCopy())
						}
					}
					return c.SubResource(subResourceName).Update(ctx, obj, opts...)
				},
			}).
			Build()

		reconciler = &gardencontroller.Reconciler{
			RuntimeClientSet: fakekubernetes.NewClientSetBuilder().
				WithClient(runtimeClient).
				WithVersion("1.31.0").
				Build(),
			Config: operatorconfigv1alpha1.OperatorConfiguration{
				Controllers: operatorconfigv1alpha1.ControllerConfiguration{
					Garden: operatorconfigv1alpha1.GardenControllerConfig{
						SyncPeriod: &metav1.Duration{Duration: time.Minute},
					},
				},
			},
			Identity:        &gardencorev1beta1.Gardener{Version: "1.2.3"},
			GardenNamespace: "garden",
			Clock:           fakeClock,
		}

		Expect(runtimeClient.Create(ctx, garden)).To(Succeed())

		kubeSystem := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}
		Expect(runtimeClient.Create(ctx, kubeSystem)).To(Succeed())
	})

	Describe("on reconciliation start → error", func() {
		It("sets Reconciling=True then transitions to Stalled=True; ObservedGeneration is bumped", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: garden.Name},
			})
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))

			updated := &operatorv1alpha1.Garden{}
			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), updated)).To(Succeed())

			By("ObservedGeneration must equal Generation (set during updateStatusOperationStart)")
			Expect(updated.Status.ObservedGeneration).To(Equal(garden.Generation))

			By("Reconciling must be absent (removed by updateStatusOperationError)")
			Expect(updated.Status.Conditions).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(gardencorev1beta1.ConditionReconciling),
			})))

			By("Stalled must be True (set by updateStatusOperationError)")
			Expect(updated.Status.Conditions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ConditionStalled),
				"Status": Equal(gardencorev1beta1.ConditionTrue),
			})))
		})
	})

	Describe("kstatus condition isolation: stale Stalled is replaced on next error", func() {
		It("a stale Stalled condition is replaced when a new error occurs", func() {
			patch := client.MergeFrom(garden.DeepCopy())
			garden.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               gardencorev1beta1.ConditionStalled,
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "Error",
					Message:            "old error from previous reconcile",
					LastTransitionTime: metav1.NewTime(fakeClock.Now()),
					LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
				},
			}
			Expect(runtimeClient.Status().Patch(ctx, garden, patch)).To(Succeed())

			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: garden.Name},
			})

			updated := &operatorv1alpha1.Garden{}
			Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), updated)).To(Succeed())

			var stalledCond *gardencorev1beta1.Condition
			for i := range updated.Status.Conditions {
				if updated.Status.Conditions[i].Type == gardencorev1beta1.ConditionStalled {
					stalledCond = &updated.Status.Conditions[i]
					break
				}
			}
			Expect(stalledCond).NotTo(BeNil())
			Expect(stalledCond.Message).NotTo(Equal("old error from previous reconcile"),
				"Stalled must have been cleared by start and re-set with the current error")
		})
	})

	Describe("when previously Stalled=True", func() {
		It("removes the Stalled condition and sets Reconciling=True upon a subsequent reconciliation run", func() {
			patch := client.MergeFrom(garden.DeepCopy())
			garden.Status.Conditions = []gardencorev1beta1.Condition{
				{
					Type:               gardencorev1beta1.ConditionStalled,
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "Error",
					Message:            "previous error",
					LastTransitionTime: metav1.NewTime(fakeClock.Now()),
					LastUpdateTime:     metav1.NewTime(fakeClock.Now()),
				},
			}
			Expect(runtimeClient.Status().Patch(ctx, garden, patch)).To(Succeed())

			_, _ = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKey{Name: garden.Name},
			})

			Expect(statusUpdates).NotTo(BeEmpty())
			initialStatus := statusUpdates[0]

			By("ObservedGeneration must equal Generation")
			Expect(initialStatus.ObservedGeneration).To(Equal(garden.Generation))

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
