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

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
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
		By("creating ManagedResource for test")
		Expect(testClient.Create(ctx, managedResource)).To(Succeed())
		log.Info("Created ManagedResource for test", "managedResource", client.ObjectKeyFromObject(managedResource))
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, managedResource)).To(Or(Succeed(), BeNotFoundError()))
	})

	Context("different class", func() {
		BeforeEach(func() {
			managedResource.Spec.Class = pointer.String("foo")
		})

		JustBeforeEach(func() {
			By("set ManagedResource to be applied successfully")
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, resourcesv1alpha1.ResourcesApplied, gardencorev1beta1.ConditionTrue)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
		})

		It("does not touch ManagedResource if it is not responsible", func() {
			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).ShouldNot(
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy)),
				containCondition(ofType(resourcesv1alpha1.ResourcesProgressing)),
			)
		})

		It("checks ManagedResource again if it is responsible now", func() {
			By("update ManagedResource to default class")
			patch := client.MergeFrom(managedResource.DeepCopy())
			managedResource.Spec.Class = nil
			Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionTrue)),
				containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionFalse)),
			)
		})
	})

	Context("ignore annotation", func() {
		BeforeEach(func() {
			metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, resourcesv1alpha1.Ignore, "true")
		})

		JustBeforeEach(func() {
			By("set ManagedResource to be applied successfully")
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, resourcesv1alpha1.ResourcesApplied, gardencorev1beta1.ConditionTrue)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
		})

		It("does not touch ManagedResource if it is ignored", func() {
			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).ShouldNot(
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy)),
				containCondition(ofType(resourcesv1alpha1.ResourcesProgressing)),
			)
		})

		It("checks ManagedResource again if it is no longer ignored", func() {
			By("update ManagedResource and remove ignore annotation")
			patch := client.MergeFrom(managedResource.DeepCopy())
			delete(managedResource.Annotations, resourcesv1alpha1.Ignore)
			Expect(testClient.Patch(ctx, managedResource, patch)).To(Succeed())

			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionTrue)),
				containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionFalse)),
			)
		})
	})

	Context("resources not applied yet", func() {
		It("does not touch ManagedResource if it has not been applied yet", func() {
			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).ShouldNot(
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy)),
				containCondition(ofType(resourcesv1alpha1.ResourcesProgressing)),
			)
		})

		It("does not touch ManagedResource if it is still being applied", func() {
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, resourcesv1alpha1.ResourcesApplied, gardencorev1beta1.ConditionProgressing)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())

			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).ShouldNot(
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy)),
				containCondition(ofType(resourcesv1alpha1.ResourcesProgressing)),
			)
		})

		It("does not touch ManagedResource if it failed to be applied", func() {
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, resourcesv1alpha1.ResourcesApplied, gardencorev1beta1.ConditionFalse)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())

			Consistently(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).ShouldNot(
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy)),
				containCondition(ofType(resourcesv1alpha1.ResourcesProgressing)),
			)
		})
	})

	Describe("Health Reconciler", func() {
		JustBeforeEach(func() {
			By("set ManagedResource to be applied successfully")
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, resourcesv1alpha1.ResourcesApplied, gardencorev1beta1.ConditionTrue)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
		})

		It("sets ManagedResource to healthy as it does not contain any resources", func() {
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				return managedResource.Status.Conditions
			}).Should(
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionTrue), withReason("ResourcesHealthy")),
			)
		})

		It("sets ManagedResource to unhealthy as resource is missing (registered in target scheme)", func() {
			By("add resources to ManagedResource status")
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
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionFalse), withReason("ConfigMapMissing")),
			)
		})

		It("sets ManagedResource to unhealthy as resource is missing (not registered in target scheme)", func() {
			By("add resources to ManagedResource status")
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
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionFalse), withReason("ManagedResourceMissing")),
			)
		})

		It("sets ManagedResource to unhealthy as resource's API group does not exist", func() {
			By("add resources to ManagedResource status")
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
				containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionFalse), withReason("ConfigMapMissing")),
			)
		})

		Context("with existing resource", func() {
			var pod *corev1.Pod

			JustBeforeEach(func() {
				By("create Pod test resource")
				pod = generatePodTestResource(managedResource.Name)
				Expect(testClient.Create(ctx, pod)).To(Succeed())

				DeferCleanup(func() {
					By("delete Pod test resource")
					Expect(testClient.Delete(ctx, pod)).To(Or(Succeed(), BeNotFoundError()))
				})

				By("add resources to ManagedResource status")
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
					containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionFalse), withReason("PodUnhealthy")),
				)
			})

			It("sets ManagedResource to healthy as Pod is running", func() {
				By("add resources to ManagedResource status")
				patch := client.MergeFrom(pod.DeepCopy())
				pod.Status.Phase = corev1.PodRunning
				Expect(testClient.Status().Patch(ctx, pod, patch)).To(Succeed())

				Eventually(func(g Gomega) []gardencorev1beta1.Condition {
					g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					return managedResource.Status.Conditions
				}).Should(
					containCondition(ofType(resourcesv1alpha1.ResourcesHealthy), withStatus(gardencorev1beta1.ConditionTrue), withReason("ResourcesHealthy")),
				)
			})
		})
	})

	Describe("Progressing Reconciler", func() {
		JustBeforeEach(func() {
			By("set ManagedResource to be applied successfully")
			patch := client.MergeFrom(managedResource.DeepCopy())
			setCondition(managedResource, resourcesv1alpha1.ResourcesApplied, gardencorev1beta1.ConditionTrue)
			Expect(testClient.Status().Patch(ctx, managedResource, patch)).To(Succeed())
		})

		It("sets Progressing to false as it does not contain any resources of interest", func() {
			By("add resources to ManagedResource status")
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
				containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionFalse), withReason("ResourcesRolledOut")),
			)
		})

		It("ignores missing resources", func() {
			By("add resources to ManagedResource status")
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
				containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionFalse), withReason("ResourcesRolledOut")),
			)
		})

		Context("with existing resources", func() {
			var (
				deployment  *appsv1.Deployment
				statefulSet *appsv1.StatefulSet
				daemonSet   *appsv1.DaemonSet
			)

			JustBeforeEach(func() {
				By("create test resources")
				deployment = generateDeploymentTestResource(managedResource.Name)
				deploymentStatus := deployment.Status.DeepCopy()
				Expect(testClient.Create(ctx, deployment)).To(Succeed())
				deployment.Status = *deploymentStatus
				Expect(testClient.Status().Update(ctx, deployment)).To(Succeed())
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
					By("delete test resources")
					Expect(testClient.Delete(ctx, deployment)).To(Or(Succeed(), BeNotFoundError()))
					Expect(testClient.Delete(ctx, statefulSet)).To(Or(Succeed(), BeNotFoundError()))
					Expect(testClient.Delete(ctx, daemonSet)).To(Or(Succeed(), BeNotFoundError()))
				})

				By("add resources to ManagedResource status")
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
					containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionFalse), withReason("ResourcesRolledOut")),
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
					containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionTrue), withReason("DeploymentProgressing")),
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
					containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionTrue), withReason("StatefulSetProgressing")),
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
					containCondition(ofType(resourcesv1alpha1.ResourcesProgressing), withStatus(gardencorev1beta1.ConditionTrue), withReason("DaemonSetProgressing")),
				)
			})
		})
	})
})

func setCondition(managedResource *resourcesv1alpha1.ManagedResource, conditionType gardencorev1beta1.ConditionType, status gardencorev1beta1.ConditionStatus) {
	managedResource.Status.Conditions = v1beta1helper.MergeConditions(managedResource.Status.Conditions, gardencorev1beta1.Condition{
		Type:               conditionType,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
	})
}

func containCondition(matchers ...gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return ContainElement(And(matchers...))
}

func ofType(conditionType gardencorev1beta1.ConditionType) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Type": Equal(conditionType),
	})
}

func withStatus(status gardencorev1beta1.ConditionStatus) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Status": Equal(status),
	})
}

func withReason(reason string) gomegatypes.GomegaMatcher {
	return gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
		"Reason": Equal(reason),
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
			Replicas: pointer.Int32(1),
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

func generateStatefulSetTestResource(name string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  testNamespace.Name,
			Generation: 42,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32(1),
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
