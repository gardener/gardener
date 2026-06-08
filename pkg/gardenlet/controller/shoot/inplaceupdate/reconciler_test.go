// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate_test

import (
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/inplaceupdate"
)

const (
	poolName       = "worker-pool"
	poolSecretName = "pool-secret-abc"
	cpNamespace    = "shoot--foo--bar"

	drainTimeout  = 20 * time.Minute
	updateTimeout = 30 * time.Minute
)

var _ = Describe("Reconciler", func() {
	var (
		ctx       context.Context
		c         client.Client
		fakeClock *testclock.FakeClock
		now       time.Time

		reconciler *Reconciler
		req        reconcile.Request
	)

	BeforeEach(func() {
		ctx = context.Background()
		now = time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
		fakeClock = testclock.NewFakeClock(now)

		c = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.SeedScheme).
			WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
			WithStatusSubresource(&corev1.Node{}).
			Build()

		reconciler = &Reconciler{
			SeedClient: c,
			Clock:      fakeClock,
			Workers: []gardencorev1beta1.Worker{{
				Name:           poolName,
				Maximum:        5,
				MaxUnavailable: new(intstr.FromInt(1)),
			}},
			ControlPlaneNamespace: cpNamespace,
		}

		req = reconcile.Request{NamespacedName: client.ObjectKey{Name: poolSecretName}}
	})

	Describe("#ShouldSkipPod", func() {
		It("should skip succeeded pods", func() {
			Expect(ShouldSkipPod(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}})).To(BeTrue())
		})

		It("should skip failed pods", func() {
			Expect(ShouldSkipPod(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed}})).To(BeTrue())
		})

		It("should skip mirror pods", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{corev1.MirrorPodAnnotationKey: "abc"},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should skip DaemonSet owned pods", func() {
			isController := true
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{{
					Kind:       "DaemonSet",
					APIVersion: "apps/v1",
					Name:       "ds",
					Controller: &isController,
				}},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should skip gardenlet pods", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1beta1constants.LabelRole: "gardenlet"},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should skip gardener-resource-manager pods", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameGardenerResourceManager},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should skip pods that tolerate the unschedulable taint with Exists operator", func() {
			pod := &corev1.Pod{Spec: corev1.PodSpec{
				Tolerations: []corev1.Toleration{{
					Key:      corev1.TaintNodeUnschedulable,
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				}},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should skip pods with a wildcard NoSchedule toleration", func() {
			pod := &corev1.Pod{Spec: corev1.PodSpec{
				Tolerations: []corev1.Toleration{{
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				}},
			}}
			Expect(ShouldSkipPod(pod)).To(BeTrue())
		})

		It("should not skip a normal pod", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"}}
			Expect(ShouldSkipPod(pod)).To(BeFalse())
		})
	})

	Describe("#NodeIsUnavailableForInPlaceUpdate", func() {
		It("should return true when the node is unschedulable and has the drain start annotation", func() {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime: now.Format(time.RFC3339)},
				},
				Spec: corev1.NodeSpec{Unschedulable: true},
			}
			Expect(NodeIsUnavailableForInPlaceUpdate(node)).To(BeTrue())
		})

		It("should return true when the node is unschedulable and has a non-successful in-place update condition", func() {
			node := &corev1.Node{
				Spec: corev1.NodeSpec{Unschedulable: true},
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
					Type:   machinev1alpha1.NodeInPlaceUpdate,
					Status: corev1.ConditionTrue,
					Reason: machinev1alpha1.ReadyForUpdate,
				}}},
			}
			Expect(NodeIsUnavailableForInPlaceUpdate(node)).To(BeTrue())
		})

		It("should return true when the node has the failed update-result label", func() {
			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					machinev1alpha1.LabelKeyNodeUpdateResult: machinev1alpha1.LabelValueNodeUpdateFailed,
				},
			}}
			Expect(NodeIsUnavailableForInPlaceUpdate(node)).To(BeTrue())
		})

		It("should return false for a schedulable node with no condition or label", func() {
			node := &corev1.Node{}
			Expect(NodeIsUnavailableForInPlaceUpdate(node)).To(BeFalse())
		})

		It("should return false for a successfully updated node (only the successful condition)", func() {
			node := &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
				Type:   machinev1alpha1.NodeInPlaceUpdate,
				Status: corev1.ConditionTrue,
				Reason: machinev1alpha1.UpdateSuccessful,
			}}}}
			Expect(NodeIsUnavailableForInPlaceUpdate(node)).To(BeFalse())
		})
	})

	Describe("#MaxUnavailableForPool", func() {
		It("should return 1 when the pool is not found", func() {
			Expect(reconciler.MaxUnavailableForPool("unknown", 5)).To(Equal(1))
		})

		It("should return 1 when MaxUnavailable is nil", func() {
			reconciler.Workers = []gardencorev1beta1.Worker{{Name: poolName, Maximum: 5}}
			Expect(reconciler.MaxUnavailableForPool(poolName, 5)).To(Equal(1))
		})

		It("should return the absolute MaxUnavailable", func() {
			reconciler.Workers = []gardencorev1beta1.Worker{{
				Name:           poolName,
				Maximum:        5,
				MaxUnavailable: new(intstr.FromInt(2)),
			}}
			Expect(reconciler.MaxUnavailableForPool(poolName, 5)).To(Equal(2))
		})

		It("should always return 1 for control-plane pools", func() {
			reconciler.Workers = []gardencorev1beta1.Worker{{
				Name:           poolName,
				Maximum:        5,
				MaxUnavailable: new(intstr.FromInt(3)),
				ControlPlane:   &gardencorev1beta1.WorkerControlPlane{},
			}}
			Expect(reconciler.MaxUnavailableForPool(poolName, 5)).To(Equal(1))
		})

		It("should scale percentage against current node count", func() {
			reconciler.Workers = []gardencorev1beta1.Worker{{
				Name:           poolName,
				Maximum:        10,
				MaxUnavailable: new(intstr.FromString("50%")),
			}}
			Expect(reconciler.MaxUnavailableForPool(poolName, 2)).To(Equal(1))
			Expect(reconciler.MaxUnavailableForPool(poolName, 4)).To(Equal(2))
		})
	})

	Describe("#Reconcile", func() {
		It("should return without error when no nodes belong to the pool", func() {
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should cordon a node and complete the drain when no pods are present", func() {
			node := makeNode("node-1", withNeedsDrain())
			Expect(c.Create(ctx, node)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Spec.Unschedulable).To(BeTrue())
			Expect(node.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime))
			Expect(node.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain))

			cond := findCondition(node)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal(machinev1alpha1.ReadyForUpdate))
		})

		It("should respect maxUnavailable and only cordon one node when limit is 1", func() {
			node1 := makeNode("node-1", withNeedsDrain())
			node2 := makeNode("node-2", withNeedsDrain())
			Expect(c.Create(ctx, node1)).To(Succeed())
			Expect(c.Create(ctx, node2)).To(Succeed())

			// Add an evictable pod to each so they don't immediately complete.
			Expect(c.Create(ctx, makePod("p1", "node-1"))).To(Succeed())
			Expect(c.Create(ctx, makePod("p2", "node-2"))).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).NotTo(BeZero())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node1), node1)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(node2), node2)).To(Succeed())

			cordoned := 0
			for _, n := range []*corev1.Node{node1, node2} {
				if n.Spec.Unschedulable {
					cordoned++
				}
			}
			Expect(cordoned).To(Equal(1))
		})

		It("should force-delete remaining pods after the drain timeout", func() {
			node := makeNode("node-1", func(n *corev1.Node) {
				n.Spec.Unschedulable = true
				metav1.SetMetaDataAnnotation(&n.ObjectMeta, v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime,
					now.Add(-drainTimeout-time.Second).Format(time.RFC3339))
			})
			Expect(c.Create(ctx, node)).To(Succeed())

			pod := makePod("stuck-pod", "node-1")
			Expect(c.Create(ctx, pod)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			err = c.Get(ctx, client.ObjectKeyFromObject(pod), &corev1.Pod{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected pod to be force-deleted")

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			cond := findCondition(node)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal(machinev1alpha1.ReadyForUpdate))
		})

		It("should mark the update as failed when the update timeout is exceeded", func() {
			node := makeNode("node-1", func(n *corev1.Node) {
				n.Spec.Unschedulable = true
				n.Status.Conditions = []corev1.NodeCondition{{
					Type:               machinev1alpha1.NodeInPlaceUpdate,
					Status:             corev1.ConditionTrue,
					Reason:             machinev1alpha1.ReadyForUpdate,
					LastTransitionTime: metav1.NewTime(now.Add(-updateTimeout - time.Second)),
				}}
			})
			Expect(c.Create(ctx, node)).To(Succeed())
			node.Status.Conditions = []corev1.NodeCondition{{
				Type:               machinev1alpha1.NodeInPlaceUpdate,
				Status:             corev1.ConditionTrue,
				Reason:             machinev1alpha1.ReadyForUpdate,
				LastTransitionTime: metav1.NewTime(now.Add(-updateTimeout - time.Second)),
			}}
			Expect(c.Status().Update(ctx, node)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Labels[machinev1alpha1.LabelKeyNodeUpdateResult]).To(Equal(machinev1alpha1.LabelValueNodeUpdateFailed))
			cond := findCondition(node)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal(machinev1alpha1.UpdateFailed))
		})

		It("should requeue close to the update timeout when a node is waiting for ReadyForUpdate", func() {
			node := makeNode("node-1")
			Expect(c.Create(ctx, node)).To(Succeed())
			node.Status.Conditions = []corev1.NodeCondition{{
				Type:               machinev1alpha1.NodeInPlaceUpdate,
				Status:             corev1.ConditionTrue,
				Reason:             machinev1alpha1.ReadyForUpdate,
				LastTransitionTime: metav1.NewTime(now),
			}}
			Expect(c.Status().Update(ctx, node)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically("<=", updateTimeout))
			Expect(result.RequeueAfter).To(BeNumerically(">", updateTimeout-5*time.Second))
		})

		It("should clean up after a successful update", func() {
			node := makeNode("node-1", func(n *corev1.Node) {
				metav1.SetMetaDataLabel(&n.ObjectMeta, machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateSuccessful)
			})
			Expect(c.Create(ctx, node)).To(Succeed())

			grmPod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "grm-1",
					Namespace: cpNamespace,
					Labels:    map[string]string{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameGardenerResourceManager},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "grm", Image: "grm:latest"}}},
			}
			Expect(c.Create(ctx, grmPod)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Labels).NotTo(HaveKey(machinev1alpha1.LabelKeyNodeUpdateResult))
			cond := findCondition(node)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal(machinev1alpha1.UpdateSuccessful))

			err = c.Get(ctx, client.ObjectKeyFromObject(grmPod), &corev1.Pod{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected GRM pod to be deleted")
		})

		It("should record the failed reason when GNA reports update failure", func() {
			node := makeNode("node-1", func(n *corev1.Node) {
				metav1.SetMetaDataLabel(&n.ObjectMeta, machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateFailed)
				metav1.SetMetaDataAnnotation(&n.ObjectMeta, machinev1alpha1.AnnotationKeyMachineUpdateFailedReason, "image pull failed")
			})
			Expect(c.Create(ctx, node)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			cond := findCondition(node)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal(machinev1alpha1.UpdateFailed))
			Expect(cond.Message).To(Equal("image pull failed"))
		})

		It("should bump LastTransitionTime when reason changes across update cycles", func() {
			staleTime := now.Add(-(updateTimeout + time.Minute))
			node := makeNode("n-sticky", func(n *corev1.Node) {
				n.Annotations = map[string]string{
					v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain: "true",
				}
				n.Status.Conditions = []corev1.NodeCondition{{
					Type:               machinev1alpha1.NodeInPlaceUpdate,
					Status:             corev1.ConditionTrue,
					Reason:             machinev1alpha1.UpdateSuccessful,
					LastTransitionTime: metav1.NewTime(staleTime),
				}}
			})
			Expect(c.Create(ctx, node)).To(Succeed())
			Expect(c.Status().Update(ctx, node)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			cond := findCondition(node)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Reason).To(Equal(machinev1alpha1.ReadyForUpdate))
			Expect(cond.LastTransitionTime.Time).To(BeTemporally("==", now))
		})

		It("should requeue with the eviction retry interval when a pod eviction is blocked by a PDB", func() {
			interceptingClient := fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithIndex(&corev1.Pod{}, indexer.PodNodeName, indexer.PodNodeNameIndexerFunc).
				WithStatusSubresource(&corev1.Node{}).
				WithInterceptorFuncs(interceptor.Funcs{
					SubResourceCreate: func(_ context.Context, _ client.Client, subResourceName string, _ client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
						if subResourceName == "eviction" {
							return apierrors.NewTooManyRequests("pdb blocks", 1)
						}
						return nil
					},
				}).
				Build()

			reconciler.SeedClient = interceptingClient

			node := makeNode("node-1", withNeedsDrain())
			Expect(interceptingClient.Create(ctx, node)).To(Succeed())
			Expect(interceptingClient.Create(ctx, makePod("p1", "node-1"))).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(20 * time.Second))

			Expect(interceptingClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Spec.Unschedulable).To(BeTrue())
			cond := findCondition(node)
			Expect(cond).To(BeNil())
		})
	})
})

func makeNode(name string, opts ...func(*corev1.Node)) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: poolSecretName,
				v1beta1constants.LabelWorkerPool:                            poolName,
			},
		},
	}
	for _, opt := range opts {
		opt(node)
	}
	return node
}

func makePod(name, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{{
				Name:  "main",
				Image: "nginx:latest",
			}},
		},
	}
}

func withNeedsDrain() func(*corev1.Node) {
	return func(n *corev1.Node) {
		metav1.SetMetaDataAnnotation(&n.ObjectMeta, v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain, "true")
	}
}

func findCondition(node *corev1.Node) *corev1.NodeCondition {
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == machinev1alpha1.NodeInPlaceUpdate {
			return &node.Status.Conditions[i]
		}
	}
	return nil
}
