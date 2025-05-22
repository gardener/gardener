// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package opentelemetryoperator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	. "github.com/gardener/gardener/pkg/component/observability/opentelemetry/opentelemetryoperator"
)

var _ = Describe("OpenTelemetry Operator", func() {
	var (
		ctx = context.Background()

		namespace         = "some-namespace"
		image             = "some-image:some-tag"
		priorityClassName = "some-priority-class"
		values            = Values{
			Image:             image,
			PriorityClassName: priorityClassName,
		}

		c         client.Client
		component component.DeployWaiter
		consistOf func(...client.Object) types.GomegaMatcher

		operatorManagedResourceName   = "opentelemetry-operator"
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
		component = NewOpenTelemetryOperator(c, namespace, values)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		serviceAccount = ServiceAccount(namespace)
		clusterRole = ClusterRole()
		clusterRoleBinding = ClusterRoleBinding(clusterRole.Name, serviceAccount.Name, serviceAccount.Namespace)
		role = Role(namespace)
		roleBinding = RoleBinding(role.Name, namespace)
		deployment = Deployment(namespace, image, priorityClassName)
		vpa = Vpa(namespace)
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
			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResource), operatorManagedResource)).To(BeNotFoundError())

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResource), operatorManagedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            OperatorManagedResourceName,
					Namespace:       namespace,
					Labels:          map[string]string{v1beta1constants.GardenRole: "seed-system-component"},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: operatorManagedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(operatorManagedResource).To(DeepEqual(expectedMr))
			Expect(operatorManagedResource).To(consistOf(
				serviceAccount,
				clusterRole,
				clusterRoleBinding,
				role,
				roleBinding,
				deployment,
				vpa,
			))

			operatorManagedResourceSecret.Name = operatorManagedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResourceSecret), operatorManagedResourceSecret)).To(Succeed())
			Expect(operatorManagedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(operatorManagedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(operatorManagedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, operatorManagedResource)).To(Succeed())
			Expect(c.Create(ctx, operatorManagedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResource), operatorManagedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResourceSecret), operatorManagedResourceSecret)).To(BeNotFoundError())
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
