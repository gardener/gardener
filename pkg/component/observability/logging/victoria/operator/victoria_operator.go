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
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	// name is the base name for victoria-operator resources.
	name = "victoria-operator"

	// Resource names
	containerName           = name
	managedResourceName     = name
	serviceAccountName      = name
	deploymentName          = name
	clusterRoleName         = name
	clusterRoleBindingName  = name
	podDisruptionBudgetName = name
	vpaName                 = name

	// Ports
	healthProbePort = 8081
	metricsPort     = 8080
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

	resources, err := registry.AddAllAndSerialize(
		v.serviceAccount(),
		v.deployment(),
		v.vpa(),
		v.clusterRole(),
		v.clusterRoleBinding(),
		v.podDisruptionBudget(),
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
							Name:            containerName,
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
								RunAsNonRoot:             ptr.To(true),
							},
						},
					},
				},
			},
		},
	}

	utilruntime.Must(references.InjectAnnotations(deployment))
	return deployment
}

func (v *victoriaOperator) vpa() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vpaName,
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
						ContainerName: containerName,
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
					{
						ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
						Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
					},
				},
			},
		},
	}
}

func (v *victoriaOperator) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: GetLabels(),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"persistentvolumeclaims", "persistentvolumeclaims/finalizers",
					"services", "services/finalizers", "serviceaccounts", "serviceaccounts/finalizers",
				},
				Verbs: []string{"create", "watch", "list", "get", "delete", "patch", "update"},
			},
			{
				APIGroups: []string{appsv1.GroupName},
				Resources: []string{
					"deployments", "deployments/finalizers",
				},
				Verbs: []string{"list", "watch", "create", "get", "delete", "patch", "update"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"configmaps/status", "pods",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{
					"events",
				},
				Verbs: []string{"create"},
			},
			{
				APIGroups: []string{"operator.victoriametrics.com"},
				Resources: []string{
					"vlsingles", "vlsingles/finalizers", "vlsingles/status",
				},
				Verbs: []string{"*"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "create", "update"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

func (v *victoriaOperator) clusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleBindingName,
			Labels: GetLabels(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
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

func (v *victoriaOperator) podDisruptionBudget() *policyv1.PodDisruptionBudget {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podDisruptionBudgetName,
			Namespace: v.namespace,
			Labels:    GetLabels(),
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable:             ptr.To(intstr.FromInt32(1)),
			Selector:                   &metav1.LabelSelector{MatchLabels: GetLabels()},
			UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
		},
	}

	return pdb
}

// GetLabels returns the labels for the victoria-operator.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:                    name,
		v1beta1constants.LabelRole:                   v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole:                  v1beta1constants.GardenRoleObservability,
		resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
	}
}
