// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package systemcomponentsconfig_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("SystemComponentsConfig tests", func() {
	var (
		pod   *corev1.Pod
		nodes []corev1.Node
	)

	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels: map[string]string{
					"resources.gardener.cloud/managed-by": "gardener",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "foo",
						Image: "fooImage",
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		for _, node := range nodes {
			node.ObjectMeta.GenerateName = "test-"

			if node.ObjectMeta.Labels == nil {
				node.ObjectMeta.Labels = nodeLabels()
			}

			node.ObjectMeta.Labels = utils.MergeStringMaps(node.ObjectMeta.Labels, cleanupNodeLabel())

			Expect(testClient.Create(ctx, &node)).To(Succeed())
		}

		DeferCleanup(func() {
			Expect(testClient.DeleteAllOf(ctx, &corev1.Node{}, client.MatchingLabels(cleanupNodeLabel()))).To(Succeed())
		})
	})

	Context("when no node exists", func() {
		It("should add the node selector and configured tolerations", func() {
			Expect(testClient.Create(ctx, pod)).To(Succeed())
			Expect(pod.Spec.NodeSelector).To(Equal(handlerNodeSelector))
			Expect(pod.Spec.Tolerations).To(ConsistOf(addKubernetesDefaultTolerations(handlerTolerations)))
		})
	})

	Context("when nodes exist", func() {
		BeforeEach(func() {
			nodes = []corev1.Node{{}, {}, {}}
		})

		Context("nodes without taints", func() {
			It("should add the node selector and configured tolerations", func() {
				Expect(testClient.Create(ctx, pod)).To(Succeed())
				Expect(pod.Spec.NodeSelector).To(Equal(handlerNodeSelector))
				Expect(pod.Spec.Tolerations).To(ConsistOf(addKubernetesDefaultTolerations(handlerTolerations)))
			})
		})

		Context("nodes with taints", func() {
			var additionalTaintsPool1, additionalTaintsPool2 []corev1.Taint

			BeforeEach(func() {
				additionalTaintsPool1 = []corev1.Taint{
					{
						Key:    "additionalTaintKey1",
						Effect: corev1.TaintEffectNoExecute,
						Value:  "additionalTaintValue1",
					},
					{
						Key:    "additionalTaintKey2",
						Effect: corev1.TaintEffectNoSchedule,
						Value:  "additionalTaintValue2",
					},
				}

				additionalTaintsPool2 = []corev1.Taint{
					{
						Key:    handlerTolerations[0].Key,
						Effect: handlerTolerations[0].Effect,
						Value:  handlerTolerations[0].Value,
					},
					{
						Key:    "additionalTaintKey3",
						Effect: corev1.TaintEffectNoSchedule,
					},
				}

				nodes = append(nodes,
					corev1.Node{
						Spec: corev1.NodeSpec{
							Taints: additionalTaintsPool1,
						},
					},
					corev1.Node{
						Spec: corev1.NodeSpec{
							Taints: additionalTaintsPool2,
						},
					},
					corev1.Node{
						Spec: corev1.NodeSpec{
							Taints: additionalTaintsPool1,
						},
					},
				)
			})

			It("should add the node selector and configured tolerations and tolerate taints of existing nodes", func() {
				Expect(testClient.Create(ctx, pod)).To(Succeed())
				Expect(pod.Spec.NodeSelector).To(Equal(handlerNodeSelector))

				expectedTolerations := make([]corev1.Toleration, 0, len(additionalTaintsPool1)+len(additionalTaintsPool2))
				for _, taint := range additionalTaintsPool1 {
					expectedTolerations = append(expectedTolerations, taintToToleration(taint))
				}
				for _, taint := range additionalTaintsPool2 {
					expectedTolerations = append(expectedTolerations, taintToToleration(taint))
				}

				Expect(pod.Spec.Tolerations).To(ConsistOf(addKubernetesDefaultTolerations(expectedTolerations)))
			})

			Context("pods with tolerations", func() {
				var existingTolerations []corev1.Toleration

				BeforeEach(func() {
					existingTolerations = []corev1.Toleration{
						{
							Key:               "existingKey",
							Operator:          corev1.TolerationOpEqual,
							Value:             "existingValue",
							Effect:            corev1.TaintEffectNoExecute,
							TolerationSeconds: pointer.Int64(10),
						},
						{
							Key:               "existingKey",
							Operator:          corev1.TolerationOpEqual,
							Value:             "existingValue",
							Effect:            corev1.TaintEffectNoExecute,
							TolerationSeconds: pointer.Int64(10),
						},
					}

					pod.Spec.Tolerations = existingTolerations
				})

				It("should add the node selector and configured tolerations and tolerate taints of existing nodes", func() {
					Expect(testClient.Create(ctx, pod)).To(Succeed())
					Expect(pod.Spec.NodeSelector).To(Equal(handlerNodeSelector))

					expectedTolerations := make([]corev1.Toleration, 0, len(additionalTaintsPool1)+len(additionalTaintsPool2))
					for _, taint := range additionalTaintsPool1 {
						expectedTolerations = append(expectedTolerations, taintToToleration(taint))
					}
					for _, taint := range additionalTaintsPool2 {
						expectedTolerations = append(expectedTolerations, taintToToleration(taint))
					}
					expectedTolerations = append(expectedTolerations, existingTolerations[0])

					Expect(pod.Spec.Tolerations).To(ConsistOf(addKubernetesDefaultTolerations(expectedTolerations)))
				})
			})
		})

		Context("when pod skips handling", func() {
			It("should not add node selector or tolerations", func() {
				var (
					selectorBefore    = pod.Spec.NodeSelector
					tolerationsBefore = pod.Spec.Tolerations
				)

				metav1.SetMetaDataLabel(&pod.ObjectMeta, "system-components-config.resources.gardener.cloud/skip", "true")

				Expect(testClient.Create(ctx, pod)).To(Succeed())
				Expect(pod.Spec.NodeSelector).To(Equal(selectorBefore))
				Expect(pod.Spec.Tolerations).To(ConsistOf(addKubernetesDefaultTolerations(tolerationsBefore)))
			})
		})

		Context("when no system component pod", func() {
			It("should not add node selector or tolerations", func() {
				var (
					selectorBefore    = pod.Spec.NodeSelector
					tolerationsBefore = pod.Spec.Tolerations
				)

				metav1.SetMetaDataLabel(&pod.ObjectMeta, "resources.gardener.cloud/managed-by", "some-manager")

				Expect(testClient.Create(ctx, pod)).To(Succeed())
				Expect(pod.Spec.NodeSelector).To(Equal(selectorBefore))
				Expect(pod.Spec.Tolerations).To(ConsistOf(addKubernetesDefaultTolerations(tolerationsBefore)))
			})
		})

		Context("when nodes don't match selector", func() {
			BeforeEach(func() {
				nonRelevantNode := corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"test-non-system-components-pool": testID,
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{{Key: "foo", Effect: corev1.TaintEffectNoExecute}, {Key: "bar", Effect: corev1.TaintEffectNoExecute}},
					},
				}

				nodes = append(nodes, nonRelevantNode)
			})

			It("should add the node selector and configured tolerations", func() {
				Expect(testClient.Create(ctx, pod)).To(Succeed())
				Expect(pod.Spec.NodeSelector).To(Equal(handlerNodeSelector))
				Expect(pod.Spec.Tolerations).To(ConsistOf(addKubernetesDefaultTolerations(handlerTolerations)))
			})
		})
	})
})

func taintToToleration(taint corev1.Taint) corev1.Toleration {
	operator := corev1.TolerationOpEqual
	if taint.Value == "" {
		operator = corev1.TolerationOpExists
	}

	return corev1.Toleration{
		Key:      taint.Key,
		Effect:   taint.Effect,
		Operator: operator,
		Value:    taint.Value,
	}
}

func addKubernetesDefaultTolerations(tolerations []corev1.Toleration) []corev1.Toleration {
	t := make([]corev1.Toleration, 0, len(tolerations)+2)
	t = append(t, tolerations...)

	// The following tolerations are added by the Kube-Apiserver.
	t = append(t, corev1.Toleration{
		Key:               "node.kubernetes.io/not-ready",
		Operator:          "Exists",
		Value:             "",
		Effect:            "NoExecute",
		TolerationSeconds: pointer.Int64(300),
	},
		corev1.Toleration{
			Key:               "node.kubernetes.io/unreachable",
			Operator:          "Exists",
			Value:             "",
			Effect:            "NoExecute",
			TolerationSeconds: pointer.Int64(300),
		})

	return t
}
