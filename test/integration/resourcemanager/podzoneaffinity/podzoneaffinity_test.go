// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package podzoneaffinity_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("PodSchedulerName tests", func() {
	var (
		pod *corev1.Pod
	)

	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "foo-container",
						Image: "foo",
					},
				},
			},
		}
		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, pod)).To(Succeed())
		})
	})

	Context("when namespace has zone enforcement label", func() {
		BeforeEach(func() {
			testNamespace.Labels = map[string]string{
				"control-plane.shoot.gardener.cloud/enforce-zone": "",
			}
			Expect(testClient.Update(ctx, testNamespace)).To(Succeed())
			DeferCleanup(func() {
				testNamespace.Labels = nil
				Expect(testClient.Update(ctx, testNamespace)).To(Succeed())
			})
		})

		It("should add podAffinity", func() {
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			Consistently(func() ([]corev1.PodAffinityTerm, error) {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
				if pod.Spec.Affinity == nil {
					return nil, fmt.Errorf("affininty is nil")
				}
				if pod.Spec.Affinity.PodAffinity == nil {
					return nil, fmt.Errorf("podAffinity is nil")
				}
				return pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution, nil
			}).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"TopologyKey": Equal(corev1.LabelTopologyZone),
				"LabelSelector": PointTo(MatchFields(IgnoreExtras, Fields{
					"MatchLabels":      BeNil(),
					"MatchExpressions": BeNil(),
				})),
			})))
		})
	})

	Context("when namespace has zone enforcement label with value", func() {
		BeforeEach(func() {
			testNamespace.Labels = map[string]string{
				"control-plane.shoot.gardener.cloud/enforce-zone": "zone-a",
			}
			Expect(testClient.Update(ctx, testNamespace)).To(Succeed())
			DeferCleanup(func() {
				testNamespace.Labels = nil
				Expect(testClient.Update(ctx, testNamespace)).To(Succeed())
			})
		})

		It("should add nodeAffinity", func() {
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			Consistently(func() ([]corev1.NodeSelectorRequirement, error) {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
				if pod.Spec.Affinity == nil {
					return nil, fmt.Errorf("affininty is nil")
				}
				if pod.Spec.Affinity.NodeAffinity == nil {
					return nil, fmt.Errorf("nodeAffinity is nil")
				}
				if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
					return nil, fmt.Errorf("requiredDuringSchedulingIgnoredDuringExecution is nil")
				}
				Expect(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).To(HaveLen(1))
				return pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions, nil
			}).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Key":      Equal(corev1.LabelTopologyZone),
				"Operator": Equal(corev1.NodeSelectorOpIn),
				"Values":   ConsistOf(Equal("zone-a")),
			})))
		})
	})

	Context("when namespace hasn't zone enforcement label", func() {
		It("should not add podAffinity", func() {
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			Consistently(func() *corev1.Affinity {
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
				return pod.Spec.Affinity
			}).Should(BeNil())
		})
	})
})
