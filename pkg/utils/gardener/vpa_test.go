// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("VPAForGardenerComponent", func() {
	var (
		ctx        context.Context
		fakeClient client.Client

		name      = "foo"
		namespace = "bar"

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		ctx = context.Background()

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: name + "-vpa", Namespace: namespace}}
		Expect(fakeClient.Create(ctx, vpa)).To(Succeed())
	})

	Describe("#ReconcileVPAForGardenerComponent", func() {
		It("should reconcile the VPA successfully", func() {
			Expect(ReconcileVPAForGardenerComponent(ctx, fakeClient, name, namespace)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
			Expect(vpa).To(Equal(&vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name + "-vpa",
					Namespace:       namespace,
					ResourceVersion: "2",
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       name,
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
							ContainerName: name,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
						}},
					},
				},
			}))
		})
	})

	Describe("#DeleteVPAForGardenerComponent", func() {
		It("should delete the VPA successfully", func() {
			Expect(DeleteVPAForGardenerComponent(ctx, fakeClient, name, namespace)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(BeNotFoundError())
		})
	})
})
