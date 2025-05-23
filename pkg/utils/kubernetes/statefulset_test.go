// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
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
			testStatefulset = "test-vali"

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

			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: testStatefulset}, gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).SetArg(2, *statefulSet).Return(nil)

			rr, err := GetContainerResourcesInStatefulSet(ctx, c, client.ObjectKey{Namespace: testNamespace, Name: testStatefulset})
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

			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: testStatefulset}, gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).SetArg(2, *statefulSet).Return(nil)

			rr, err := GetContainerResourcesInStatefulSet(ctx, c, client.ObjectKey{Namespace: testNamespace, Name: testStatefulset})
			Expect(err).NotTo(HaveOccurred())
			Expect(rr).To(HaveLen(len(statefulSet.Spec.Template.Spec.Containers)))
			Expect(rr["container-1"]).To(Equal(expectedResources))
			Expect(rr["container-2"]).To(Equal(expectedResources))
		})

		It("should return error if statefulSet is not found", func() {
			var (
				ctx = context.TODO()
			)

			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: testNamespace, Name: testStatefulset}, gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(errors.New("error"))

			_, err := GetContainerResourcesInStatefulSet(ctx, c, client.ObjectKey{Namespace: testNamespace, Name: testStatefulset})
			Expect(err).To(HaveOccurred())
		})
	})
})
