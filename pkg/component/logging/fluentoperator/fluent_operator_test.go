// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fluentoperator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Fluent Operator", func() {
	var (
		ctx = context.TODO()

		name              = "fluent-operator"
		namespace         = "some-namespace"
		image             = "some-image:some-tag"
		priorityClassName = "some-priority-class"
		values            = Values{
			Image:             image,
			PriorityClassName: priorityClassName,
		}

		c         client.Client
		component component.DeployWaiter

		operatorManagedResourceName   = "fluent-operator"
		operatorManagedResource       *resourcesv1alpha1.ManagedResource
		operatorManagedResourceSecret *corev1.Secret

		serviceAccount     *corev1.ServiceAccount
		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
		role               *rbacv1.Role
		roleBinding        *rbacv1.RoleBinding
		deployment         *appsv1.Deployment
		vpa                *vpaautoscalingv1.VerticalPodAutoscaler
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		component = NewFluentOperator(c, namespace, values)

		serviceAccount = &corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					v1beta1constants.LabelApp:   name,
					v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
				},
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:logging:fluent-operator",
				Labels: map[string]string{
					v1beta1constants.LabelApp:   name,
					v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"fluentbit.fluent.io"},
					Resources: []string{"fluentbits", "clusterfluentbitconfigs", "clusterfilters", "clusterinputs", "clusteroutputs", "clusterparsers", "collectors", "fluentbitconfigs", "filters", "outputs", "parsers"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"secrets", "configmaps", "serviceaccounts", "services"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets", "statefulsets"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"rbac.authorization.k8s.io"},
					Resources: []string{"clusterrolebindings", "clusterroles"},
					Verbs:     []string{"get", "list", "watch", "create"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{"extensions.gardener.cloud"},
					Resources: []string{"clusters"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fluent-operator",
				Labels: map[string]string{
					v1beta1constants.LabelApp:   name,
					v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:logging:fluent-operator",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "fluent-operator",
				Namespace: namespace,
			}},
		}
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:logging:fluent-operator",
				Namespace: namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create", "get", "watch", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "get", "list", "watch", "patch"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets", "statefulsets"},
					Verbs:     []string{"create", "delete", "patch", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"secrets", "configmaps", "serviceaccounts", "services", "namespaces"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
			},
		}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:logging:fluent-operator",
				Namespace: namespace,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "fluent-operator",
					Namespace: namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "gardener.cloud:logging:fluent-operator",
			},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fluent-operator",
				Namespace: namespace,
				Labels: map[string]string{
					v1beta1constants.LabelApp:                    name,
					v1beta1constants.LabelRole:                   v1beta1constants.LabelLogging,
					v1beta1constants.GardenRole:                  v1beta1constants.GardenRoleLogging,
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
				},
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: pointer.Int32(2),
				Replicas:             pointer.Int32(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						v1beta1constants.LabelApp:   name,
						v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
						v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1beta1constants.LabelApp:                             name,
							v1beta1constants.LabelRole:                            v1beta1constants.LabelLogging,
							v1beta1constants.GardenRole:                           v1beta1constants.GardenRoleLogging,
							v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
						},
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: serviceAccount.Name,
						PriorityClassName:  priorityClassName,
						Containers: []corev1.Container{{
							Name:            name,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--leader-elect=true",
								"--disable-component-controllers=fluentd",
							},
							Env: []corev1.EnvVar{
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("20m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "env",
									MountPath: "/fluent-operator",
								},
							},
						}},
						Volumes: []corev1.Volume{
							{
								Name: "env",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
					},
				},
			},
		}

		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       deployment.Name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		operatorManagedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      OperatorManagedResourceName,
				Namespace: namespace,
			},
		}
		operatorManagedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + operatorManagedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResource), operatorManagedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, operatorManagedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResourceSecret), operatorManagedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, operatorManagedResourceSecret.Name)))

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResource), operatorManagedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            OperatorManagedResourceName,
					Namespace:       namespace,
					Labels:          map[string]string{v1beta1constants.GardenRole: "seed-system-component"},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: operatorManagedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(operatorManagedResource).To(DeepEqual(expectedMr))

			operatorManagedResourceSecret.Name = operatorManagedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResourceSecret), operatorManagedResourceSecret)).To(Succeed())
			Expect(operatorManagedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(operatorManagedResourceSecret.Data).To(HaveLen(7))
			Expect(operatorManagedResourceSecret.Immutable).To(Equal(pointer.Bool(true)))
			Expect(operatorManagedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			Expect(string(operatorManagedResourceSecret.Data["serviceaccount__"+namespace+"__fluent-operator.yaml"])).To(Equal(componenttest.Serialize(serviceAccount)))
			Expect(string(operatorManagedResourceSecret.Data["clusterrole____gardener.cloud_logging_fluent-operator.yaml"])).To(Equal(componenttest.Serialize(clusterRole)))
			Expect(string(operatorManagedResourceSecret.Data["clusterrolebinding____fluent-operator.yaml"])).To(Equal(componenttest.Serialize(clusterRoleBinding)))
			Expect(string(operatorManagedResourceSecret.Data["role__"+namespace+"__gardener.cloud_logging_fluent-operator.yaml"])).To(Equal(componenttest.Serialize(role)))
			Expect(string(operatorManagedResourceSecret.Data["rolebinding__"+namespace+"__gardener.cloud_logging_fluent-operator.yaml"])).To(Equal(componenttest.Serialize(roleBinding)))
			Expect(string(operatorManagedResourceSecret.Data["deployment__"+namespace+"__fluent-operator.yaml"])).To(Equal(componenttest.Serialize(deployment)))
			Expect(string(operatorManagedResourceSecret.Data["verticalpodautoscaler__"+namespace+"__fluent-operator.yaml"])).To(Equal(componenttest.Serialize(vpa)))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, operatorManagedResource)).To(Succeed())
			Expect(c.Create(ctx, operatorManagedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResource), operatorManagedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, operatorManagedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResourceSecret), operatorManagedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, operatorManagedResourceSecret.Name)))
		})
	})

	Context("waiting functions", func() {
		var fakeOps *retryfake.Ops

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResources fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResources don't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       operatorManagedResourceName,
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
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resources to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       operatorManagedResourceName,
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
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resources deletion times out", func() {
				fakeOps.MaxAttempts = 2

				operatorManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      operatorManagedResourceName,
						Namespace: namespace,
					},
				}
				Expect(c.Create(ctx, operatorManagedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
