// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package node_test

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/node"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
}

var _ = Describe("Reconciler", func() {
	var (
		log       logr.Logger
		logBuffer *gbytes.Buffer
		recorder  *record.FakeRecorder

		node *corev1.Node
	)

	BeforeEach(func() {
		logBuffer = gbytes.NewBuffer()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(logBuffer))
		recorder = record.NewFakeRecorder(1)

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"kubernetes.io/os": "linux",
				},
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

	Describe("RemoveTaint", func() {
		var (
			ctx  context.Context
			node *corev1.Node

			c runtimeclient.Client
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

			Expect(c.Get(ctx, runtimeclient.ObjectKeyFromObject(node), node)).To(Succeed())
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

			Expect(c.Get(ctx, runtimeclient.ObjectKeyFromObject(node), node)).To(Succeed())
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
