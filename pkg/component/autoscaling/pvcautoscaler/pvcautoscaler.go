// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package pvcautoscaler

import (
	"context"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/cache"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// PVCAutoscalerManagedResourceName is the name of the PVCAutoscaler managed resource.
	PVCAutoscalerManagedResourceName = name

	name               = "pvc-autoscaler"
	serviceAccountName = name
	metricsPortName    = "metrics"
	metricsPort        = int32(8080)
)

// Values keeps values for the PVCAutoscaler.
type Values struct {
	// Image is the image of the PVCAutoscaler.
	Image string
	// PriorityClassName is the name of the priority class of the PVCAutoscaler.
	PriorityClassName string
}

type pvcautoscaler struct {
	client    client.Client
	namespace string
	values    Values
}

// NewPVCAutoscaler creates a new instance of PVCAutoscaler.
func NewPVCAutoscaler(
	client client.Client,
	namespace string,
	values Values,
) component.DeployWaiter {
	return &pvcautoscaler{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

func (p *pvcautoscaler) Deploy(ctx context.Context) error {
	var (
		registry            = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		serviceAccount      = p.serviceAccount()
		clusterRoles        = p.clusterRoles()
		clusterRoleBindings = p.clusterRoleBindings()
		role                = p.role()
		roleBinding         = p.roleBinding()
		service             = p.service()
		deployment          = p.deployment()
		pdb                 = p.podDisruptionBudget()
		vpa                 = p.verticalPodAutoscaler()
		serviceMonitor      = p.serviceMonitor()
	)

	utilruntime.Must(references.InjectAnnotations(deployment))
	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service, networkingv1.NetworkPolicyPort{
		Port:     new(intstr.FromInt32(metricsPort)),
		Protocol: new(corev1.ProtocolTCP),
	}))

	resources := []client.Object{
		serviceAccount,
		role,
		roleBinding,
		service,
		deployment,
		pdb,
		vpa,
		serviceMonitor,
	}

	for _, cr := range clusterRoles {
		resources = append(resources, cr)
	}

	for _, crb := range clusterRoleBindings {
		resources = append(resources, crb)
	}

	serializedResources, err := registry.AddAllAndSerialize(resources...)
	if err != nil {
		return err
	}
	return managedresources.CreateForSeed(ctx, p.client, p.namespace, PVCAutoscalerManagedResourceName, false, serializedResources)
}

func (p *pvcautoscaler) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, p.client, p.namespace, PVCAutoscalerManagedResourceName)
}

var timeoutWaitForManagedResources = 2 * time.Minute

func (p *pvcautoscaler) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, p.client, p.namespace, PVCAutoscalerManagedResourceName)
}

func (p *pvcautoscaler) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, p.client, p.namespace, PVCAutoscalerManagedResourceName)
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp: name,
	}
}

func (p *pvcautoscaler) serviceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		AutomountServiceAccountToken: new(false),
	}
}

func (p *pvcautoscaler) clusterRoles() []*rbacv1.ClusterRole {
	return []*rbacv1.ClusterRole{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pvc-autoscaler-autoscaling-persistentvolumeclaimautoscaler-editor-role",
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"autoscaling.gardener.cloud"},
					Resources: []string{"persistentvolumeclaimautoscalers"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"autoscaling.gardener.cloud"},
					Resources: []string{"persistentvolumeclaimautoscalers/status"},
					Verbs:     []string{"get"},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pvc-autoscaler-autoscaling-persistentvolumeclaimautoscaler-viewer-role",
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"autoscaling.gardener.cloud"},
					Resources: []string{"persistentvolumeclaimautoscalers"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{"autoscaling.gardener.cloud"},
					Resources: []string{"persistentvolumeclaimautoscalers/status"},
					Verbs:     []string{"get"},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pvc-autoscaler-manager-role",
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"*"},
					Resources: []string{"*/scale"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"persistentvolumeclaims"},
					Verbs:     []string{"get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"persistentvolumeclaims/status"},
					Verbs:     []string{"get"},
				},
				{
					APIGroups: []string{"autoscaling.gardener.cloud"},
					Resources: []string{"persistentvolumeclaimautoscalers"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"autoscaling.gardener.cloud"},
					Resources: []string{"persistentvolumeclaimautoscalers/finalizers"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups: []string{"autoscaling.gardener.cloud"},
					Resources: []string{"persistentvolumeclaimautoscalers/status"},
					Verbs:     []string{"get", "patch", "update"},
				},
				{
					APIGroups: []string{"storage.k8s.io"},
					Resources: []string{"storageclasses"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pvc-autoscaler-metrics-reader",
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					NonResourceURLs: []string{"/metrics"},
					Verbs:           []string{"get"},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pvc-autoscaler-proxy-role",
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"authentication.k8s.io"},
					Resources: []string{"tokenreviews"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"authorization.k8s.io"},
					Resources: []string{"subjectaccessreviews"},
					Verbs:     []string{"create"},
				},
			},
		},
	}
}

func (p *pvcautoscaler) clusterRoleBindings() []*rbacv1.ClusterRoleBinding {
	return []*rbacv1.ClusterRoleBinding{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pvc-autoscaler-manager-rolebinding",
				Labels: getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "pvc-autoscaler-manager-role",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: p.namespace,
			}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "pvc-autoscaler-proxy-rolebinding",
				Labels: getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "pvc-autoscaler-proxy-role",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: p.namespace,
			}},
		},
	}
}

func (p *pvcautoscaler) role() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-autoscaler-leader-election-role",
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch"},
			},
		},
	}
}

func (p *pvcautoscaler) roleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-autoscaler-leader-election-rolebinding",
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: p.namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "pvc-autoscaler-leader-election-role",
		},
	}
}

func (p *pvcautoscaler) service() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-autoscaler-controller-manager-metrics-service",
			Namespace: p.namespace,
			Labels: utils.MergeStringMaps(getLabels(), map[string]string{
				"control-plane": "controller-manager",
			}),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       8443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("https"),
				},
				{
					Name:       metricsPortName,
					Port:       metricsPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString(metricsPortName),
				},
			},
			Selector: map[string]string{
				"control-plane": "controller-manager",
			},
		},
	}
}

func (p *pvcautoscaler) deployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.DeploymentNamePVCAutoscaler,
			Namespace: p.namespace,
			Labels: utils.MergeStringMaps(getLabels(), map[string]string{
				resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
				"control-plane": "controller-manager",
			}),
		},
		Spec: appsv1.DeploymentSpec{
			RevisionHistoryLimit: new(int32(2)),
			Replicas:             new(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: utils.MergeStringMaps(getLabels(), map[string]string{
					"control-plane": "controller-manager",
				}),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(getLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:                   v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:      v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("prometheus-cache", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
						"control-plane": "controller-manager",
					}),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					PriorityClassName:  p.values.PriorityClassName,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: new(true),
						RunAsUser:    new(int64(65532)),
						RunAsGroup:   new(int64(65532)),
						FSGroup:      new(int64(65532)),
					},
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           p.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--health-probe-bind-address=:8081",
								"--metrics-bind-address=:8080",
								"--leader-elect",
								"--interval=30s",
								"--prometheus-address=http://prometheus-cache.garden.svc.cluster.local:80",
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
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "health",
									ContainerPort: 8081,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          metricsPortName,
									ContainerPort: metricsPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromString("health"),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromString("health"),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								FailureThreshold:    3,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: new(false),
							},
						},
					},
				},
			},
		},
	}
}

func (p *pvcautoscaler) podDisruptionBudget() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: new(intstr.FromInt32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: utils.MergeStringMaps(getLabels(), map[string]string{
					"control-plane": "controller-manager",
				}),
			},
			UnhealthyPodEvictionPolicy: new(policyv1.AlwaysAllow),
		},
	}
}

func (p *pvcautoscaler) verticalPodAutoscaler() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: p.namespace,
			Labels:    getLabels(),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       v1beta1constants.DeploymentNamePVCAutoscaler,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: new(vpaautoscalingv1.UpdateModeRecreate),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName:    name,
						ControlledValues: new(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
					{
						ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
						Mode:          new(vpaautoscalingv1.ContainerScalingModeOff),
					},
				},
			},
		},
	}
}

func (p *pvcautoscaler) serviceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(name, p.namespace, cache.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				Port: metricsPortName,
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						Action:      "replace",
						Replacement: new(name),
						TargetLabel: "job",
					},
				},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"pvc_autoscaler_resized_total",
					"pvc_autoscaler_threshold_reached_total",
					"pvc_autoscaler_max_capacity_reached_total",
					"pvc_autoscaler_skipped_total",
				),
			}},
		},
	}
}
