// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package criticalcomponents_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("CriticalComponents tests", func() {
	var (
		resourceName string

		node             *corev1.Node
		daemonSet        *appsv1.DaemonSet
		pod, criticalPod *corev1.Pod
	)

	BeforeEach(func() {
		resourceName = "test-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   resourceName,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: corev1.NodeSpec{
				Taints: []corev1.Taint{
					{
						Key:    "node.gardener.cloud/critical-components-not-ready",
						Effect: "NoSchedule",
					},
				},
			},
		}

		daemonSet = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-daemon-",
				Namespace:    testNamespace.Name,
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test-daemon"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "test-daemon"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "app",
							Image: "app",
						}},
						Tolerations: []corev1.Toleration{{
							Effect:   corev1.TaintEffectNoSchedule,
							Operator: corev1.TolerationOpExists,
						}},
					},
				},
			},
		}

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-pod-",
				Namespace:    testNamespace.Name,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "app",
					Image: "app",
				}},
				NodeName: resourceName,
			},
		}

		criticalPod = pod.DeepCopy()
		metav1.SetMetaDataLabel(&criticalPod.ObjectMeta, "node.gardener.cloud/critical-component", "true")
	})

	JustBeforeEach(func() {
		if daemonSet != nil {
			By("Create DaemonSet for test")
			Expect(testClient.Create(ctx, daemonSet)).To(Succeed())
			log.Info("Create DaemonSet for test", "daemonSetName", daemonSet.Name)

			By("Wait until manager cache has observed DaemonSet")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(daemonSet), daemonSet)
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete DaemonSet")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, daemonSet))).To(Succeed())
			})
		}

		if pod != nil {
			By("Create Pod for test")
			Expect(testClient.Create(ctx, pod)).To(Succeed())
			log.Info("Create Pod for test", "podName", pod.Name)

			By("Wait until manager cache has observed Pod")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete Pod")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, pod))).To(Succeed())
			})
		}

		By("Create Node for test")
		Expect(testClient.Create(ctx, node)).To(Succeed())
		log.Info("Create Node for test", "nodeName", node.Name)

		DeferCleanup(func() {
			By("Delete Node")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, node))).To(Succeed())
		})
	})

	Context("no DaemonSets", func() {
		BeforeEach(func() {
			daemonSet = nil
		})

		It("should remove the taint", func() {
			eventuallyTaintIsRemoved(node)
		})
	})

	Context("no node-critical DaemonSets", func() {
		It("should remove the taint", func() {
			eventuallyTaintIsRemoved(node)
		})
	})

	Context("node-critical DaemonSets", func() {
		BeforeEach(func() {
			metav1.SetMetaDataLabel(&daemonSet.ObjectMeta, "node.gardener.cloud/critical-component", "true")
			daemonSet.Spec.Template.Labels["node.gardener.cloud/critical-component"] = "true"
		})

		It("should remove the taint once the DaemonSet pod got ready", func() {
			consistentlyTaintIsNotRemoved(node)

			By("Create a node-critical daemon Pod")
			Expect(controllerutil.SetControllerReference(daemonSet, criticalPod, testClient.Scheme())).To(Succeed())
			Expect(testClient.Create(ctx, criticalPod)).To(Succeed())

			By("Wait until manager cache has observed Pod")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(criticalPod), criticalPod)
			}).Should(Succeed())

			DeferCleanup(func() {
				By("Delete node-critical Pod")
				Expect(client.IgnoreNotFound(testClient.Delete(ctx, criticalPod))).To(Succeed())
			})

			consistentlyTaintIsNotRemoved(node)

			patchPodReady(criticalPod)

			eventuallyTaintIsRemoved(node)
		})
	})

	Context("non-critical Pods", func() {
		It("should remove the taint if there are no node-critical Pods", func() {
			eventuallyTaintIsRemoved(node)
		})
	})

	Context("critical Pods", func() {
		BeforeEach(func() {
			metav1.SetMetaDataLabel(&pod.ObjectMeta, "node.gardener.cloud/critical-component", "true")
		})

		It("should remove the taint once the Pod got ready", func() {
			consistentlyTaintIsNotRemoved(node)

			patchPodReady(pod)

			eventuallyTaintIsRemoved(node)
		})
	})

	Context("critical Pods with CSINode driver requirement", func() {
		BeforeEach(func() {
			metav1.SetMetaDataLabel(&pod.ObjectMeta, "node.gardener.cloud/critical-component", "true")
			metav1.SetMetaDataAnnotation(&pod.ObjectMeta, "node.gardener.cloud/wait-for-csi-node-driver", "foo.driver.example.com")
		})

		It("should remove the taint once the CSINode object got ready", func() {
			patchPodReady(pod)

			// taint should still not be removed
			consistentlyTaintIsNotRemoved(node)

			createRequiredCSINodeObject(node.Name, "foo.driver.example.com")

			eventuallyTaintIsRemoved(node)
		})
	})
})

func eventuallyTaintIsRemoved(node *corev1.Node) {
	By("Verify that the taint has been removed")
	EventuallyWithOffset(1, func(g Gomega) []corev1.Taint {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		return node.Spec.Taints
	}).ShouldNot(ContainElement(corev1.Taint{
		Key:    "node.gardener.cloud/critical-components-not-ready",
		Effect: "NoSchedule",
	}))
}

func consistentlyTaintIsNotRemoved(node *corev1.Node) {
	By("Verify that the taint has not been removed")
	ConsistentlyWithOffset(1, func(g Gomega) []corev1.Taint {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
		return node.Spec.Taints
	}).Should(ContainElement(corev1.Taint{
		Key:    "node.gardener.cloud/critical-components-not-ready",
		Effect: "NoSchedule",
	}))
}

func patchPodReady(pod *corev1.Pod) {
	By("Patching Pod to be ready")
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Status.Conditions = []corev1.PodCondition{
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		},
	}
	ExpectWithOffset(1, testClient.Status().Patch(ctx, pod, patch)).To(Succeed())
}

func createRequiredCSINodeObject(nodeName, driverName string) {
	By("Creating required CSINode object")
	csiNode := storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: storagev1.CSINodeSpec{
			Drivers: []storagev1.CSINodeDriver{
				{
					Name:   driverName,
					NodeID: string(uuid.NewUUID()),
				},
			},
		},
	}
	ExpectWithOffset(1, testClient.Create(ctx, &csiNode)).To(Succeed())
}
