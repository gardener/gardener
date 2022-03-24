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

package vpa_test

import (
	"context"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/vpa"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("VPA", func() {
	var (
		ctx = context.TODO()

		namespace = "some-namespace"
		values    = Values{}

		c         client.Client
		component component.DeployWaiter

		imageExporter = "some-image:for-exporter"

		valuesExporter = ValuesExporter{
			Image: imageExporter,
		}

		vpaUpdateModeAuto = vpaautoscalingv1.UpdateModeAuto

		managedResourceName   string
		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		serviceExporter            *corev1.Service
		serviceAccountExporter     *corev1.ServiceAccount
		clusterRoleExporter        *rbacv1.ClusterRole
		clusterRoleBindingExporter *rbacv1.ClusterRoleBinding
		deploymentExporter         *appsv1.Deployment
		vpaExporter                *vpaautoscalingv1.VerticalPodAutoscaler

		serviceAccountUpdater    *corev1.ServiceAccount
		clusterRoleUpdater       *rbacv1.ClusterRole
		shootAccessSecretUpdater *corev1.Secret
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		component = New(c, namespace, values)
		managedResourceName = ""

		serviceExporter = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-exporter",
				Namespace: namespace,
				Labels: map[string]string{
					"app":                 "vpa-exporter",
					"gardener.cloud/role": "vpa",
				},
			},
			Spec: corev1.ServiceSpec{
				Type:            corev1.ServiceTypeClusterIP,
				SessionAffinity: corev1.ServiceAffinityNone,
				Selector:        map[string]string{"app": "vpa-exporter"},
				Ports: []corev1.ServicePort{{
					Name:       "metrics",
					Protocol:   corev1.ProtocolTCP,
					Port:       9570,
					TargetPort: intstr.FromInt(9570),
				}},
			},
		}
		serviceAccountExporter = &corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-exporter",
				Namespace: namespace,
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}
		clusterRoleExporter = &rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:exporter",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{"autoscaling.k8s.io"},
				Resources: []string{"verticalpodautoscalers"},
				Verbs:     []string{"get", "watch", "list"},
			}},
		}
		clusterRoleBindingExporter = &rbacv1.ClusterRoleBinding{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRoleBinding",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:exporter",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:vpa:target:exporter",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "vpa-exporter",
				Namespace: namespace,
			}},
		}
		deploymentExporter = &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-exporter",
				Namespace: namespace,
				Labels: map[string]string{
					"app":                 "vpa-exporter",
					"gardener.cloud/role": "vpa",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(1),
				RevisionHistoryLimit: pointer.Int32(2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "vpa-exporter",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                 "vpa-exporter",
							"gardener.cloud/role": "vpa",
						},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: "vpa-exporter",
						Containers: []corev1.Container{{
							Name:            "exporter",
							Image:           imageExporter,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/usr/local/bin/vpa-exporter",
								"--port=9570",
							},
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9570,
								Protocol:      corev1.ProtocolTCP,
							}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("30m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
						}},
					},
				},
			},
		}
		vpaExporter = &vpaautoscalingv1.VerticalPodAutoscaler{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "autoscaling.k8s.io/v1",
				Kind:       "VerticalPodAutoscaler",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-exporter-vpa",
				Namespace: namespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "vpa-exporter",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{UpdateMode: &vpaUpdateModeAuto},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "*",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("30m"),
								corev1.ResourceMemory: resource.MustParse("50Mi"),
							},
						},
					},
				},
			},
		}

		serviceAccountUpdater = &corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vpa-updater",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}
		clusterRoleUpdater = &rbacv1.ClusterRole{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "ClusterRole",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:vpa:target:evictioner",
				Labels: map[string]string{
					"gardener.cloud/role": "vpa",
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"apps", "extensions"},
					Resources: []string{"replicasets"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods/eviction"},
					Verbs:     []string{"create"},
				},
			},
		}
		shootAccessSecretUpdater = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-vpa-updater",
				Namespace: namespace,
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
				},
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "vpa-updater",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}
	})

	JustBeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		Context("cluster type seed", func() {
			BeforeEach(func() {
				component = New(c, namespace, Values{
					ClusterType: ClusterTypeSeed,
					Exporter:    valuesExporter,
				})
				managedResourceName = "vpa"
			})

			It("should successfully deploy all resources", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

				Expect(component.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ManagedResource",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceName,
						Namespace:       namespace,
						ResourceVersion: "1",
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class: pointer.String("seed"),
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResourceSecret.Name,
						}},
						KeepObjects: pointer.Bool(false),
					},
				}))

				clusterRoleExporter.Name = replaceTargetSubstrings(clusterRoleExporter.Name)
				clusterRoleBindingExporter.Name = replaceTargetSubstrings(clusterRoleBindingExporter.Name)
				clusterRoleBindingExporter.RoleRef.Name = replaceTargetSubstrings(clusterRoleBindingExporter.RoleRef.Name)
				clusterRoleUpdater.Name = replaceTargetSubstrings(clusterRoleUpdater.Name)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Data).To(HaveLen(8))

				By("checking vpa-exporter resources")
				Expect(string(managedResourceSecret.Data["service__"+namespace+"__vpa-exporter.yaml"])).To(Equal(serialize(serviceExporter)))
				Expect(string(managedResourceSecret.Data["serviceaccount__"+namespace+"__vpa-exporter.yaml"])).To(Equal(serialize(serviceAccountExporter)))
				Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_vpa_source_exporter.yaml"])).To(Equal(serialize(clusterRoleExporter)))
				Expect(string(managedResourceSecret.Data["clusterrolebinding____gardener.cloud_vpa_source_exporter.yaml"])).To(Equal(serialize(clusterRoleBindingExporter)))
				Expect(string(managedResourceSecret.Data["deployment__"+namespace+"__vpa-exporter.yaml"])).To(Equal(serialize(deploymentExporter)))
				Expect(string(managedResourceSecret.Data["verticalpodautoscaler__"+namespace+"__vpa-exporter-vpa.yaml"])).To(Equal(serialize(vpaExporter)))

				By("checking vpa-updater resources")
				Expect(string(managedResourceSecret.Data["serviceaccount__"+namespace+"__vpa-updater.yaml"])).To(Equal(serialize(serviceAccountUpdater)))
				Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_vpa_source_evictioner.yaml"])).To(Equal(serialize(clusterRoleUpdater)))
			})

			It("should delete the legacy resources", func() {
				legacyExporterClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:exporter"}}
				Expect(c.Create(ctx, legacyExporterClusterRole)).To(Succeed())

				legacyExporterClusterRoleBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:exporter"}}
				Expect(c.Create(ctx, legacyExporterClusterRoleBinding)).To(Succeed())

				legacyUpdaterClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:evictioner"}}
				Expect(c.Create(ctx, legacyUpdaterClusterRole)).To(Succeed())

				Expect(component.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(legacyExporterClusterRole), &rbacv1.ClusterRole{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(legacyExporterClusterRoleBinding), &rbacv1.ClusterRoleBinding{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(legacyUpdaterClusterRole), &rbacv1.ClusterRole{})).To(BeNotFoundError())
			})
		})

		Context("cluster type shoot", func() {
			BeforeEach(func() {
				component = New(c, namespace, Values{
					ClusterType: ClusterTypeShoot,
					Exporter:    valuesExporter,
				})
				managedResourceName = "shoot-core-vpa"
			})

			It("should successfully deploy all resources", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

				Expect(component.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ManagedResource",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceName,
						Namespace:       namespace,
						ResourceVersion: "1",
						Labels:          map[string]string{"origin": "gardener"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResourceSecret.Name,
						}},
						KeepObjects: pointer.Bool(false),
					},
				}))

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Data).To(HaveLen(4))

				By("checking vpa-exporter application resources")
				Expect(string(managedResourceSecret.Data["serviceaccount__"+namespace+"__vpa-exporter.yaml"])).To(Equal(serialize(serviceAccountExporter)))
				Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_vpa_target_exporter.yaml"])).To(Equal(serialize(clusterRoleExporter)))
				Expect(string(managedResourceSecret.Data["clusterrolebinding____gardener.cloud_vpa_target_exporter.yaml"])).To(Equal(serialize(clusterRoleBindingExporter)))

				By("checking vpa-exporter runtime resources")
				service := &corev1.Service{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(serviceExporter), service)).To(Succeed())
				serviceExporter.ResourceVersion = "1"
				Expect(service).To(Equal(serviceExporter))

				deployment := &appsv1.Deployment{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(deploymentExporter), deployment)).To(Succeed())
				deploymentExporter.ResourceVersion = "1"
				Expect(deployment).To(Equal(deploymentExporter))

				vpa := &vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(vpaExporter), vpa)).To(Succeed())
				vpaExporter.ResourceVersion = "1"
				Expect(vpa).To(Equal(vpaExporter))

				By("checking vpa-updater application resources")
				Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_vpa_target_evictioner.yaml"])).To(Equal(serialize(clusterRoleUpdater)))

				By("checking vpa-updater runtime resources")
				secret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(shootAccessSecretUpdater), secret)).To(Succeed())
				shootAccessSecretUpdater.ResourceVersion = "1"
				Expect(secret).To(Equal(shootAccessSecretUpdater))

			})
		})
	})

	Describe("#Destroy", func() {
		Context("cluster type seed", func() {
			BeforeEach(func() {
				component = New(c, namespace, Values{ClusterType: ClusterTypeSeed})
				managedResourceName = "vpa"
			})

			It("should successfully destroy all resources", func() {
				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(component.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
			})
		})

		Context("cluster type shoot", func() {
			BeforeEach(func() {
				component = New(c, namespace, Values{ClusterType: ClusterTypeShoot})
				managedResourceName = "shoot-core-vpa"
			})

			It("should successfully destroy all resources", func() {
				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				By("creating vpa-exporter runtime resources")
				Expect(c.Create(ctx, serviceExporter)).To(Succeed())
				Expect(c.Create(ctx, deploymentExporter)).To(Succeed())
				Expect(c.Create(ctx, vpaExporter)).To(Succeed())

				Expect(component.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))

				By("checking vpa-exporter runtime resources")
				Expect(c.Get(ctx, client.ObjectKeyFromObject(serviceExporter), &corev1.Service{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(deploymentExporter), &appsv1.Deployment{})).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(vpaExporter), &vpaautoscalingv1.VerticalPodAutoscaler{})).To(BeNotFoundError())
			})
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		AfterEach(func() {
			resetVars()
		})

		Describe("#Wait", func() {
			tests := func(managedResourceName string) {
				It("should fail because reading the ManagedResource fails", func() {
					Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
				})

				It("should fail because the ManagedResource doesn't become healthy", func() {
					fakeOps.MaxAttempts = 2

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceName,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: resourcesv1alpha1.ManagedResourceStatus{
							ObservedGeneration: 1,
							Conditions: []gardencorev1beta1.Condition{
								{
									Type:   resourcesv1alpha1.ResourcesApplied,
									Status: gardencorev1beta1.ConditionFalse,
								},
								{
									Type:   resourcesv1alpha1.ResourcesHealthy,
									Status: gardencorev1beta1.ConditionFalse,
								},
							},
						},
					}))

					Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
				})

				It("should successfully wait for the managed resource to become healthy", func() {
					fakeOps.MaxAttempts = 2

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceName,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: resourcesv1alpha1.ManagedResourceStatus{
							ObservedGeneration: 1,
							Conditions: []gardencorev1beta1.Condition{
								{
									Type:   resourcesv1alpha1.ResourcesApplied,
									Status: gardencorev1beta1.ConditionTrue,
								},
								{
									Type:   resourcesv1alpha1.ResourcesHealthy,
									Status: gardencorev1beta1.ConditionTrue,
								},
							},
						},
					}))

					Expect(component.Wait(ctx)).To(Succeed())
				})
			}

			Context("cluster type seed", func() {
				BeforeEach(func() {
					component = New(c, namespace, Values{ClusterType: ClusterTypeSeed})
				})

				tests("vpa")
			})

			Context("cluster type shoot", func() {
				BeforeEach(func() {
					component = New(c, namespace, Values{ClusterType: ClusterTypeShoot})
				})

				tests("shoot-core-vpa")
			})
		})

		Describe("#WaitCleanup", func() {
			tests := func(managedResourceName string) {
				It("should fail when the wait for the managed resource deletion times out", func() {
					fakeOps.MaxAttempts = 2

					managedResource := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:      managedResourceName,
							Namespace: namespace,
						},
					}

					Expect(c.Create(ctx, managedResource)).To(Succeed())

					Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
				})

				It("should not return an error when it's already removed", func() {
					Expect(component.WaitCleanup(ctx)).To(Succeed())
				})
			}

			Context("cluster type seed", func() {
				BeforeEach(func() {
					component = New(c, namespace, Values{ClusterType: ClusterTypeSeed})
				})

				tests("vpa")
			})

			Context("cluster type shoot", func() {
				BeforeEach(func() {
					component = New(c, namespace, Values{ClusterType: ClusterTypeShoot})
				})

				tests("shoot-core-vpa")
			})
		})
	})
})

func serialize(obj client.Object) string {
	var (
		scheme        = kubernetes.SeedScheme
		groupVersions []schema.GroupVersion
	)

	for k := range scheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}

	var (
		ser   = json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme, scheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
		codec = serializer.NewCodecFactory(scheme).CodecForVersions(ser, ser, schema.GroupVersions(groupVersions), schema.GroupVersions(groupVersions))
	)

	serializationYAML, err := runtime.Encode(codec, obj)
	Expect(err).NotTo(HaveOccurred())

	return string(serializationYAML)
}

func replaceTargetSubstrings(in string) string {
	return strings.Replace(in, "target", "source", -1)
}
