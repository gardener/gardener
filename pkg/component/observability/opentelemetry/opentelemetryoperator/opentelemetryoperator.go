// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package opentelemetryoperator

import (
	"context"
	"time"

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

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// OperatorManagedResourceName is the name of the OpenTelemetry Operator managed resource.
	OperatorManagedResourceName = "opentelemetry-operator"
	name                        = "opentelemetry-operator"
	roleName                    = "gardener.cloud:opentelemetry:opentelemetry-operator"
)

// Values keeps values for the OpenTelemetry Operator.
type Values struct {
	// Image is the image of the OpenTelemetry Operator.
	Image string
	// PriorityClassName is the name of the priority class of the OpenTelemetry Operator.
	PriorityClassName string
}

type opentelemetryOperator struct {
	client    client.Client
	namespace string
	values    Values
}

// NewOpenTelemetryOperator creates a new instance of OpenTelemetry Operator.
func NewOpenTelemetryOperator(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &opentelemetryOperator{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (otel *opentelemetryOperator) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: otel.namespace,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   roleName,
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
					APIGroups: []string{"config.openshift.io"},
					Resources: []string{"infrastructures", "infrastructures/status"},
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
				{
					APIGroups: []string{"route.openshift.io"},
					Resources: []string{"routes", "routes/custom-host"},
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
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleName,
				Namespace: otel.namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"list", "patch", "create", "get", "watch", "update", "delete"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps/status"},
					Verbs:     []string{"get", "update", "patch"},
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
				Name:      role.Name,
				Namespace: otel.namespace,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      serviceAccount.Name,
					Namespace: serviceAccount.Namespace,
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
				Namespace: otel.namespace,
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
						ServiceAccountName: serviceAccount.Name,
						PriorityClassName:  otel.values.PriorityClassName,
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: ptr.To(true),
							RunAsUser:    ptr.To[int64](65532),
							RunAsGroup:   ptr.To[int64](65532),
							FSGroup:      ptr.To[int64](65532),
						},
						Containers: []corev1.Container{
							{
								Name:            name,
								Image:           otel.values.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args: []string{
									"--metrics-addr=127.0.0.1:8080",
									"--enable-leader-election",
									"--zap-log-level=info",
									"--zap-time-encoding=rfc3339nano",
									"--enable-nginx-instrumentation=true",
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
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
								},
							},
						},
					},
				},
			},
		}
		vpaUpdateMode = vpaautoscalingv1.UpdateModeAuto
		vpa           = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: otel.namespace,
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
	)

	utilruntime.Must(references.InjectAnnotations(deployment))

	serializedResources, err := registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		role,
		roleBinding,
		deployment,
		vpa,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, otel.client, otel.namespace, OperatorManagedResourceName, false, serializedResources)
}

func (otel *opentelemetryOperator) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, otel.client, otel.namespace, OperatorManagedResourceName)
}

var timeoutWaitForManagedResources = 2 * time.Minute

func (otel *opentelemetryOperator) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, otel.client, otel.namespace, OperatorManagedResourceName)
}

func (otel *opentelemetryOperator) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, otel.client, otel.namespace, OperatorManagedResourceName)
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:   name,
		v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
	}
}
