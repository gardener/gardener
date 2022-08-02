// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package podschedulername_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Pod Scheduler Name Webhook Handler", func() {
	var pod *corev1.Pod

	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "test",
					Image: "foo:latest",
				}},
			},
		}

		DeferCleanup(func() {
			By("deleting test pod")
			Expect(testClient.Delete(ctx, pod)).To(Or(Succeed(), BeNotFoundError()))
		})
	})

	Describe("ignored requests", func() {
		BeforeEach(func() {
			Expect(testClient.Create(ctx, pod)).To(Succeed())
		})

		It("Update does nothing", func() {
			expected := &corev1.Pod{}
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), expected)).To(Succeed())
			Expect(testClient.Update(ctx, pod)).To(Succeed())
			Expect(pod).To(Equal(expected))
		})
		It("Delete does nothing", func() {
			Expect(testClient.Delete(ctx, pod)).To(Succeed())
		})
	})

	It("should set .spec.schedulerName", func() {
		Expect(testClient.Create(ctx, pod)).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(pod.Spec.SchedulerName).To(Equal("gardener-shoot-controlplane-scheduler"))
	})
})
