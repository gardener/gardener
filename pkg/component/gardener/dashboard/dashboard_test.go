// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dashboard_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/gardener/dashboard"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("GardenerDashboard", func() {
	var (
		ctx context.Context

		managedResourceNameRuntime = "gardener-dashboard-runtime"
		managedResourceNameVirtual = "gardener-dashboard-virtual"
		namespace                  = "some-namespace"

		image = "gardener-dashboard-image:latest"

		fakeClient        client.Client
		fakeSecretManager secretsmanager.Interface
		deployer          component.DeployWaiter
		values            Values

		fakeOps   *retryfake.Ops
		consistOf func(...client.Object) types.GomegaMatcher

		managedResourceRuntime       *resourcesv1alpha1.ManagedResource
		managedResourceVirtual       *resourcesv1alpha1.ManagedResource
		managedResourceSecretRuntime *corev1.Secret
		managedResourceSecretVirtual *corev1.Secret

		virtualGardenAccessSecret *corev1.Secret
		deployment                *appsv1.Deployment
		service                   *corev1.Service
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)
		values = Values{
			LogLevel:       "info",
			RuntimeVersion: semver.MustParse("1.26.4"),
			Image:          image,
		}

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

		managedResourceRuntime = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameRuntime,
				Namespace: namespace,
			},
		}
		managedResourceVirtual = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameVirtual,
				Namespace: namespace,
			},
		}
		managedResourceSecretRuntime = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceRuntime.Name,
				Namespace: namespace,
			},
		}
		managedResourceSecretVirtual = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceVirtual.Name,
				Namespace: namespace,
			},
		}

		virtualGardenAccessSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-gardener-dashboard",
				Namespace: namespace,
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "gardener-dashboard",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-dashboard",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
					"high-availability-config.resources.gardener.cloud/type": "server",
				},
				Annotations: map[string]string{
					"reference.resources.gardener.cloud/secret-867d23cd": "generic-token-kubeconfig",
					"reference.resources.gardener.cloud/secret-da1c7d68": "shoot-access-gardener-dashboard",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":  "gardener",
						"role": "dashboard",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                              "gardener",
							"role":                             "dashboard",
							"networking.gardener.cloud/to-dns": "allowed",
							"networking.gardener.cloud/to-public-networks":                                 "allowed",
							"networking.resources.gardener.cloud/to-virtual-garden-kube-apiserver-tcp-443": "allowed",
						},
						Annotations: map[string]string{
							"reference.resources.gardener.cloud/secret-867d23cd": "generic-token-kubeconfig",
							"reference.resources.gardener.cloud/secret-da1c7d68": "shoot-access-gardener-dashboard",
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:            "gardener-garden-system-200",
						AutomountServiceAccountToken: ptr.To(false),
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: ptr.To(true),
							RunAsUser:    ptr.To[int64](65532),
							RunAsGroup:   ptr.To[int64](65532),
							FSGroup:      ptr.To[int64](65532),
						},
						Containers: []corev1.Container{
							{
								Name:            "gardener-dashboard",
								Image:           image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--optimize-for-size",
									"server.js",
								},
								Env: []corev1.EnvVar{
									{
										Name:  "KUBECONFIG",
										Value: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
									},
									{
										Name:  "METRICS_PORT",
										Value: "9050",
									},
									{
										Name: "POD_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.name",
											},
										},
									},
									{
										Name: "POD_NAMESPACE",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.namespace",
											},
										},
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
								Ports: []corev1.ContainerPort{
									{
										Name:          "http",
										ContainerPort: 8080,
										Protocol:      corev1.ProtocolTCP,
									},
									{
										Name:          "metrics",
										ContainerPort: 9050,
										Protocol:      corev1.ProtocolTCP,
									},
								},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										TCPSocket: &corev1.TCPSocketAction{
											Port: intstr.FromString("http"),
										},
									},
									InitialDelaySeconds: 15,
									TimeoutSeconds:      5,
									FailureThreshold:    6,
									SuccessThreshold:    1,
									PeriodSeconds:       20,
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path:   "/healthz",
											Port:   intstr.FromString("http"),
											Scheme: "HTTP",
										},
									},
									InitialDelaySeconds: 5,
									TimeoutSeconds:      5,
									FailureThreshold:    6,
									SuccessThreshold:    1,
									PeriodSeconds:       10,
								},
							},
						},
					},
				},
			},
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-dashboard",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "gardener",
					"role": "dashboard",
				},
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: GetLabels(),
				Ports: []corev1.ServicePort{{
					Name:       "http",
					Port:       8080,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(8080),
				}},
				SessionAffinity: corev1.ServiceAffinityClientIP,
			},
		}

		utilruntime.Must(gardener.InjectGenericKubeconfig(deployment, "generic-token-kubeconfig", "shoot-access-gardener-dashboard"))
	})

	JustBeforeEach(func() {
		deployer = New(fakeClient, namespace, fakeSecretManager, values)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	Describe("#Deploy", func() {
		Context("resources generation", func() {
			var expectedRuntimeObject []client.Object

			BeforeEach(func() {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())
			})

			JustBeforeEach(func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
				expectedRuntimeMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceRuntime.Name,
						Namespace:       managedResourceRuntime.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels: map[string]string{
							"gardener.cloud/role":                "seed-system-component",
							"care.gardener.cloud/condition-type": "VirtualComponentsHealthy",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class:       ptr.To("seed"),
						SecretRefs:  []corev1.LocalObjectReference{{Name: managedResourceRuntime.Spec.SecretRefs[0].Name}},
						KeepObjects: ptr.To(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedRuntimeMr))
				Expect(managedResourceRuntime).To(Equal(expectedRuntimeMr))

				managedResourceSecretRuntime.Name = managedResourceRuntime.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
				expectedVirtualMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceVirtual.Name,
						Namespace:       managedResourceVirtual.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels: map[string]string{
							"origin":                             "gardener",
							"care.gardener.cloud/condition-type": "VirtualComponentsHealthy",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceVirtual.Spec.SecretRefs[0].Name}},
						KeepObjects:  ptr.To(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedVirtualMr))
				Expect(managedResourceVirtual).To(Equal(expectedVirtualMr))
				expectedRuntimeObject = []client.Object{
					deployment,
					service,
				}

				managedResourceSecretVirtual.Name = expectedVirtualMr.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())
				Expect(managedResourceSecretRuntime.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretRuntime.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecretRuntime.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				Expect(managedResourceVirtual).To(consistOf())
				Expect(managedResourceSecretVirtual.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretVirtual.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecretVirtual.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			})

			It("should successfully deploy all resources", func() {
				Expect(managedResourceRuntime).To(consistOf(expectedRuntimeObject...))
			})

			Context("secrets", func() {
				It("should successfully deploy the access secret for the virtual garden", func() {
					actualAccessSecret := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(virtualGardenAccessSecret), actualAccessSecret)).To(Succeed())
					virtualGardenAccessSecret.ResourceVersion = "1"
					Expect(actualAccessSecret).To(Equal(virtualGardenAccessSecret))
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretVirtual)).To(Succeed())
			Expect(fakeClient.Create(ctx, virtualGardenAccessSecret)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(virtualGardenAccessSecret), virtualGardenAccessSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the runtime and virtual ManagedResources are unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because the runtime ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because the virtual ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should succeed because the runtime and virtual ManagedResource are healthy and progressing", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})

			It("should succeed because the both ManagedResource are healthy and progressed", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the runtime managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should fail when the wait for the virtual managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when they are already removed", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

var (
	healthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
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
	}
	unhealthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
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
	}
)
