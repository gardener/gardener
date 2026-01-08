// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// managedResourceName is the name of the ManagedResource for the victoria-operator resources.
	managedResourceName = "victoria-operator"
	// serviceAccountName is the name of the ServiceAccount for the victoria-operator.
	serviceAccountName = "victoria-operator"
	// deploymentName is the name of the Deployment for the victoria-operator.
	deploymentName = "victoria-operator"
	// healthProbePort is the port for health probe.
	healthProbePort = 8081
	// metricsPort is the port for metrics.
	metricsPort = 8080
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

// Values contains configuration values for the victoria-operator resources.
type Values struct {
	// Image defines the container image of victoria-operator.
	Image string
	// PriorityClassName is the name of the priority class for the deployment.
	PriorityClassName string
}

// New creates a new instance of DeployWaiter for the victoria-operator.
func New(client client.Client, namespace string, values Values) component.DeployWaiter {
	return &victoriaOperator{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type victoriaOperator struct {
	client    client.Client
	namespace string
	values    Values
}

func (v *victoriaOperator) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	deployment := v.deployment()
	if err := references.InjectAnnotations(deployment); err != nil {
		return err
	}

	resources, err := registry.AddAllAndSerialize(
		v.serviceAccount(),
		deployment,
		v.vpa(),
		v.clusterRole(),
		v.clusterRoleBinding(),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, v.client, v.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, resources)
}

func (v *victoriaOperator) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, v.client, v.namespace, managedResourceName)
}

func (v *victoriaOperator) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, v.client, v.namespace, managedResourceName)
}

func (v *victoriaOperator) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, v.client, v.namespace, managedResourceName)
}

func (v *victoriaOperator) serviceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: v.namespace,
			Labels:    GetLabels(),
		},
		AutomountServiceAccountToken: ptr.To(false),
	}
}

func (v *victoriaOperator) deployment() *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: v.namespace,
			Labels:    GetLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             ptr.To[int32](1),
			RevisionHistoryLimit: ptr.To[int32](2),
			Selector:             &metav1.LabelSelector{MatchLabels: GetLabels()},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					PriorityClassName:  v.values.PriorityClassName,
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "victoria-operator",
							Image:           v.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--leader-elect",
								fmt.Sprintf("--health-probe-bind-address=:%d", healthProbePort),
								fmt.Sprintf("--metrics-bind-address=:%d", metricsPort),
								"--controller.disableReconcileFor=VLAgent,VLCluster,VLogs,VMAgent,VMAlert,VMAlertmanager,VMAlertmanagerConfig,VMAnomaly,VMAuth,VMCluster,VMNodeScrape,VMPodScrape,VMProbe,VMRule,VMScrapeConfig,VMServiceScrape,VMSingle,VMStaticScrape,VMUser,VTSingle,VTCluster",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "WATCH_NAMESPACE",
									Value: "",
								},
								{
									Name:  "VM_ENABLEDPROMETHEUSCONVERTER_PODMONITOR",
									Value: "false",
								},
								{
									Name:  "VM_ENABLEDPROMETHEUSCONVERTER_SERVICESCRAPE",
									Value: "false",
								},
								{
									Name:  "VM_ENABLEDPROMETHEUSCONVERTER_PROMETHEUSRULE",
									Value: "false",
								},
								{
									Name:  "VM_ENABLEDPROMETHEUSCONVERTER_PROBE",
									Value: "false",
								},
								{
									Name:  "VM_ENABLEDPROMETHEUSCONVERTER_ALERTMANAGERCONFIG",
									Value: "false",
								},
								{
									Name:  "VM_ENABLEDPROMETHEUSCONVERTER_SCRAPECONFIG",
									Value: "false",
								},
								{
									Name:  "VM_DISABLESELFSERVICESCRAPECREATION",
									Value: "true",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("80m"),
									corev1.ResourceMemory: resource.MustParse("120Mi"),
								},
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("120m"),
									corev1.ResourceMemory: resource.MustParse("520Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt32(healthProbePort),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt32(healthProbePort),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
							},
						},
					},
				},
			},
		},
	}

	return deployment
}

func (v *victoriaOperator) vpa() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: v.namespace,
			Labels:    GetLabels(),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       deploymentName,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
		},
	}
}

func (v *victoriaOperator) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "victoria-operator",
			Labels: GetLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				NonResourceURLs: []string{"/metrics", "/metrics/resources", "/metrics/slis"},
				Verbs:           []string{"get", "watch", "list"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"configmaps", "configmaps/finalizers", "endpoints", "events",
					"persistentvolumeclaims", "persistentvolumeclaims/finalizers",
					"pods/eviction", "secrets", "secrets/finalizers", "services",
					"services/finalizers", "serviceaccounts", "serviceaccounts/finalizers",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"configmaps/status", "pods", "nodes", "nodes/proxy",
					"nodes/metrics", "namespaces",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{appsv1.GroupName},
				Resources: []string{
					"deployments", "deployments/finalizers", "statefulsets",
					"statefulsets/finalizers", "daemonsets", "daemonsets/finalizers",
					"replicasets", "statefulsets/status",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{"monitoring.coreos.com"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{rbacv1.GroupName},
				Resources: []string{
					"clusterrolebindings", "clusterrolebindings/finalizers",
					"clusterroles", "clusterroles/finalizers", "roles", "rolebindings",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"list", "get", "watch"},
			},
			{
				APIGroups: []string{"policy"},
				Resources: []string{"poddisruptionbudgets", "poddisruptionbudgets/finalizers"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"autoscaling"},
				Resources: []string{"horizontalpodautoscalers"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses", "ingresses/finalizers"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{"discovery.k8s.io"},
				Resources: []string{"endpointslices"},
				Verbs:     []string{"list", "watch", "get"},
			},
			{
				APIGroups: []string{"operator.victoriametrics.com"},
				Resources: []string{
					"vlagents", "vlagents/finalizers", "vlagents/status",
					"vlogs", "vlogs/finalizers", "vlogs/status",
					"vlsingles", "vlsingles/finalizers", "vlsingles/status",
					"vlclusters", "vlclusters/finalizers", "vlclusters/status",
					"vmagents", "vmagents/finalizers", "vmagents/status",
					"vmalertmanagerconfigs", "vmalertmanagerconfigs/finalizers", "vmalertmanagerconfigs/status",
					"vmalertmanagers", "vmalertmanagers/finalizers", "vmalertmanagers/status",
					"vmalerts", "vmalerts/finalizers", "vmalerts/status",
					"vmauths", "vmauths/finalizers", "vmauths/status",
					"vmclusters", "vmclusters/finalizers", "vmclusters/status",
					"vmnodescrapes", "vmnodescrapes/finalizers", "vmnodescrapes/status",
					"vmpodscrapes", "vmpodscrapes/finalizers", "vmpodscrapes/status",
					"vmprobes", "vmprobes/finalizers", "vmprobes/status",
					"vmrules", "vmrules/finalizers", "vmrules/status",
					"vmscrapeconfigs", "vmscrapeconfigs/finalizers", "vmscrapeconfigs/status",
					"vmservicescrapes", "vmservicescrapes/finalizers", "vmservicescrapes/status",
					"vmsingles", "vmsingles/finalizers", "vmsingles/status",
					"vmstaticscrapes", "vmstaticscrapes/finalizers", "vmstaticscrapes/status",
					"vmusers", "vmusers/finalizers", "vmusers/status",
					"vmanomalies", "vmanomalies/finalizers", "vmanomalies/status",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"*"},
			},
		},
	}
}

func (v *victoriaOperator) clusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "victoria-operator",
			Labels: GetLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "victoria-operator",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: v.namespace,
			},
		},
	}
}

// GetLabels returns the labels for the victoria-operator.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:   "victoria-operator",
		v1beta1constants.LabelRole:  v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleObservability,
	}
}
