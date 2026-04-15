// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Statefulset.", func() {
	Describe("#GetContainerResourcesInStatefulSet", func() {
		var (
			ctx               = context.TODO()
			fakeClient        client.Client
			testNamespace     string
			testStatefulset   string
			statefulSet       *appsv1.StatefulSet
			expectedResources *corev1.ResourceRequirements
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

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

		It("should return container resources when statefulset contains one container", func() {
			statefulSet.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:      "container-1",
					Resources: *expectedResources,
				},
			}
			Expect(fakeClient.Create(ctx, statefulSet)).To(Succeed())

			rr, err := GetContainerResourcesInStatefulSet(ctx, fakeClient, client.ObjectKeyFromObject(statefulSet))
			Expect(err).NotTo(HaveOccurred())
			Expect(rr).To(HaveLen(1))
			Expect(rr["container-1"]).To(Equal(expectedResources))
		})

		It("should return all container resources when statefulset contains two containers", func() {
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
			Expect(fakeClient.Create(ctx, statefulSet)).To(Succeed())

			rr, err := GetContainerResourcesInStatefulSet(ctx, fakeClient, client.ObjectKeyFromObject(statefulSet))
			Expect(err).NotTo(HaveOccurred())
			Expect(rr).To(HaveLen(2))
			Expect(rr["container-1"]).To(Equal(expectedResources))
			Expect(rr["container-2"]).To(Equal(expectedResources))
		})

		It("should return nil if statefulSet is not found", func() {
			rr, err := GetContainerResourcesInStatefulSet(ctx, fakeClient, client.ObjectKeyFromObject(statefulSet))
			Expect(err).NotTo(HaveOccurred())
			Expect(rr).To(BeNil())
		})
	})
})
