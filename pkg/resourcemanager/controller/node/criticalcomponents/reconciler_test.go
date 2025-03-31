// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package criticalcomponents_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/node/criticalcomponents"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(storagev1.AddToScheme(scheme))
}

var _ = Describe("Reconciler", func() {
	var (
		fakeClient client.Client
		log        logr.Logger
		logBuffer  *gbytes.Buffer
		recorder   *record.FakeRecorder

		node *corev1.Node
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		logBuffer = gbytes.NewBuffer()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(logBuffer))
		recorder = record.NewFakeRecorder(1)

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"kubernetes.io/os": "linux",
				},
				Name: "node-1",
			},
		}
	})

	Describe("AllNodeCriticalDaemonPodsAreScheduled", func() {
		var (
			criticalDaemonSets, nonCriticalDaemonSets []appsv1.DaemonSet
			pods                                      []corev1.Pod
		)

		BeforeEach(func() {
			pods = nil
			criticalDaemonSets = []appsv1.DaemonSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "critical1",
						Namespace: "kube-system",
						Labels: map[string]string{
							"node.gardener.cloud/critical-component": "true",
						},
					},
					Spec: appsv1.DaemonSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"node.gardener.cloud/critical-component": "true",
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "critical2",
						Namespace: "default",
						Labels: map[string]string{
							"node.gardener.cloud/critical-component": "true",
						},
					},
					Spec: appsv1.DaemonSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"node.gardener.cloud/critical-component": "true",
								},
							},
						},
					},
				},
			}
			nonCriticalDaemonSets = []appsv1.DaemonSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "non-critical1",
						Namespace: "kube-system",
						Labels: map[string]string{
							"node.gardener.cloud/critical-component": "false",
						},
					},
					Spec: appsv1.DaemonSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"node.gardener.cloud/critical-component": "false",
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "non-critical2",
						Namespace: "kube-system",
					},
					Spec: appsv1.DaemonSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{},
							},
						},
					},
				},
			}
		})

		It("should return true if there are no DaemonSets", func() {
			Expect(AllNodeCriticalDaemonPodsAreScheduled(log, recorder, node, nil, pods)).To(BeTrue())
		})

		It("should return true if there are no node-critical DaemonSets", func() {
			Expect(AllNodeCriticalDaemonPodsAreScheduled(log, recorder, node, nonCriticalDaemonSets, pods)).To(BeTrue())
		})

		It("should return true if there are no node-critical DaemonSets that should be scheduled to Node", func() {
			criticalDaemonSetCopy := criticalDaemonSets[0].DeepCopy()
			criticalDaemonSetCopy.Spec.Template.Spec.NodeSelector = map[string]string{
				"kubernetes.io/os": "not-linux",
			}
			criticalDaemonSets[0] = *criticalDaemonSetCopy

			Expect(AllNodeCriticalDaemonPodsAreScheduled(log, recorder, node, criticalDaemonSets[0:1], pods)).To(BeTrue())
		})

		It("should return false if there are node-critical DaemonSets but no daemon pods", func() {
			pods = append(pods, nonDaemonPod())

			Expect(AllNodeCriticalDaemonPodsAreScheduled(log, recorder, node, criticalDaemonSets, pods)).To(BeFalse())
			Eventually(logBuffer).Should(gbytes.Say(`not scheduled to Node yet.+\[{"Namespace":"kube-system","Name":"critical1"},{"Namespace":"default","Name":"critical2"}\]`))
		})

		It("should return false if one of the node-critical DaemonSets has no corresponding daemon pod yet", func() {
			pods = append(pods, daemonPodFor(&criticalDaemonSets[0]))

			Expect(AllNodeCriticalDaemonPodsAreScheduled(log, recorder, node, criticalDaemonSets, pods)).To(BeFalse())
			Eventually(logBuffer).Should(gbytes.Say(`not scheduled to Node yet.+\[{"Namespace":"default","Name":"critical2"}\]`))
		})

		It("should return true if there are node-critical DaemonSets with corresponding daemon pods", func() {
			pods = append(pods, daemonPodFor(&criticalDaemonSets[0]), daemonPodFor(&criticalDaemonSets[1]))

			allDaemonSets := append(nonCriticalDaemonSets, criticalDaemonSets...)
			Expect(AllNodeCriticalDaemonPodsAreScheduled(log, recorder, node, allDaemonSets, pods)).To(BeTrue())
		})
	})

	Describe("AllNodeCriticalPodsAreReady", func() {
		var pods []corev1.Pod

		BeforeEach(func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "foo",
				},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					}},
				},
			}

			pod2 := pod.DeepCopy()
			pod2.Name = "pod2"
			pods = []corev1.Pod{*pod, *pod2}
		})

		It("should return true if there are no node-critical pods", func() {
			Expect(AllNodeCriticalPodsAreReady(log, recorder, node, nil)).To(BeTrue())
		})

		It("should return false if there are unready node-critical pods", func() {
			pods[0].Status.Conditions[0].Status = corev1.ConditionFalse

			Expect(AllNodeCriticalPodsAreReady(log, recorder, node, pods)).To(BeFalse())
			Eventually(logBuffer).Should(gbytes.Say(`Unready node-critical Pods.+\[{"Namespace":"foo","Name":"pod1"}\]`))
		})

		It("should return true if there all node-critical pods are ready", func() {
			Expect(AllNodeCriticalPodsAreReady(log, recorder, node, pods)).To(BeTrue())
		})
	})

	Describe("GetRequiredDrivers", func() {
		var pods []corev1.Pod

		BeforeEach(func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "foo",
				},
			}

			pod2 := pod.DeepCopy()
			pod2.Name = "pod2"
			pod3 := pod.DeepCopy()
			pod3.Name = "pod3"
			pods = []corev1.Pod{*pod, *pod2, *pod3}
		})

		It("should return an empty driver set if there are no node-critical pods", func() {
			Expect(GetRequiredDrivers(nil).Len()).To(Equal(0))
		})

		It("should return an empty driver set if there are node-critical pods without the wait-for-csi-node annotation", func() {
			Expect(GetRequiredDrivers(pods).Len()).To(Equal(0))
		})

		It("should return the correct number of drivers if there are node-critical pods with the wait-for-csi-node annotation", func() {
			pods[0].Annotations = map[string]string{
				"node.gardener.cloud/wait-for-csi-node-foo": "foo.driver.example.com",
				"unrelated.k8s.io/something":                "true",
			}
			pods[1].Annotations = map[string]string{
				"node.gardener.cloud/wait-for-csi-node-foo": "foo.driver.example.com", // duplicate driver should only be considered once
			}
			pods[2].Annotations = map[string]string{
				"node.gardener.cloud/wait-for-csi-node-bar": "bar.driver.example.com",
				"unrelated.k8s.io/something-else":           "false",
			}

			Expect(GetRequiredDrivers(pods).Len()).To(Equal(2))
			Expect(GetRequiredDrivers(pods).UnsortedList()).To(ContainElement("foo.driver.example.com"))
			Expect(GetRequiredDrivers(pods).UnsortedList()).To(ContainElement("bar.driver.example.com"))
		})
	})

	Describe("GetExistingDriversFromCSINode", func() {
		var csiNode *storagev1.CSINode

		BeforeEach(func() {
			csiNode = &storagev1.CSINode{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-1", // CSINodes have the same name as the corresponding Node object
				},
			}
		})

		It("should return an empty driver set and no error if there is no CSINode object", func() {
			// Note: we take the name of the node to find the CSINode object
			existingDrivers, err := GetExistingDriversFromCSINode(context.TODO(), fakeClient, client.ObjectKeyFromObject(node))
			Expect(err).ToNot(HaveOccurred())
			Expect(existingDrivers.Len()).To(Equal(0))

		})

		It("should return an empty driver set and no error if there is an empty CSINode object", func() {
			Expect(fakeClient.Create(context.TODO(), csiNode)).To(Succeed())

			// Note: we take the name of the node to find the CSINode object
			existingDrivers, err := GetExistingDriversFromCSINode(context.TODO(), fakeClient, client.ObjectKeyFromObject(node))

			Expect(err).ToNot(HaveOccurred())
			Expect(existingDrivers.Len()).To(Equal(0))
		})

		It("should return a driver set with one driver and no error if there is an CSINode object with one specified driver", func() {
			csiNode.Spec.Drivers = []storagev1.CSINodeDriver{
				{
					Name:   "foo.driver.example.org",
					NodeID: "node-driver-id",
				},
			}
			Expect(fakeClient.Create(context.TODO(), csiNode)).To(Succeed())

			// Note: we take the name of the node to find the CSINode object
			existingDrivers, err := GetExistingDriversFromCSINode(context.TODO(), fakeClient, client.ObjectKeyFromObject(node))

			Expect(err).ToNot(HaveOccurred())
			Expect(existingDrivers.Len()).To(Equal(1))
			Expect(existingDrivers.UnsortedList()[0]).To(Equal("foo.driver.example.org"))
		})
	})

	Describe("AllCSINodeDriversAreReady", func() {
		var requiredDrivers, existingDrivers sets.Set[string]

		BeforeEach(func() {
			requiredDrivers = sets.Set[string]{}
			existingDrivers = sets.Set[string]{}
		})

		It("should return true if there are no required and no existing drivers", func() {
			Expect(AllCSINodeDriversAreReady(log, recorder, node, nil, nil)).To(BeTrue())
		})

		It("should return false if there are some required, but no existing drivers", func() {
			requiredDrivers.Insert("foo.driver.example.com")
			requiredDrivers.Insert("bar.driver.example.com")

			Expect(AllCSINodeDriversAreReady(log, recorder, node, requiredDrivers, nil)).To(BeFalse())
			// note that the order if driver names can vary, therefore we only
			// check that there are exactly two occurrences of *.driver.example.com
			Eventually(logBuffer).Should(gbytes.Say(`Unready required CSI drivers.+(?:foo|bar)\.driver\.example\.com\"\,\"(?:foo|bar)\.driver\.example\.com\"\]`))
		})

		It("should return true if there are some required and matching existing drivers", func() {
			requiredDrivers.Insert("foo.driver.example.com")
			requiredDrivers.Insert("bar.driver.example.com")
			existingDrivers.Insert("foo.driver.example.com")
			existingDrivers.Insert("bar.driver.example.com")
			Expect(AllCSINodeDriversAreReady(log, recorder, node, requiredDrivers, existingDrivers)).To(BeTrue())
		})
	})

	Describe("RemoveTaint", func() {
		var (
			ctx  context.Context
			node *corev1.Node

			c client.Client
		)

		BeforeEach(func() {
			ctx = context.Background()

			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
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

			c = fakeclient.NewClientBuilder().WithObjects(node).Build()
		})

		It("should remove the critical-components-not-ready taint if it's the only taint", func() {
			Expect(RemoveTaint(ctx, c, node)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Spec.Taints).To(BeEmpty())
		})

		It("should remove only the critical-components-not-ready taint", func() {
			node.Spec.Taints = []corev1.Taint{
				{
					Key:    "node.kubernetes.io/not-ready",
					Effect: "NoExecute",
				},
				{
					Key:    "node.gardener.cloud/critical-components-not-ready",
					Effect: "NoSchedule",
				},
				{
					Key:    "node.kubernetes.io/unreachable",
					Effect: "NoExecute",
				},
			}
			Expect(c.Update(ctx, node)).To(Succeed())

			Expect(RemoveTaint(ctx, c, node)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(node), node)).To(Succeed())
			Expect(node.Spec.Taints).To(HaveLen(2))
			Expect(node.Spec.Taints).To(Equal([]corev1.Taint{
				{
					Key:    "node.kubernetes.io/not-ready",
					Effect: "NoExecute",
				},
				{
					Key:    "node.kubernetes.io/unreachable",
					Effect: "NoExecute",
				},
			}))
		})

		It("should patch the node even if it doesn't have the taint", func() {
			mockClient := mockclient.NewMockClient(gomock.NewController(GinkgoT()))
			node.Spec.Taints = nil

			test.EXPECTPatchWithOptimisticLock(ctx, mockClient, node.DeepCopy(), node, types.MergePatchType)

			Expect(RemoveTaint(ctx, mockClient, node)).To(Succeed())
		})
	})
})

func nonDaemonPod() corev1.Pod {
	nameSuffix, err := utils.GenerateRandomString(5)
	Expect(err).NotTo(HaveOccurred())

	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-" + nameSuffix,
			Namespace: "foo",
		},
	}
}

func daemonPodFor(daemonSet *appsv1.DaemonSet) corev1.Pod {
	nameSuffix, err := utils.GenerateRandomString(5)
	Expect(err).NotTo(HaveOccurred())

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      daemonSet.Name + "-" + nameSuffix,
			Namespace: daemonSet.Namespace,
		},
	}

	daemonSet.UID = uuid.NewUUID()
	Expect(controllerutil.SetControllerReference(daemonSet, &pod, scheme)).To(Succeed())
	return pod
}
