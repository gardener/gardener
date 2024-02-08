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

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Health controller tests", func() {
	var managedResource *resourcesv1alpha1.ManagedResource

	BeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    testNamespace.Name,
				GenerateName: "test-",
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{{
					Name: "foo",
				}},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create ManagedResource for test")
		Expect(testClient.Create(ctx, managedResource)).To(Succeed())
		log.Info("Created ManagedResource for test", "managedResource", client.ObjectKeyFromObject(managedResource))
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
	})

	Context("different class", func() {
		BeforeEach(func() {
			managedResource.Spec.Class = ptr.To("foo")
		})

		JustBeforeEach(func() {
			By("Set ManagedResource to be applied successfully")
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, gardencorev1beta1.ConditionTrue)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
		})

		It("does not touch ManagedResource if it is not responsible", func() {
			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(And(
				Not(ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy))),
				Not(ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing))),
			))
		})

		It("checks ManagedResource again if it is responsible now", func() {
			By("Update ManagedResource to default class")
			patch := client.MergeFrom(managedResource.DeepCopy())
			managedResource.Spec.Class = nil
			Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(And(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionTrue)),
				ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse)),
			))
		})
	})

	Context("ignore annotation", func() {
		BeforeEach(func() {
			metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, resourcesv1alpha1.Ignore, "true")
		})

		JustBeforeEach(func() {
			By("Set ManagedResource to be applied successfully")
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, gardencorev1beta1.ConditionTrue)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
		})

		It("does not touch ManagedResource if it is ignored", func() {
			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(And(
				Not(ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy))),
				Not(ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing))),
			))
		})

		It("checks ManagedResource again if it is no longer ignored", func() {
			By("Update ManagedResource and remove ignore annotation")
			patch := client.MergeFrom(managedResource.DeepCopy())
			delete(managedResource.Annotations, resourcesv1alpha1.Ignore)
			Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(And(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionTrue)),
				ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse)),
			))
		})
	})

	Context("resources not applied yet", func() {
		It("does not touch ManagedResource if it has not been applied yet", func() {
			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(And(
				Not(ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy))),
				Not(ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing))),
			))
		})

		It("does not touch ManagedResource if it is still being applied", func() {
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, gardencorev1beta1.ConditionProgressing)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())

			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(And(
				Not(ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy))),
				Not(ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing))),
			))
		})

		It("does not touch ManagedResource if it failed to be applied", func() {
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, gardencorev1beta1.ConditionFalse)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())

			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(And(
				Not(ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy))),
				Not(ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing))),
			))
		})
	})

	Describe("Health Reconciler", func() {
		JustBeforeEach(func() {
			By("Set ManagedResource to be applied successfully")
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, gardencorev1beta1.ConditionTrue)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
		})

		It("sets ManagedResource to healthy as it does not contain any resources", func() {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("ResourcesHealthy")),
			)
		})

		It("sets ManagedResource to unhealthy as resource is missing (registered in target scheme)", func() {
			By("Add resources to ManagedResource status")
			patch := client.MergeFrom(managedResource.DeepCopy())
			managedResource.Status.Resources = []resourcesv1alpha1.ObjectReference{{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  testNamespace.Name,
					Name:       "non-existing",
				},
			}}
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ConfigMapMissing")),
			)
		})

		It("sets ManagedResource to unhealthy as resource is missing (not registered in target scheme)", func() {
			By("Add resources to ManagedResource status")
			patch := client.MergeFrom(managedResource.DeepCopy())
			managedResource.Status.Resources = []resourcesv1alpha1.ObjectReference{{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "resources.gardener.cloud/v1alpha1",
					Kind:       "ManagedResource",
					Namespace:  testNamespace.Name,
					Name:       "non-existing",
				},
			}}
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ManagedResourceMissing")),
			)
		})

		It("sets ManagedResource to unhealthy as resource's API group does not exist", func() {
			By("Add resources to ManagedResource status")
			patch := client.MergeFrom(managedResource.DeepCopy())
			managedResource.Status.Resources = []resourcesv1alpha1.ObjectReference{{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "non-existing.k8s.io/v1",
					Kind:       "ConfigMap",
					Namespace:  testNamespace.Name,
					Name:       managedResource.Name,
				},
			}}
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ConfigMapMissing")),
			)
		})

		Context("with existing resource", func() {
			var pod *corev1.Pod

			JustBeforeEach(func() {
				By("Create Pod test resource")
				pod = generatePodTestResource(managedResource.Name)
				Expect(testClient.Create(ctx, pod)).To(Succeed())

				DeferCleanup(func() {
					By("Delete Pod test resource")
					Expect(testClient.Delete(ctx, pod)).To(Or(Succeed(), BeNotFoundError()))
				})

				By("Add resources to ManagedResource status")
				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.Status.Resources = []resourcesv1alpha1.ObjectReference{{
					ObjectReference: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  pod.Namespace,
						Name:       pod.Name,
					},
				}}
				Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
			})

			It("sets ManagedResource to unhealthy as Pod is not ready", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("PodUnhealthy")),
				)
			})

			It("sets ManagedResource to healthy even if Pod is not ready but skip-health-check annotation is present", func() {
				patch := client.MergeFrom(pod.DeepCopy())
				metav1.SetMetaDataAnnotation(&pod.ObjectMeta, resourcesv1alpha1.SkipHealthCheck, "true")
				Expect(testClient.Patch(ctx, pod, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("ResourcesHealthy")),
				)
			})

			It("sets ManagedResource to healthy as Pod is running", func() {
				By("Add resources to ManagedResource status")
				patch := client.MergeFrom(pod.DeepCopy())
				pod.Status.Phase = corev1.PodRunning
				Expect(testClient.Status().Patch(ctx, pod, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesHealthy), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("ResourcesHealthy")),
				)
			})
		})
	})

	Describe("Progressing Reconciler", func() {
		JustBeforeEach(func() {
			By("Set ManagedResource to be applied successfully")
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, gardencorev1beta1.ConditionTrue)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
		})

		It("sets Progressing to false as it does not contain any resources of interest", func() {
			By("Add resources to ManagedResource status")
			patch := client.MergeFrom(managedResource.DeepCopy())
			managedResource.Status.Resources = []resourcesv1alpha1.ObjectReference{{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  testNamespace.Name,
					Name:       "non-existing",
				},
			}}
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ResourcesRolledOut")),
			)
		})

		It("ignores missing resources", func() {
			By("Add resources to ManagedResource status")
			patch := client.MergeFrom(managedResource.DeepCopy())
			managedResource.Status.Resources = []resourcesv1alpha1.ObjectReference{{
				ObjectReference: corev1.ObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Namespace:  testNamespace.Name,
					Name:       "non-existing",
				},
			}}
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ResourcesRolledOut")),
			)
		})

		Context("with existing resources", func() {
			var (
				deployment  *appsv1.Deployment
				pod         *corev1.Pod
				statefulSet *appsv1.StatefulSet
				daemonSet   *appsv1.DaemonSet
			)

			JustBeforeEach(func() {
				By("Create test resources")
				deployment = generateDeploymentTestResource(managedResource.Name)
				deploymentStatus := deployment.Status.DeepCopy()
				Expect(testClient.Create(ctx, deployment)).To(Succeed())
				deployment.Status = *deploymentStatus
				Expect(testClient.Status().Update(ctx, deployment)).To(Succeed())

				pod = generatePodForDeployment(deployment)
				Expect(testClient.Create(ctx, pod)).To(Succeed())

				statefulSet = generateStatefulSetTestResource(managedResource.Name)
				statefulSetStatus := statefulSet.Status.DeepCopy()
				Expect(testClient.Create(ctx, statefulSet)).To(Succeed())
				statefulSet.Status = *statefulSetStatus
				Expect(testClient.Status().Update(ctx, statefulSet)).To(Succeed())

				daemonSet = generateDaemonSetTestResource(managedResource.Name)
				daemonSetStatus := daemonSet.Status.DeepCopy()
				Expect(testClient.Create(ctx, daemonSet)).To(Succeed())
				daemonSet.Status = *daemonSetStatus
				Expect(testClient.Status().Update(ctx, daemonSet)).To(Succeed())

				DeferCleanup(func() {
					By("Delete test resources")
					Expect(testClient.Delete(ctx, pod)).To(Or(Succeed(), BeNotFoundError()))
					Expect(testClient.Delete(ctx, deployment)).To(Or(Succeed(), BeNotFoundError()))
					Expect(testClient.Delete(ctx, statefulSet)).To(Or(Succeed(), BeNotFoundError()))
					Expect(testClient.Delete(ctx, daemonSet)).To(Or(Succeed(), BeNotFoundError()))
				})

				By("Add resources to ManagedResource status")
				patch := client.MergeFrom(managedResource.DeepCopy())
				managedResource.Status.Resources = []resourcesv1alpha1.ObjectReference{
					{
						ObjectReference: corev1.ObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Namespace:  deployment.Namespace,
							Name:       deployment.Name,
						},
					},
					{
						ObjectReference: corev1.ObjectReference{
							APIVersion: "apps/v1",
							Kind:       "StatefulSet",
							Namespace:  statefulSet.Namespace,
							Name:       statefulSet.Name,
						},
					},
					{
						ObjectReference: corev1.ObjectReference{
							APIVersion: "apps/v1",
							Kind:       "DaemonSet",
							Namespace:  daemonSet.Namespace,
							Name:       daemonSet.Name,
						},
					},
				}
				Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
			})

			It("sets Progressing to false as all resources have been fully rolled out", func() {
				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ResourcesRolledOut")),
				)
			})

			It("sets Progressing to true as Deployment is not fully rolled out", func() {
				patch := client.MergeFrom(deployment.DeepCopy())
				deployment.Status.Conditions = []appsv1.DeploymentCondition{{
					Type:    appsv1.DeploymentProgressing,
					Status:  corev1.ConditionFalse,
					Reason:  "ProgressDeadlineExceeded",
					Message: `ReplicaSet "nginx-946d57896" has timed out progressing.`,
				}}
				Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("DeploymentProgressing")),
				)
			})

			It("sets Progressing to true as Deployment still has non-terminated pods", func() {
				pod2 := generatePodForDeployment(deployment)
				Expect(testClient.Create(ctx, pod2)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, pod2)).To(Or(Succeed(), BeNotFoundError()))
				})

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("DeploymentProgressing")),
				)
			})

			It("sets Progressing to false even if Deployment is not fully rolled out but skip-health-check annotation is present", func() {
				patch := client.MergeFrom(deployment.DeepCopy())
				metav1.SetMetaDataAnnotation(&deployment.ObjectMeta, resourcesv1alpha1.SkipHealthCheck, "true")
				deployment.Status.Conditions = []appsv1.DeploymentCondition{{
					Type:    appsv1.DeploymentProgressing,
					Status:  corev1.ConditionFalse,
					Reason:  "ProgressDeadlineExceeded",
					Message: `ReplicaSet "nginx-946d57896" has timed out progressing.`,
				}}
				Expect(testClient.Patch(ctx, deployment, patch)).To(Succeed())
				Expect(testClient.Status().Patch(ctx, deployment, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ResourcesRolledOut")),
				)
			})

			It("sets Progressing to true as StatefulSet is not fully rolled out", func() {
				patch := client.MergeFrom(statefulSet.DeepCopy())
				statefulSet.Status.UpdatedReplicas--
				Expect(testClient.Status().Patch(ctx, statefulSet, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("StatefulSetProgressing")),
				)
			})

			It("sets Progressing to false even if StatefulSet is not fully rolled out but skip-health-check annotation is present", func() {
				patch := client.MergeFrom(statefulSet.DeepCopy())
				metav1.SetMetaDataAnnotation(&statefulSet.ObjectMeta, resourcesv1alpha1.SkipHealthCheck, "true")
				statefulSet.Status.UpdatedReplicas--
				Expect(testClient.Patch(ctx, statefulSet, patch)).To(Succeed())
				Expect(testClient.Status().Patch(ctx, statefulSet, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ResourcesRolledOut")),
				)
			})

			It("sets Progressing to true as DaemonSet is not fully rolled out", func() {
				patch := client.MergeFrom(daemonSet.DeepCopy())
				daemonSet.Status.UpdatedNumberScheduled--
				Expect(testClient.Status().Patch(ctx, daemonSet, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionTrue), WithReason("DaemonSetProgressing")),
				)
			})

			It("sets Progressing to false even if DaemonSet is not fully rolled out but skip-health-check annotation is present", func() {
				patch := client.MergeFrom(daemonSet.DeepCopy())
				metav1.SetMetaDataAnnotation(&daemonSet.ObjectMeta, resourcesv1alpha1.SkipHealthCheck, "true")
				daemonSet.Status.UpdatedNumberScheduled--
				Expect(testClient.Patch(ctx, daemonSet, patch)).To(Succeed())
				Expect(testClient.Status().Patch(ctx, daemonSet, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					ContainCondition(OfType(resourcesv1alpha1.ResourcesProgressing), WithStatus(gardencorev1beta1.ConditionFalse), WithReason("ResourcesRolledOut")),
				)
			})
		})
	})
})

func setCondition(managedResource *resourcesv1alpha1.ManagedResource, status gardencorev1beta1.ConditionStatus) {
	managedResource.Status.Conditions = v1beta1helper.MergeConditions(managedResource.Status.Conditions, gardencorev1beta1.Condition{
		Type:               resourcesv1alpha1.ResourcesApplied,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
	})
}

func generatePodTestResource(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace.Name,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:    "test",
				Image:   "alpine",
				Command: []string{"sh", "-c", "echo hello"},
			}},
			// set to non-existing node, so that no kubelet will interfere when testing against existing cluster, so that we
			// solely control the pod's status
			NodeName: "non-existing",
		},
	}
}

func generateDeploymentTestResource(name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  testNamespace.Name,
			Generation: 42,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test": "foo",
				},
			},
			Template: generatePodTemplate(),
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 42,
			Conditions: []appsv1.DeploymentCondition{{
				Type:    appsv1.DeploymentProgressing,
				Status:  corev1.ConditionTrue,
				Reason:  "NewReplicaSetAvailable",
				Message: `ReplicaSet "test-foo-abcdef" has successfully progressed.`,
			}},
		},
	}
}

func generatePodForDeployment(deployment *appsv1.Deployment) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: deployment.Name + "-pod-",
			Namespace:    deployment.Namespace,
			Labels:       deployment.Spec.Selector.MatchLabels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: "app",
			}},
		},
	}
}

func generateStatefulSetTestResource(name string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  testNamespace.Name,
			Generation: 42,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test": "foo",
				},
			},
			Template: generatePodTemplate(),
		},
		Status: appsv1.StatefulSetStatus{
			ObservedGeneration: 42,
			Replicas:           1,
			CurrentReplicas:    1,
			UpdatedReplicas:    1,
		},
	}
}

func generateDaemonSetTestResource(name string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  testNamespace.Name,
			Generation: 42,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test": "foo",
				},
			},
			Template: generatePodTemplate(),
		},
		Status: appsv1.DaemonSetStatus{
			ObservedGeneration:     42,
			DesiredNumberScheduled: 1,
			CurrentNumberScheduled: 1,
			UpdatedNumberScheduled: 1,
		},
	}
}

func generatePodTemplate() corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"test": "foo",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "test",
				Image: "ubuntu",
			}},
		},
	}
}
