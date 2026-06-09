// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate_test

import (
	"fmt"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("InPlaceUpdate controller tests", func() {
	BeforeEach(func() {
		DeferCleanup(func() {
			By("Cleanup test pods")
			podList := &corev1.PodList{}
			Expect(testClient.List(ctx, podList, client.MatchingLabels{testID: testRunID})).To(Succeed())
			for i := range podList.Items {
				pod := &podList.Items[i]
				if len(pod.Finalizers) > 0 {
					removeFinalizers(pod)
				}
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, pod, client.GracePeriodSeconds(0)))).To(Succeed())
			}

			By("Cleanup test nodes")
			nodeList := &corev1.NodeList{}
			Expect(testClient.List(ctx, nodeList, client.MatchingLabels{testID: testRunID})).To(Succeed())
			for i := range nodeList.Items {
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, &nodeList.Items[i]))).To(Succeed())
			}
		})
	})

	Context("drain initiation", func() {
		It("should cordon the node and drive the drain to completion when no evictable pods exist", func() {
			node := newNode(uniqueName("simple"), poolDefault, poolDefaultSecretName)
			Expect(testClient.Create(ctx, node)).To(Succeed())
			addNeedsDrain(node)

			By("Wait for the drain to complete in a single reconcile (no pods to evict)")
			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(node.Spec.Unschedulable).To(BeTrue())
				g.Expect(node.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain))
				g.Expect(node.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime))
				g.Expect(findCondition(node)).To(matchCondition(machinev1alpha1.ReadyForUpdate))
			}).Should(Succeed())
		})

		It("should hold the drain in progress while pods are still terminating, and complete once they go away", func() {
			node := newNode(uniqueName("hold"), poolDefault, poolDefaultSecretName)
			Expect(testClient.Create(ctx, node)).To(Succeed())

			pod := newPod(uniqueName("pod"), testNamespace.Name, node.Name, func(p *corev1.Pod) {
				p.Finalizers = []string{finalizerKeepAlive}
			})
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			addNeedsDrain(node)

			By("Wait for cordon and drain-start annotation")
			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(node.Spec.Unschedulable).To(BeTrue())
				g.Expect(node.Annotations).To(HaveKey(v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime))
				g.Expect(node.Annotations).To(HaveKeyWithValue(v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain, "true"))
			}).Should(Succeed())

			By("Wait for the eviction to mark the pod for deletion")
			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
				g.Expect(pod.DeletionTimestamp).NotTo(BeNil())
			}).Should(Succeed())

			By("Confirm the controller does not finish the drain while the pod still exists")
			Consistently(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(node.Annotations).To(HaveKey(v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime))
				g.Expect(findCondition(node)).To(BeNil())
			}, 2*time.Second, 200*time.Millisecond).Should(Succeed())

			By("Remove finalizer and force-delete the pod so the drain can complete")
			removeFinalizers(pod)
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, pod, client.GracePeriodSeconds(0)))).To(Succeed())
			Eventually(func(g Gomega) {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "pod should be gone")
			}).Should(Succeed())

			// Re-trigger reconcile by toggling needs-drain off and on, rather than
			// waiting for the podEvictionRetryInterval (20s).
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				patch := client.MergeFrom(node.DeepCopy())
				delete(node.Annotations, v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain)
				g.Expect(testClient.Patch(ctx, node, patch)).To(Succeed())
			}).Should(Succeed())
			addNeedsDrain(node)

			By("Wait for the drain to finish and ReadyForUpdate to be set")
			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(node.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime))
				g.Expect(node.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain))
				g.Expect(findCondition(node)).To(matchCondition(machinev1alpha1.ReadyForUpdate))
			}).Should(Succeed())
		})

		It("should respect the pool's MaxUnavailable when multiple nodes need drain", func() {
			By("Create three nodes in the multi-pool, all needing drain, each holding a finalizer pod to keep drain in-progress")
			nodes := make([]*corev1.Node, 3)
			for i := range nodes {
				nodes[i] = newNode(uniqueName("multi"), poolMulti, poolMultiSecretName)
				Expect(testClient.Create(ctx, nodes[i])).To(Succeed())
				pod := newPod(uniqueName("pod"), testNamespace.Name, nodes[i].Name, func(p *corev1.Pod) {
					p.Finalizers = []string{finalizerKeepAlive}
				})
				Expect(testClient.Create(ctx, pod)).To(Succeed())
				addNeedsDrain(nodes[i])
			}

			By("Eventually exactly two of the three nodes have started the drain")
			Eventually(func(g Gomega) int {
				return countDrainStarted(g, nodes)
			}).Should(Equal(2))

			By("Confirm the cap is held: never more than two nodes have started the drain")
			Consistently(func(g Gomega) int {
				return countDrainStarted(g, nodes)
			}, 2*time.Second, 200*time.Millisecond).Should(Equal(2))
		})

		It("should force the control-plane pool's MaxUnavailable to 1 even when the spec says 99%", func() {
			nodes := make([]*corev1.Node, 2)
			for i := range nodes {
				nodes[i] = newNode(uniqueName("cp"), poolControlPlane, poolControlPlaneSecretName)
				Expect(testClient.Create(ctx, nodes[i])).To(Succeed())
				pod := newPod(uniqueName("pod"), testNamespace.Name, nodes[i].Name, func(p *corev1.Pod) {
					p.Finalizers = []string{finalizerKeepAlive}
				})
				Expect(testClient.Create(ctx, pod)).To(Succeed())
				addNeedsDrain(nodes[i])
			}

			By("Verify only a single control-plane node ever starts a drain")
			Eventually(func(g Gomega) int {
				return countDrainStarted(g, nodes)
			}).Should(Equal(1))
			Consistently(func(g Gomega) int {
				return countDrainStarted(g, nodes)
			}, 2*time.Second, 200*time.Millisecond).Should(Equal(1))
		})

		It("should treat a node already failed (LabelKeyNodeUpdateResult=failed) as unavailable for the cap", func() {
			failedNode := newNode(uniqueName("failed"), poolDefault, poolDefaultSecretName)
			failedNode.Labels[machinev1alpha1.LabelKeyNodeUpdateResult] = machinev1alpha1.LabelValueNodeUpdateFailed
			Expect(testClient.Create(ctx, failedNode)).To(Succeed())

			pendingNode := newNode(uniqueName("pending"), poolDefault, poolDefaultSecretName)
			Expect(testClient.Create(ctx, pendingNode)).To(Succeed())
			addNeedsDrain(pendingNode)

			Consistently(func(g Gomega) bool {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(pendingNode), pendingNode)).To(Succeed())
				return pendingNode.Spec.Unschedulable
			}, 3*time.Second, 200*time.Millisecond).Should(BeFalse())

			Eventually(func(g Gomega) *corev1.NodeCondition {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(failedNode), failedNode)).To(Succeed())
				return findCondition(failedNode)
			}).Should(matchCondition(machinev1alpha1.UpdateFailed))
		})
	})

	Context("pod skip rules during drain", func() {
		assertNodeReachesReadyForUpdate := func(node *corev1.Node) {
			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(node.Annotations).NotTo(HaveKey(v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime))
				g.Expect(findCondition(node)).To(matchCondition(machinev1alpha1.ReadyForUpdate))
			}).Should(Succeed())
		}

		It("should skip mirror pods", func() {
			node := newNode(uniqueName("mirror"), poolDefault, poolDefaultSecretName)
			Expect(testClient.Create(ctx, node)).To(Succeed())

			pod := newPod(uniqueName("mirror-pod"), testNamespace.Name, node.Name, func(p *corev1.Pod) {
				metav1.SetMetaDataAnnotation(&p.ObjectMeta, corev1.MirrorPodAnnotationKey, "abc")
			})
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			addNeedsDrain(node)
			assertNodeReachesReadyForUpdate(node)
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed(), "mirror pod should still exist")
			Expect(pod.DeletionTimestamp).To(BeNil())
		})

		It("should skip DaemonSet-owned pods", func() {
			node := newNode(uniqueName("ds"), poolDefault, poolDefaultSecretName)
			Expect(testClient.Create(ctx, node)).To(Succeed())

			pod := newPod(uniqueName("ds-pod"), testNamespace.Name, node.Name, func(p *corev1.Pod) {
				p.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "DaemonSet",
					Name:       "fluentd",
					UID:        "00000000-0000-0000-0000-000000000001",
					Controller: new(true),
				}}
			})
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			addNeedsDrain(node)
			assertNodeReachesReadyForUpdate(node)
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			Expect(pod.DeletionTimestamp).To(BeNil())
		})

		It("should skip the gardenlet pod (it must not evict itself)", func() {
			node := newNode(uniqueName("gnl"), poolDefault, poolDefaultSecretName)
			Expect(testClient.Create(ctx, node)).To(Succeed())

			pod := newPod(uniqueName("gardenlet"), testNamespace.Name, node.Name, func(p *corev1.Pod) {
				p.Labels[v1beta1constants.LabelRole] = "gardenlet"
			})
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			addNeedsDrain(node)
			assertNodeReachesReadyForUpdate(node)
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			Expect(pod.DeletionTimestamp).To(BeNil())
		})

		It("should skip the gardener-resource-manager pod (its webhook validates GNA's calls)", func() {
			node := newNode(uniqueName("grm"), poolDefault, poolDefaultSecretName)
			Expect(testClient.Create(ctx, node)).To(Succeed())

			pod := newPod(uniqueName("grm-pod"), testNamespace.Name, node.Name, func(p *corev1.Pod) {
				p.Labels[v1beta1constants.LabelApp] = v1beta1constants.DeploymentNameGardenerResourceManager
			})
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			addNeedsDrain(node)
			assertNodeReachesReadyForUpdate(node)
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			Expect(pod.DeletionTimestamp).To(BeNil())
		})

		It("should skip pods that tolerate the unschedulable taint (they would just reschedule)", func() {
			node := newNode(uniqueName("toler"), poolDefault, poolDefaultSecretName)
			Expect(testClient.Create(ctx, node)).To(Succeed())

			pod := newPod(uniqueName("toler-pod"), testNamespace.Name, node.Name, func(p *corev1.Pod) {
				p.Spec.Tolerations = []corev1.Toleration{{
					Key:      corev1.TaintNodeUnschedulable,
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				}}
			})
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			addNeedsDrain(node)
			assertNodeReachesReadyForUpdate(node)
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			Expect(pod.DeletionTimestamp).To(BeNil())
		})
	})

	Context("update result reporting", func() {
		It("should clean up the node and delete gardener-resource-manager pods after a successful update", func() {
			node := newNode(uniqueName("ok"), poolDefault, poolDefaultSecretName)
			node.Spec.Unschedulable = true
			Expect(testClient.Create(ctx, node)).To(Succeed())

			setReadyForUpdateCondition(node, fakeClock.Now())

			By("Create two GRM pods and one unrelated pod in the control plane namespace")
			grm1 := newPod(uniqueName("grm-1"), testNamespace.Name, "", func(p *corev1.Pod) {
				p.Spec.NodeName = ""
				p.Labels[v1beta1constants.LabelApp] = v1beta1constants.DeploymentNameGardenerResourceManager
			})
			grm2 := newPod(uniqueName("grm-2"), testNamespace.Name, "", func(p *corev1.Pod) {
				p.Spec.NodeName = ""
				p.Labels[v1beta1constants.LabelApp] = v1beta1constants.DeploymentNameGardenerResourceManager
			})
			other := newPod(uniqueName("other"), testNamespace.Name, "", func(p *corev1.Pod) {
				p.Spec.NodeName = ""
			})
			Expect(testClient.Create(ctx, grm1)).To(Succeed())
			Expect(testClient.Create(ctx, grm2)).To(Succeed())
			Expect(testClient.Create(ctx, other)).To(Succeed())

			By("Set update-result=successful to trigger reconcile")
			setUpdateResultLabel(node, machinev1alpha1.LabelValueNodeUpdateSuccessful)

			By("Wait for cleanup: label removed, condition set to UpdateSuccessful, GRM pods deleted")
			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(node.Labels).NotTo(HaveKey(machinev1alpha1.LabelKeyNodeUpdateResult))
				g.Expect(findCondition(node)).To(matchCondition(machinev1alpha1.UpdateSuccessful))
				g.Expect(node.Spec.Unschedulable).To(BeTrue())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				err1 := testClient.Get(ctx, client.ObjectKeyFromObject(grm1), &corev1.Pod{})
				err2 := testClient.Get(ctx, client.ObjectKeyFromObject(grm2), &corev1.Pod{})
				g.Expect(apierrors.IsNotFound(err1)).To(BeTrue(), "grm1 should be deleted")
				g.Expect(apierrors.IsNotFound(err2)).To(BeTrue(), "grm2 should be deleted")
			}).Should(Succeed())

			By("The unrelated pod must still exist")
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(other), other)).To(Succeed())
				g.Expect(other.DeletionTimestamp).To(BeNil())
			}, 1*time.Second, 200*time.Millisecond).Should(Succeed())
		})

		It("should record a failure with the GNA-supplied reason", func() {
			node := newNode(uniqueName("fail-reason"), poolDefault, poolDefaultSecretName)
			Expect(testClient.Create(ctx, node)).To(Succeed())

			By("Pre-set the failure reason annotation")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				patch := client.MergeFrom(node.DeepCopy())
				metav1.SetMetaDataAnnotation(&node.ObjectMeta, machinev1alpha1.AnnotationKeyMachineUpdateFailedReason, "kubelet failed to start")
				g.Expect(testClient.Patch(ctx, node, patch)).To(Succeed())
			}).Should(Succeed())

			setUpdateResultLabel(node, machinev1alpha1.LabelValueNodeUpdateFailed)

			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				cond := findCondition(node)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(corev1.ConditionTrue))
				g.Expect(cond.Reason).To(Equal(machinev1alpha1.UpdateFailed))
				g.Expect(cond.Message).To(Equal("kubelet failed to start"))
				g.Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateFailed))
			}).Should(Succeed())
		})

		It("should record a failure with the default message when no reason annotation is set", func() {
			node := newNode(uniqueName("fail-default"), poolDefault, poolDefaultSecretName)
			Expect(testClient.Create(ctx, node)).To(Succeed())

			setUpdateResultLabel(node, machinev1alpha1.LabelValueNodeUpdateFailed)

			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				cond := findCondition(node)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Reason).To(Equal(machinev1alpha1.UpdateFailed))
				g.Expect(cond.Message).To(Equal("GNA reported in-place update failure"))
			}).Should(Succeed())
		})
	})

	Context("update timeout", func() {
		It("should mark the update failed when the ReadyForUpdate condition exceeds updateTimeout", func() {
			node := newNode(uniqueName("stuck"), poolDefault, poolDefaultSecretName)
			node.Spec.Unschedulable = true
			Expect(testClient.Create(ctx, node)).To(Succeed())
			setReadyForUpdateCondition(node, fakeClock.Now())

			fakeClock.Step(31 * time.Minute)

			By("Re-trigger reconcile via needs-drain toggle")
			addNeedsDrain(node)

			Eventually(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(node.Labels).To(HaveKeyWithValue(machinev1alpha1.LabelKeyNodeUpdateResult, machinev1alpha1.LabelValueNodeUpdateFailed))
				cond := findCondition(node)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(corev1.ConditionTrue))
				g.Expect(cond.Reason).To(Equal(machinev1alpha1.UpdateFailed))
			}).Should(Succeed())
		})

		It("should not mark the update failed when the ReadyForUpdate condition is still within updateTimeout", func() {
			node := newNode(uniqueName("ok-timeout"), poolDefault, poolDefaultSecretName)
			node.Spec.Unschedulable = true
			Expect(testClient.Create(ctx, node)).To(Succeed())
			setReadyForUpdateCondition(node, fakeClock.Now())

			fakeClock.Step(10 * time.Minute)

			By("Re-trigger reconcile via needs-drain toggle")
			addNeedsDrain(node)

			Consistently(func(g Gomega) {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
				g.Expect(node.Labels).NotTo(HaveKey(machinev1alpha1.LabelKeyNodeUpdateResult))
				cond := findCondition(node)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Reason).To(Equal(machinev1alpha1.ReadyForUpdate))
			}, 2*time.Second, 200*time.Millisecond).Should(Succeed())
		})
	})
})

func findCondition(node *corev1.Node) *corev1.NodeCondition {
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == machinev1alpha1.NodeInPlaceUpdate {
			return &node.Status.Conditions[i]
		}
	}
	return nil
}

func matchCondition(reason string) types.GomegaMatcher {
	return WithTransform(func(c *corev1.NodeCondition) (string, error) {
		if c == nil {
			return "", fmt.Errorf("condition is nil")
		}
		if c.Status != corev1.ConditionTrue {
			return "", fmt.Errorf("condition status is %q, want True", c.Status)
		}
		return c.Reason, nil
	}, Equal(reason))
}

func countDrainStarted(g Gomega, nodes []*corev1.Node) int {
	count := 0
	for _, n := range nodes {
		fresh := &corev1.Node{}
		g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(n), fresh)).To(Succeed())
		if fresh.Spec.Unschedulable && fresh.Annotations[v1beta1constants.AnnotationNodeAgentInPlaceUpdateDrainStartTime] != "" {
			count++
		}
	}
	return count
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%s-%s", prefix, testRunID, gardenerutils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:6])
}

func newNode(name, pool, poolSecret string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				testID:                           testRunID,
				v1beta1constants.LabelWorkerPool: pool,
				v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName: poolSecret,
			},
		},
	}
}

func newPod(name, namespace, nodeName string, opts ...func(*corev1.Pod)) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{testID: testRunID},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{Name: "main", Image: "registry.k8s.io/pause:3.10"},
			},
		},
	}
	for _, o := range opts {
		o(pod)
	}
	return pod
}

func removeFinalizers(obj client.Object) {
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Or(Succeed(), BeNotFoundError()))
		if obj.GetDeletionTimestamp() != nil && len(obj.GetFinalizers()) == 0 {
			return
		}
		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		obj.SetFinalizers(nil)
		g.Expect(client.IgnoreNotFound(testClient.Patch(ctx, obj, patch))).To(Succeed())
	}).Should(Succeed())
}

func addNeedsDrain(node *corev1.Node) {
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		patch := client.MergeFrom(node.DeepCopy())
		metav1.SetMetaDataAnnotation(&node.ObjectMeta, v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain, "true")
		g.Expect(testClient.Patch(ctx, node, patch)).To(Succeed())
	}).Should(Succeed())
}

func setUpdateResultLabel(node *corev1.Node, value string) {
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		patch := client.MergeFrom(node.DeepCopy())
		metav1.SetMetaDataLabel(&node.ObjectMeta, machinev1alpha1.LabelKeyNodeUpdateResult, value)
		g.Expect(testClient.Patch(ctx, node, patch)).To(Succeed())
	}).Should(Succeed())
}

func setReadyForUpdateCondition(node *corev1.Node, lastTransition time.Time) {
	Eventually(func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		patch := client.MergeFrom(node.DeepCopy())
		node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{
			Type:               machinev1alpha1.NodeInPlaceUpdate,
			Status:             corev1.ConditionTrue,
			Reason:             machinev1alpha1.ReadyForUpdate,
			Message:            "test set",
			LastTransitionTime: metav1.NewTime(lastTransition),
		})
		g.Expect(testClient.Status().Patch(ctx, node, patch)).To(Succeed())
	}).Should(Succeed())
}
