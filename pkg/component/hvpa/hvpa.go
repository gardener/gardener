// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package hvpa

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/monitoring/prometheus/cache"
	monitoringutils "github.com/gardener/gardener/pkg/component/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ManagedResourceName is the name of the managed resource for the resources.
	ManagedResourceName = "hvpa"

	deploymentName = "hvpa-controller"
	containerName  = "hvpa-controller"
	serviceName    = "hvpa-controller"
	roleName       = "hvpa-controller"

	portNameMetrics = "metrics"
	portMetrics     = 9569
)

// Interface contains functions for an HVPA deployer.
type Interface interface {
	component.DeployWaiter
}

// New creates a new instance of DeployWaiter for the HVPA controller.
func New(client client.Client, namespace string, values Values) Interface {
	return &hvpa{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type hvpa struct {
	client    client.Client
	namespace string
	values    Values
}

// Values is a set of configuration values for the HVPA component.
type Values struct {
	// Image is the container image.
	Image string
	// KubernetesVersion is the version of the runtime cluster.
	KubernetesVersion *semver.Version
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
}

func (h *hvpa) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hvpa-controller",
				Namespace: h.namespace,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "system:hvpa-controller",
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "replicationcontrollers"},
					Verbs:     []string{"get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
					Verbs:     []string{"get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"autoscaling"},
					Resources: []string{"horizontalpodautoscalers"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"autoscaling.k8s.io"},
					Resources: []string{"hvpas"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"autoscaling.k8s.io"},
					Resources: []string{"hvpas/status"},
					Verbs:     []string{"get", "patch", "update"},
				},
				{
					APIGroups: []string{"autoscaling.k8s.io"},
					Resources: []string{"verticalpodautoscalers"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{"jobs"},
					Verbs:     []string{"get", "list", "patch", "update", "watch"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "hvpa-controller-rolebinding",
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
				Namespace: h.namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{"coordination.k8s.io"},
					Resources:     []string{"leases"},
					ResourceNames: []string{"hvpa-controller"},
					Verbs:         []string{"get", "watch", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "get", "list", "watch", "patch"},
				},
			},
		}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      role.Name,
				Namespace: h.namespace,
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
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: h.namespace,
				Labels:    utils.MergeStringMaps(getLabels(), getDeploymentLabels()),
			},
			Spec: corev1.ServiceSpec{
				Type:            corev1.ServiceTypeClusterIP,
				SessionAffinity: corev1.ServiceAffinityNone,
				Selector:        getDeploymentLabels(),
				Ports: []corev1.ServicePort{{
					Name:       portNameMetrics,
					Protocol:   corev1.ProtocolTCP,
					Port:       portMetrics,
					TargetPort: intstr.FromInt32(portMetrics),
				}},
			},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: h.namespace,
				Labels: utils.MergeStringMaps(getLabels(), getDeploymentLabels(), map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
				}),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To(int32(1)),
				RevisionHistoryLimit: ptr.To(int32(2)),
				Selector:             &metav1.LabelSelector{MatchLabels: utils.MergeStringMaps(getLabels(), getDeploymentLabels())},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: utils.MergeStringMaps(getLabels(), getDeploymentLabels(), map[string]string{
							v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
						}),
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  h.values.PriorityClassName,
						ServiceAccountName: serviceAccount.Name,
						Containers: []corev1.Container{{
							Name:            containerName,
							Image:           h.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"./manager",
								"--logtostderr=true",
								"--leader-elect=true",
								"--enable-detailed-metrics=true",
								fmt.Sprintf("--metrics-bind-address=:%d", portMetrics),
								"--v=2",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("500Mi"),
								},
							},
							Ports: []corev1.ContainerPort{{
								ContainerPort: portMetrics,
							}},
						}},
					},
				},
			},
		}
		vpaUpdateMode = vpaautoscalingv1.UpdateModeAuto
		vpa           = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hvpa-controller-vpa",
				Namespace: h.namespace,
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

		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: deployment.Namespace,
				Labels:    utils.MergeStringMaps(getLabels(), getDeploymentLabels()),
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: utils.IntStrPtrFromInt32(1),
				Selector:       deployment.Spec.Selector,
			},
		}

		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: monitoringutils.ConfigObjectMeta(serviceName, h.namespace, cache.Label),
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: getDeploymentLabels()},
				Endpoints: []monitoringv1.Endpoint{{
					Port: portNameMetrics,
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
						"hvpa_aggregate_applied_scaling_total",
						"hvpa_aggregate_blocked_scalings_total",
						"hvpa_spec_replicas",
						"hvpa_status_replicas",
						"hvpa_status_applied_hpa_current_replicas",
						"hvpa_status_applied_hpa_desired_replicas",
						"hvpa_status_applied_vpa_recommendation",
						"hvpa_status_blocked_hpa_current_replicas",
						"hvpa_status_blocked_hpa_desired_replicas",
						"hvpa_status_blocked_vpa_recommendation",
					),
				}},
			},
		}
	)

	kubernetesutils.SetAlwaysAllowEviction(podDisruptionBudget, h.values.KubernetesVersion)

	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service, networkingv1.NetworkPolicyPort{
		Port:     utils.IntStrPtrFromInt32(portMetrics),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}))

	resources, err := registry.AddAllAndSerialize(
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		role,
		roleBinding,
		service,
		deployment,
		podDisruptionBudget,
		vpa,
		serviceMonitor,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, h.client, h.namespace, ManagedResourceName, false, resources)
}

func (h *hvpa) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, h.client, h.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (h *hvpa) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, h.client, h.namespace, ManagedResourceName)
}

func (h *hvpa) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, h.client, h.namespace, ManagedResourceName)
}

func getLabels() map[string]string {
	return map[string]string{v1beta1constants.GardenRole: "hvpa"}
}

func getDeploymentLabels() map[string]string {
	return map[string]string{v1beta1constants.LabelApp: "hvpa-controller"}
}
