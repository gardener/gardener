// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/opentelemetry/operator"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("OpenTelemetry Operator", func() {
	var (
		ctx = context.Background()

		name              = "opentelemetry-operator"
		namespace         = "some-namespace"
		image             = "some-image:some-tag"
		priorityClassName = "some-priority-class"
		values            Values

		c         client.Client
		component component.DeployWaiter
		consistOf func(...client.Object) types.GomegaMatcher

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

		values = Values{
			Image:             image,
			PriorityClassName: priorityClassName,
		}
		component = NewOpenTelemetryOperator(c, namespace, values)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps", "pods", "serviceaccounts", "services"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces", "secrets"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets", "deployments", "statefulsets"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"replicasets"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"autoscaling"},
					Resources: []string{"horizontalpodautoscalers"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{"jobs"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create", "get", "list", "update"},
				},
				{
					APIGroups: []string{"monitoring.coreos.com"},
					Resources: []string{"podmonitors", "servicemonitors"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"networking.k8s.io"},
					Resources: []string{"ingresses"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"opentelemetry.io"},
					Resources: []string{"instrumentations", "opentelemetrycollectors"},
					Verbs:     []string{"get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"opentelemetry.io"},
					Resources: []string{"opampbridges", "targetallocators"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"opentelemetry.io"},
					Resources: []string{"opampbridges/finalizers"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups: []string{"opentelemetry.io"},
					Resources: []string{"opampbridges/status", "opentelemetrycollectors/finalizers", "opentelemetrycollectors/status", "targetallocators/status"},
					Verbs:     []string{"get", "patch", "update"},
				},
				{
					APIGroups: []string{"policy"},
					Resources: []string{"poddisruptionbudgets"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"list", "patch", "create", "get", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch"},
				},
			},
		}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    getLabels(),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      serviceAccount.Name,
					Namespace: namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     role.Name,
			},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.DeploymentNameOpenTelemetryOperator,
				Namespace: namespace,
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
				}),
			},
			Spec: appsv1.DeploymentSpec{
				RevisionHistoryLimit: ptr.To[int32](2),
				Replicas:             ptr.To[int32](1),
				Selector: &metav1.LabelSelector{
					MatchLabels: getLabels(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(getLabels(), map[string]string{
							v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
						}),
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: name,
						PriorityClassName:  priorityClassName,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: ptr.To(true),
							RunAsUser:    ptr.To[int64](65532),
							RunAsGroup:   ptr.To[int64](65532),
							FSGroup:      ptr.To[int64](65532),
						},
						Containers: []corev1.Container{
							{
								Name:            name,
								Image:           image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--metrics-addr=127.0.0.1:8080",
									"--enable-leader-election",
									"--zap-log-level=info",
									"--zap-time-encoding=rfc3339nano",
								},
								Env: []corev1.EnvVar{
									{
										Name:  "ENABLE_WEBHOOKS",
										Value: "false",
									},
									{
										Name: "NAMESPACE",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.namespace",
											},
										},
									},
									{
										Name: "SERVICE_ACCOUNT_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "spec.serviceAccountName",
											},
										},
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
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
				Labels:    getLabels(),
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       v1beta1constants.DeploymentNameOpenTelemetryOperator,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: name,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("64Mi"),
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

			managedResource := &resourcesv1alpha1.ManagedResource{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResource), managedResource)).To(Succeed())
			managedResourceSecret := &corev1.Secret{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(operatorManagedResourceSecret), managedResourceSecret)).To(Succeed())

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
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       OperatorManagedResourceName,
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
						Name:       OperatorManagedResourceName,
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
						Name:      OperatorManagedResourceName,
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

func getLabels() map[string]string {
	return map[string]string{
		"app":                 "opentelemetry-operator",
		"role":                "observability",
		"gardener.cloud/role": "observability",
	}
}
