// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpainplaceupdates_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("VPAInPlaceUpdates tests", func() {
	var (
		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	BeforeEach(func() {
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					Name:       "test-deployment",
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
			},
		}
	})

	Context("when skip label is specified", func() {
		BeforeEach(func() {
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeRecreate),
			}
		})

		It("should not mutate vertical pod autoscaler", func() {
			metav1.SetMetaDataLabel(&vpa.ObjectMeta, "vpa-in-place-updates.resources.gardener.cloud/skip", "")

			Expect(testClient.Create(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeRecreate)))
		})

	})

	Context("when update mode is Auto", func() {
		BeforeEach(func() {
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
			}
		})

		It("should mutate vertical pod autoscaler", func() {
			Expect(testClient.Create(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeInPlaceOrRecreate)))
		})
	})

	Context("when update mode is Recreate", func() {
		BeforeEach(func() {
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeRecreate),
			}
		})

		It("should mutate vertical pod autoscaler", func() {
			Expect(testClient.Create(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeInPlaceOrRecreate)))
		})
	})

	Context("when update mode is Initial", func() {
		BeforeEach(func() {
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeInitial),
			}
		})

		It("should not mutate vertical pod autoscaler", func() {
			Expect(testClient.Create(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeInitial)))
		})
	})

	Context("when update mode is Off", func() {
		BeforeEach(func() {
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeOff),
			}
		})

		It("should not mutate vertical pod autoscaler", func() {
			Expect(testClient.Create(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeOff)))
		})
	})

	Context("when update mode is not specified", func() {
		It("should mutate vertical pod autoscaler", func() {
			Expect(testClient.Create(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeInPlaceOrRecreate)))
		})
	})
})
