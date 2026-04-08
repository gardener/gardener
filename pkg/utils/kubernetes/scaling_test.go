// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("scale", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		key        = client.ObjectKey{Name: "foo", Namespace: "bar"}
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
	})

	Context("ScaleStatefulSet", func() {
		It("sets scale to 2", func() {
			Expect(fakeClient.Create(ctx, &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
			})).To(Succeed())

			Expect(ScaleStatefulSet(ctx, fakeClient, key, 2)).To(Succeed())

			ss := &appsv1.StatefulSet{}
			Expect(fakeClient.Get(ctx, key, ss)).To(Succeed())
			Expect(ss.Spec.Replicas).To(Equal(ptr.To[int32](2)))
		})
	})

	Context("ScaleDeployment", func() {
		It("sets scale to 2", func() {
			Expect(fakeClient.Create(ctx, &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
			})).To(Succeed())

			Expect(ScaleDeployment(ctx, fakeClient, key, 2)).To(Succeed())

			dep := &appsv1.Deployment{}
			Expect(fakeClient.Get(ctx, key, dep)).To(Succeed())
			Expect(dep.Spec.Replicas).To(Equal(ptr.To[int32](2)))
		})
	})

	Describe("#WaitUntilDeploymentScaledToDesiredReplicas", func() {
		It("should wait until deployment was scaled", func() {
			Expect(fakeClient.Create(ctx, &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Generation: 2},
				Spec:       appsv1.DeploymentSpec{Replicas: ptr.To[int32](2)},
				Status: appsv1.DeploymentStatus{
					ObservedGeneration: 2,
					Replicas:           2,
					AvailableReplicas:  2,
				},
			})).To(Succeed())

			Expect(WaitUntilDeploymentScaledToDesiredReplicas(ctx, fakeClient, key, 2)).To(Succeed())
		})
	})

	Describe("#WaitUntilStatefulSetScaledToDesiredReplicas", func() {
		It("should wait until statefulset was scaled", func() {
			Expect(fakeClient.Create(ctx, &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Generation: 2},
				Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To[int32](2)},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 2,
					Replicas:           2,
					AvailableReplicas:  2,
				},
			})).To(Succeed())

			Expect(WaitUntilStatefulSetScaledToDesiredReplicas(ctx, fakeClient, key, 2)).To(Succeed())
		})
	})

	Describe("#ScaleStatefulSetAndWaitUntilScaled", func() {
		It("should scale and wait until statefulset was scaled", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithInterceptorFuncs(interceptor.Funcs{
				SubResourcePatch: func(_ context.Context, _ client.Client, subResourceName string, obj client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
					sts, ok := obj.(*appsv1.StatefulSet)
					Expect(ok).To(BeTrue())
					Expect(sts.Name).To(Equal(key.Name))
					Expect(sts.Namespace).To(Equal(key.Namespace))
					Expect(subResourceName).To(Equal("scale"))

					return nil
				},
			}).Build()

			Expect(fakeClient.Create(ctx, &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Generation: 2},
				Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To[int32](2)},
				Status: appsv1.StatefulSetStatus{
					ObservedGeneration: 2,
					Replicas:           2,
					AvailableReplicas:  2,
				},
			})).To(Succeed())

			Expect(ScaleStatefulSetAndWaitUntilScaled(ctx, fakeClient, key, 2)).To(Succeed())
		})
	})
})
