// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vpaevictionrequirements_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var _ = Describe("VPA EvictionRequirements controller tests", func() {
	var (
		vpa                            *vpaautoscalingv1.VerticalPodAutoscaler
		targetDeployment               *appsv1.Deployment
		maintenanceWindowNow           *gardencorev1beta1.MaintenanceTimeWindow
		maintenanceWindowAlreadyPassed *gardencorev1beta1.MaintenanceTimeWindow
		upscaleOnlyRequirement         = []*vpaautoscalingv1.EvictionRequirement{{
			Resources:         []corev1.ResourceName{corev1.ResourceMemory, corev1.ResourceCPU},
			ChangeRequirement: vpaautoscalingv1.TargetHigherThanRequests,
		}}
	)

	BeforeEach(func() {
		fakeClock.SetTime(time.Now())

		maintenanceWindowNow = &gardencorev1beta1.MaintenanceTimeWindow{
			Begin: time.Now().Format("150405-0700"),
			End:   time.Now().Add(1 * time.Hour).Format("150405-0700"),
		}
		maintenanceWindowAlreadyPassed = &gardencorev1beta1.MaintenanceTimeWindow{
			Begin: time.Now().Add(-3 * time.Hour).Format("150405-0700"),
			End:   time.Now().Add(-2 * time.Hour).Format("150405-0700"),
		}

		targetDeployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: testNamespace.Name,
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "test-container", Image: "my-nonexisting-image"},
						},
					},
				},
			},
		}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vpa",
				Namespace: testNamespace.Name,
				Labels: map[string]string{
					testID: testRunID,
					v1beta1constants.LabelVPAEvictionRequirementsController: v1beta1constants.EvictionRequirementManagedByController,
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef:    &autoscalingv1.CrossVersionObjectReference{Name: targetDeployment.Name, APIVersion: "apps/v1", Kind: "Deployment"},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{EvictionRequirements: []*vpaautoscalingv1.EvictionRequirement{}},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create deployment")
		Expect(testClient.Create(ctx, targetDeployment)).To(Succeed())

		DeferCleanup(func() {
			By("Delete deployment")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, targetDeployment))).To(Succeed())
		})

		By("Create VPA")
		Expect(testClient.Create(ctx, vpa)).To(Succeed())

		DeferCleanup(func() {
			By("Delete VPA")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, vpa))).To(Succeed())
		})
	})

	Context("VPA is labeled with downscale-in-maintenance-window-only", func() {
		BeforeEach(func() {
			metav1.SetMetaDataLabel(&vpa.ObjectMeta, v1beta1constants.LabelVPAEvictionRequirementDownscaleRestriction, v1beta1constants.EvictionRequirementInMaintenanceWindowOnly)
		})

		When("the Shoot is outside its maintenance window", func() {
			BeforeEach(func() {
				metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationShootMaintenanceWindow, maintenanceWindowAlreadyPassed.Begin+","+maintenanceWindowAlreadyPassed.End)
			})

			It("should add an EvictionRequirement denying scaling down and requeue for the beginning of the next window", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
					g.Expect(vpa.Spec).To(MatchFields(IgnoreExtras, Fields{
						"UpdatePolicy": PointTo(MatchFields(IgnoreExtras, Fields{
							"EvictionRequirements": ConsistOf(upscaleOnlyRequirement),
						})),
					}))
				}).Should(Succeed())
			})
		})

		When("the Shoot is inside its maintenance window", func() {
			BeforeEach(func() {
				metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationShootMaintenanceWindow, maintenanceWindowNow.Begin+","+maintenanceWindowNow.End)
				vpa.Spec.UpdatePolicy.EvictionRequirements = upscaleOnlyRequirement
			})

			It("should remove the EvictionRequirement and requeue for the end of the maintenance window", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
					g.Expect(vpa.Spec).To(MatchFields(IgnoreExtras, Fields{
						"UpdatePolicy": PointTo(MatchFields(IgnoreExtras, Fields{
							"EvictionRequirements": BeEmpty(),
						})),
					}))
				}).Should(Succeed())
			})
		})

		When("the Shoot maintenance window is updated", func() {
			BeforeEach(func() {
				metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationShootMaintenanceWindow, maintenanceWindowNow.Begin+","+maintenanceWindowNow.End)
				vpa.Spec.UpdatePolicy.EvictionRequirements = upscaleOnlyRequirement
			})

			It("reconciles the VPA again", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
					g.Expect(vpa.Spec).To(MatchFields(IgnoreExtras, Fields{
						"UpdatePolicy": PointTo(MatchFields(IgnoreExtras, Fields{
							"EvictionRequirements": BeEmpty(),
						})),
					}))
				}).Should(Succeed())

				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
				metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationShootMaintenanceWindow, maintenanceWindowAlreadyPassed.Begin+","+maintenanceWindowAlreadyPassed.End)
				Expect(testClient.Update(ctx, vpa)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
					g.Expect(vpa.Spec).To(MatchFields(IgnoreExtras, Fields{
						"UpdatePolicy": PointTo(MatchFields(IgnoreExtras, Fields{
							"EvictionRequirements": ConsistOf(upscaleOnlyRequirement),
						})),
					}))
				}).Should(Succeed())
			})
		})
	})

	Context("VPA is labeled with downscale-never", func() {
		BeforeEach(func() {
			metav1.SetMetaDataLabel(&vpa.ObjectMeta, v1beta1constants.LabelVPAEvictionRequirementDownscaleRestriction, v1beta1constants.EvictionRequirementNever)
		})

		It("should add an EvictionRequirement and not requeue, regardless of a Shoot's maintenance window", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
				g.Expect(vpa.Spec).To(MatchFields(IgnoreExtras, Fields{
					"UpdatePolicy": PointTo(MatchFields(IgnoreExtras, Fields{
						"EvictionRequirements": ConsistOf(upscaleOnlyRequirement),
					})),
				}))
			}).Should(Succeed())
		})
		When("VPA has an annotation indicating that the Shoot's maintenance window is now", func() {
			It("doesn't remove the EvictionRequirement", func() {
				Eventually(func(g Gomega) {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
					g.Expect(vpa.Spec).To(MatchFields(IgnoreExtras, Fields{
						"UpdatePolicy": PointTo(MatchFields(IgnoreExtras, Fields{
							"EvictionRequirements": ConsistOf(upscaleOnlyRequirement),
						})),
					}))
				}).Should(Succeed())
			})
		})
	})

})
