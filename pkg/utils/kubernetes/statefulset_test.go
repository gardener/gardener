// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes_test

import (
	"context"
	"errors"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Statefulset.", func() {
	Describe("#GetContainerResourcesInStatefulSet", func() {
		var (
			ctrl              *gomock.Controller
			c                 *mockclient.MockClient
			testNamespace     string
			testStatefulset   string
			statefulSet       *appsv1.StatefulSet
			expectedResources *corev1.ResourceRequirements
		)

		BeforeEach(func() {
			expectedResources = &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("300Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("3000Mi"),
				},
			}

			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)
			testNamespace = "test-namespace"
			testStatefulset = "test-loki"

			statefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testStatefulset,
					Namespace: testNamespace,
				},
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{},
					},
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return container resources when statefulset contains one container", func() {
			var (
				ctx = context.TODO()
			)

			statefulSet.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:      "container-1",
					Resources: *expectedResources,
				},
			}

			c.EXPECT().Get(ctx, Key(testNamespace, testStatefulset), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).SetArg(2, *statefulSet).Return(nil)

			rr, err := GetContainerResourcesInStatefulSet(ctx, c, Key(testNamespace, testStatefulset))
			Expect(err).NotTo(HaveOccurred())
			Expect(rr).To(HaveLen(len(statefulSet.Spec.Template.Spec.Containers)))
			Expect(rr["container-1"]).To(Equal(expectedResources))
		})

		It("should return all container resources when statefulset contains two containers", func() {
			var (
				ctx = context.TODO()
			)

			statefulSet.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:      "container-1",
					Resources: *expectedResources,
				},
				{
					Name:      "container-2",
					Resources: *expectedResources,
				},
			}

			c.EXPECT().Get(ctx, Key(testNamespace, testStatefulset), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).SetArg(2, *statefulSet).Return(nil)

			rr, err := GetContainerResourcesInStatefulSet(ctx, c, Key(testNamespace, testStatefulset))
			Expect(err).NotTo(HaveOccurred())
			Expect(rr).To(HaveLen(len(statefulSet.Spec.Template.Spec.Containers)))
			Expect(rr["container-1"]).To(Equal(expectedResources))
			Expect(rr["container-2"]).To(Equal(expectedResources))
		})

		It("should return error if statefulSet is not found", func() {
			var (
				ctx = context.TODO()
			)

			c.EXPECT().Get(ctx, Key(testNamespace, testStatefulset), gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(errors.New("error"))

			_, err := GetContainerResourcesInStatefulSet(ctx, c, Key(testNamespace, testStatefulset))
			Expect(err).To(HaveOccurred())
		})
	})
})
