// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator

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
	OperatorManagedResourceName = name

	name               = "opentelemetry-operator"
	serviceAccountName = name
	roleName           = name
	clusterRoleName    = name
)

// Values keeps values for the OpenTelemetry Operator.
type Values struct {
	// Image is the image of the OpenTelemetry Operator.
	Image string
	// PriorityClassName is the name of the priority class of the OpenTelemetry Operator.
	PriorityClassName string
}

type openTelemetryOperator struct {
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
	return &openTelemetryOperator{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (o *openTelemetryOperator) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		serviceAccount     = o.serviceAccount()
		clusterRole        = o.clusterRole()
		clusterRoleBinding = o.clusterRoleBinding()
		role               = o.role()
		roleBinding        = o.roleBinding()
		deployment         = o.deployment()
		vpa                = o.vpa()
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

	return managedresources.CreateForSeed(ctx, o.client, o.namespace, OperatorManagedResourceName, false, serializedResources)
}

func (o *openTelemetryOperator) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, o.client, o.namespace, OperatorManagedResourceName)
}

var timeoutWaitForManagedResources = 2 * time.Minute

func (o *openTelemetryOperator) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, o.client, o.namespace, OperatorManagedResourceName)
}

func (o *openTelemetryOperator) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, o.client, o.namespace, OperatorManagedResourceName)
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:   name,
		v1beta1constants.LabelRole:  v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleObservability,
	}
}

func (o *openTelemetryOperator) serviceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: o.namespace,
			Labels:    getLabels(),
		},
		AutomountServiceAccountToken: ptr.To(false),
	}
}

func (*openTelemetryOperator) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
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
}

func (o *openTelemetryOperator) clusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: getLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: o.namespace,
		}},
	}
}

func (o *openTelemetryOperator) role() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: o.namespace,
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
}

func (o *openTelemetryOperator) roleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: o.namespace,
			Labels:    getLabels(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      name,
				Namespace: o.namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     roleName,
		},
	}
}

func (o *openTelemetryOperator) deployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.DeploymentNameOpenTelemetryOperator,
			Namespace: o.namespace,
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
					PriorityClassName:  o.values.PriorityClassName,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To[int64](65532),
						RunAsGroup:   ptr.To[int64](65532),
						FSGroup:      ptr.To[int64](65532),
					},
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           o.values.Image,
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
						},
					},
				},
			},
		},
	}
}

func (o *openTelemetryOperator) vpa() *vpaautoscalingv1.VerticalPodAutoscaler {
	vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: o.namespace,
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
}
