// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubestatemetrics_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
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
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("KubeStateMetrics", func() {
	var (
		ctx = context.Background()

		namespace         = "some-namespace"
		image             = "some-image:some-tag"
		priorityClassName = "some-priorityclass"
		values            = Values{}

		c         client.Client
		sm        secretsmanager.Interface
		ksm       component.DeployWaiter
		consistOf func(...client.Object) gomegatypes.GomegaMatcher

		managedResourceName       string
		managedResourceTargetName string
		managedResource           *resourcesv1alpha1.ManagedResource
		managedResourceTarget     *resourcesv1alpha1.ManagedResource
		managedResourceSecret     *corev1.Secret

		vpaUpdateMode       = vpaautoscalingv1.UpdateModeAuto
		vpaControlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly

		serviceAccountFor            func(string) *corev1.ServiceAccount
		secretShootAccess            *corev1.Secret
		vpaFor                       func(string) *vpaautoscalingv1.VerticalPodAutoscaler
		pdbFor                       func(string) *policyv1.PodDisruptionBudget
		customResourceStateConfigMap *corev1.ConfigMap

		clusterRoleFor = func(clusterType component.ClusterType, nameSuffix string) *rbacv1.ClusterRole {
			name := "gardener.cloud:monitoring:kube-state-metrics"
			if clusterType == component.ClusterTypeSeed {
				name += values.NameSuffix
			}

			obj := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"component": "kube-state-metrics" + nameSuffix,
						"type":      string(clusterType),
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{
							"nodes",
							"pods",
							"services",
							"resourcequotas",
							"replicationcontrollers",
							"limitranges",
							"persistentvolumeclaims",
							"namespaces",
						},
						Verbs: []string{"list", "watch"},
					},
					{
						APIGroups: []string{"apps", "extensions"},
						Resources: []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
						Verbs:     []string{"list", "watch"},
					},
					{
						APIGroups: []string{"batch"},
						Resources: []string{"cronjobs", "jobs"},
						Verbs:     []string{"list", "watch"},
					},
					{
						APIGroups: []string{"apiextensions.k8s.io"},
						Resources: []string{"customresourcedefinitions"},
						Verbs:     []string{"list", "watch"},
					},
					{
						APIGroups: []string{"autoscaling.k8s.io"},
						Resources: []string{"verticalpodautoscalers"},
						Verbs:     []string{"list", "watch"},
					},
					{
						APIGroups: []string{"operator.gardener.cloud"},
						Resources: []string{"gardens"},
						Verbs:     []string{"list", "watch"},
					},
				},
			}

			if clusterType == component.ClusterTypeSeed {
				obj.Rules = append(obj.Rules, rbacv1.PolicyRule{
					APIGroups: []string{"autoscaling"},
					Resources: []string{"horizontalpodautoscalers"},
					Verbs:     []string{"list", "watch"},
				})
			}

			return obj
		}
		clusterRoleBindingFor = func(clusterType component.ClusterType, nameSuffix string) *rbacv1.ClusterRoleBinding {
			name := "gardener.cloud:monitoring:kube-state-metrics"
			if clusterType == component.ClusterTypeSeed {
				name += values.NameSuffix
			}

			obj := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"component": "kube-state-metrics" + nameSuffix,
						"type":      string(clusterType),
					},
					Annotations: map[string]string{
						"resources.gardener.cloud/delete-on-invalid-update": "true",
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     name,
				},
				Subjects: []rbacv1.Subject{{
					Kind: rbacv1.ServiceAccountKind,
					Name: "kube-state-metrics" + nameSuffix,
				}},
			}

			if clusterType == component.ClusterTypeSeed {
				obj.Subjects[0].Namespace = namespace
			} else {
				obj.Subjects[0].Namespace = "kube-system"
			}

			return obj
		}
		serviceFor = func(clusterType component.ClusterType) *corev1.Service {
			name := "kube-state-metrics"
			if clusterType == component.ClusterTypeSeed {
				name += values.NameSuffix
			}

			obj := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						"component": name,
						"type":      string(clusterType),
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Selector: map[string]string{
						"component": name,
						"type":      string(clusterType),
					},
					Ports: []corev1.ServicePort{{
						Name:       "metrics",
						Port:       80,
						TargetPort: intstr.FromInt32(8080),
						Protocol:   corev1.ProtocolTCP,
					}},
				},
			}

			if clusterType == component.ClusterTypeSeed {
				obj.Annotations = map[string]string{
					"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8080}]`,
					"networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports":   `[{"protocol":"TCP","port":8080}]`,
				}
			}
			if clusterType == component.ClusterTypeShoot {
				obj.Annotations = map[string]string{"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8080}]`}
			}

			return obj
		}
		deploymentFor = func(clusterType component.ClusterType) *appsv1.Deployment {
			name := "kube-state-metrics"
			if clusterType == component.ClusterTypeSeed {
				name += values.NameSuffix
			}

			var (
				maxUnavailable = intstr.FromInt32(1)
				selectorLabels = map[string]string{
					"component": "kube-state-metrics" + values.NameSuffix,
					"type":      string(clusterType),
				}

				deploymentLabels             map[string]string
				podLabels                    map[string]string
				args                         []string
				automountServiceAccountToken *bool
				serviceAccountName           string
				volumeMounts                 []corev1.VolumeMount
				volumes                      []corev1.Volume
			)

			if clusterType == component.ClusterTypeSeed {
				deploymentLabels = map[string]string{
					"component": name,
					"type":      string(clusterType),
					"role":      "monitoring",
				}
				podLabels = map[string]string{
					"component":                        name,
					"type":                             string(clusterType),
					"role":                             "monitoring",
					"networking.gardener.cloud/to-dns": "allowed",
					"networking.gardener.cloud/to-runtime-apiserver": "allowed",
				}
				if values.NameSuffix == SuffixSeed {
					args = []string{
						"--port=8080",
						"--telemetry-port=8081",
						"--resources=deployments,pods,statefulsets,nodes,horizontalpodautoscalers,persistentvolumeclaims,replicasets,namespaces",
						"--metric-labels-allowlist=nodes=[*],pods=[origin]",
						"--metric-annotations-allowlist=namespaces=[shoot.gardener.cloud/uid]",
						"--metric-allowlist=" +
							"^kube_daemonset_metadata_generation$," +
							"^kube_daemonset_status_current_number_scheduled$," +
							"^kube_daemonset_status_desired_number_scheduled$," +
							"^kube_daemonset_status_number_available$," +
							"^kube_daemonset_status_number_unavailable$," +
							"^kube_daemonset_status_updated_number_scheduled$," +
							"^kube_deployment_metadata_generation$," +
							"^kube_deployment_spec_replicas$," +
							"^kube_deployment_status_observed_generation$," +
							"^kube_deployment_status_replicas$," +
							"^kube_deployment_status_replicas_available$," +
							"^kube_deployment_status_replicas_unavailable$," +
							"^kube_deployment_status_replicas_updated$," +
							"^kube_horizontalpodautoscaler_spec_max_replicas$," +
							"^kube_horizontalpodautoscaler_spec_min_replicas$," +
							"^kube_horizontalpodautoscaler_status_current_replicas$," +
							"^kube_horizontalpodautoscaler_status_desired_replicas$," +
							"^kube_horizontalpodautoscaler_status_condition$," +
							"^kube_namespace_annotations$," +
							"^kube_node_info$," +
							"^kube_node_labels$," +
							"^kube_node_spec_taint$," +
							"^kube_node_spec_unschedulable$," +
							"^kube_node_status_allocatable$," +
							"^kube_node_status_capacity$," +
							"^kube_node_status_condition$," +
							"^kube_persistentvolumeclaim_resource_requests_storage_bytes$," +
							"^kube_pod_container_info$," +
							"^kube_pod_container_resource_limits$," +
							"^kube_pod_container_resource_requests$," +
							"^kube_pod_container_status_restarts_total$," +
							"^kube_pod_info$," +
							"^kube_pod_labels$," +
							"^kube_pod_owner$," +
							"^kube_pod_spec_volumes_persistentvolumeclaims_info$," +
							"^kube_pod_status_phase$," +
							"^kube_pod_status_ready$," +
							"^kube_replicaset_owner$," +
							"^kube_statefulset_metadata_generation$," +
							"^kube_statefulset_replicas$," +
							"^kube_statefulset_status_observed_generation$," +
							"^kube_statefulset_status_replicas$," +
							"^kube_statefulset_status_replicas_current$," +
							"^kube_statefulset_status_replicas_ready$," +
							"^kube_statefulset_status_replicas_updated$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_target_cpu$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_target_memory$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_uncappedtarget_cpu$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_uncappedtarget_memory$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_upperbound_cpu$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_upperbound_memory$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_lowerbound_cpu$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_lowerbound_memory$," +
							"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_minallowed_cpu$," +
							"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_minallowed_memory$," +
							"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_maxallowed_cpu$," +
							"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_maxallowed_memory$," +
							"^kube_customresource_verticalpodautoscaler_spec_updatepolicy_updatemode$",
						"--custom-resource-state-config-file=/config/custom-resource-state.yaml",
					}
				} else if values.NameSuffix == SuffixRuntime {
					args = []string{
						"--port=8080",
						"--telemetry-port=8081",
						"--resources=deployments,pods,statefulsets,nodes,horizontalpodautoscalers,persistentvolumeclaims,replicasets,namespaces",
						"--metric-labels-allowlist=nodes=[*],pods=[origin]",
						"--metric-annotations-allowlist=namespaces=[shoot.gardener.cloud/uid]",
						"--metric-allowlist=" +
							"^kube_pod_container_status_restarts_total$," +
							"^kube_pod_status_phase$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_target_cpu$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_target_memory$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_uncappedtarget_cpu$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_uncappedtarget_memory$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_upperbound_cpu$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_upperbound_memory$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_lowerbound_cpu$," +
							"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_lowerbound_memory$," +
							"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_minallowed_cpu$," +
							"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_minallowed_memory$," +
							"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_maxallowed_cpu$," +
							"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_maxallowed_memory$," +
							"^kube_customresource_verticalpodautoscaler_spec_updatepolicy_updatemode$," +
							"^garden_garden_condition$," +
							"^garden_garden_last_operation$",
						"--custom-resource-state-config-file=/config/custom-resource-state.yaml",
					}
				}
				serviceAccountName = "kube-state-metrics" + values.NameSuffix
			}

			volumes = []corev1.Volume{{
				Name: customResourceStateConfigMap.Name,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: customResourceStateConfigMap.Name,
						},
					},
				},
			}}

			volumeMounts = []corev1.VolumeMount{{
				Name:      customResourceStateConfigMap.Name,
				MountPath: "/config",
				ReadOnly:  true,
			}}

			if clusterType == component.ClusterTypeShoot {
				deploymentLabels = map[string]string{
					"component":           "kube-state-metrics",
					"type":                string(clusterType),
					"gardener.cloud/role": "monitoring",
				}
				podLabels = map[string]string{
					"component":                        "kube-state-metrics",
					"type":                             string(clusterType),
					"gardener.cloud/role":              "monitoring",
					"networking.gardener.cloud/to-dns": "allowed",
					"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
				}
				args = []string{
					"--port=8080",
					"--telemetry-port=8081",
					"--resources=daemonsets,deployments,nodes,pods,statefulsets,replicasets",
					"--namespaces=kube-system",
					"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
					"--metric-labels-allowlist=nodes=[*],pods=[origin]",
					"--metric-allowlist=" +
						"^kube_daemonset_metadata_generation$," +
						"^kube_daemonset_status_current_number_scheduled$," +
						"^kube_daemonset_status_desired_number_scheduled$," +
						"^kube_daemonset_status_number_available$," +
						"^kube_daemonset_status_number_unavailable$," +
						"^kube_daemonset_status_updated_number_scheduled$," +
						"^kube_deployment_metadata_generation$," +
						"^kube_deployment_spec_replicas$," +
						"^kube_deployment_status_observed_generation$," +
						"^kube_deployment_status_replicas$," +
						"^kube_deployment_status_replicas_available$," +
						"^kube_deployment_status_replicas_unavailable$," +
						"^kube_deployment_status_replicas_updated$," +
						"^kube_node_info$," +
						"^kube_node_labels$," +
						"^kube_node_spec_taint$," +
						"^kube_node_spec_unschedulable$," +
						"^kube_node_status_allocatable$," +
						"^kube_node_status_capacity$," +
						"^kube_node_status_condition$," +
						"^kube_pod_container_info$," +
						"^kube_pod_container_resource_limits$," +
						"^kube_pod_container_resource_requests$," +
						"^kube_pod_container_status_restarts_total$," +
						"^kube_pod_info$," +
						"^kube_pod_labels$," +
						"^kube_pod_status_phase$," +
						"^kube_pod_status_ready$," +
						"^kube_replicaset_owner$," +
						"^kube_replicaset_metadata_generation$," +
						"^kube_replicaset_spec_replicas$," +
						"^kube_replicaset_status_observed_generation$," +
						"^kube_replicaset_status_replicas$," +
						"^kube_replicaset_status_ready_replicas$," +
						"^kube_statefulset_metadata_generation$," +
						"^kube_statefulset_replicas$," +
						"^kube_statefulset_status_observed_generation$," +
						"^kube_statefulset_status_replicas$," +
						"^kube_statefulset_status_replicas_current$," +
						"^kube_statefulset_status_replicas_ready$," +
						"^kube_statefulset_status_replicas_updated$," +
						"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_target_cpu$," +
						"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_target_memory$," +
						"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_uncappedtarget_cpu$," +
						"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_uncappedtarget_memory$," +
						"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_upperbound_cpu$," +
						"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_upperbound_memory$," +
						"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_lowerbound_cpu$," +
						"^kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_lowerbound_memory$," +
						"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_minallowed_cpu$," +
						"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_minallowed_memory$," +
						"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_maxallowed_cpu$," +
						"^kube_customresource_verticalpodautoscaler_spec_resourcepolicy_containerpolicies_maxallowed_memory$," +
						"^kube_customresource_verticalpodautoscaler_spec_updatepolicy_updatemode$",
					"--custom-resource-state-config-file=/config/custom-resource-state.yaml",
				}
				automountServiceAccountToken = ptr.To(false)
				volumeMounts = append(volumeMounts, corev1.VolumeMount{
					Name:      "kubeconfig",
					MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
					ReadOnly:  true,
				})
				volumes = append(volumes, corev1.Volume{
					Name: "kubeconfig",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							DefaultMode: ptr.To[int32](420),
							Sources: []corev1.VolumeProjection{
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "generic-token-kubeconfig",
										},
										Items: []corev1.KeyToPath{{
											Key:  "kubeconfig",
											Path: "kubeconfig",
										}},
										Optional: ptr.To(false),
									},
								},
								{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "shoot-access-kube-state-metrics",
										},
										Items: []corev1.KeyToPath{{
											Key:  "token",
											Path: "token",
										}},
										Optional: ptr.To(false),
									},
								},
							},
						},
					},
				})
			}

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels:    deploymentLabels,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             ptr.To[int32](0),
					RevisionHistoryLimit: ptr.To[int32](2),
					Selector:             &metav1.LabelSelector{MatchLabels: selectorLabels},
					Strategy: appsv1.DeploymentStrategy{
						Type: appsv1.RollingUpdateDeploymentStrategyType,
						RollingUpdate: &appsv1.RollingUpdateDeployment{
							MaxUnavailable: &maxUnavailable,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: podLabels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:            "kube-state-metrics",
								Image:           image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Args:            args,
								Ports: []corev1.ContainerPort{{
									Name:          "metrics",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								}},
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/healthz",
											Port: intstr.FromInt32(8080),
										},
									},
									InitialDelaySeconds: 5,
									TimeoutSeconds:      5,
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/healthz",
											Port: intstr.FromInt32(8080),
										},
									},
									InitialDelaySeconds: 5,
									PeriodSeconds:       30,
									SuccessThreshold:    1,
									FailureThreshold:    3,
									TimeoutSeconds:      5,
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("32Mi"),
									},
								},
								SecurityContext: &corev1.SecurityContext{
									AllowPrivilegeEscalation: ptr.To(false),
								},
								VolumeMounts: volumeMounts,
							}},
							AutomountServiceAccountToken: automountServiceAccountToken,
							PriorityClassName:            priorityClassName,
							ServiceAccountName:           serviceAccountName,
							Volumes:                      volumes,
						},
					},
				},
			}

			Expect(references.InjectAnnotations(deployment)).To(Succeed())
			return deployment
		}

		scrapeConfigCacheFor = func(nameSuffix string) *monitoringv1alpha1.ScrapeConfig {
			return &monitoringv1alpha1.ScrapeConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cache-kube-state-metrics",
					Namespace: namespace,
					Labels:    map[string]string{"prometheus": "cache"},
				},
				Spec: monitoringv1alpha1.ScrapeConfigSpec{
					KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
						Role:       "Service",
						Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{namespace}},
					}},
					RelabelConfigs: []monitoringv1.RelabelConfig{
						{
							SourceLabels: []monitoringv1.LabelName{
								"__meta_kubernetes_service_label_component",
								"__meta_kubernetes_service_port_name",
							},
							Regex:  "kube-state-metrics" + nameSuffix + ";metrics",
							Action: "keep",
						},
						{
							SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_label_type"},
							Regex:        `(.+)`,
							Replacement:  ptr.To(`${1}`),
							TargetLabel:  "type",
						},
						{
							Action:      "replace",
							Replacement: ptr.To("kube-state-metrics"),
							TargetLabel: "job",
						},
						{
							TargetLabel: "instance",
							Replacement: ptr.To("kube-state-metrics"),
						},
					},
					MetricRelabelConfigs: []monitoringv1.RelabelConfig{
						{
							SourceLabels: []monitoringv1.LabelName{"pod"},
							Regex:        `^.+\.tf-pod.+$`,
							Action:       "drop",
						},
					},
				},
			}
		}
		scrapeConfigSeed = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "seed-kube-state-metrics",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "seed"},
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
					Role:       "Service",
					Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{namespace}},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						SourceLabels: []monitoringv1.LabelName{
							"__meta_kubernetes_service_label_component",
							"__meta_kubernetes_service_port_name",
						},
						Regex:  "kube-state-metrics-seed;metrics",
						Action: "keep",
					},
					{
						Action:      "replace",
						Replacement: ptr.To("kube-state-metrics"),
						TargetLabel: "job",
					},
					{
						TargetLabel: "instance",
						Replacement: ptr.To("kube-state-metrics"),
					},
				},
				MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
					SourceLabels: []monitoringv1.LabelName{"namespace"},
					Regex:        `shoot-.+`,
					Action:       "drop",
				}},
			},
		}
		scrapeConfigGarden = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "garden-kube-state-metrics",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "garden"},
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
					Role:       "Service",
					Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{namespace}},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						SourceLabels: []monitoringv1.LabelName{
							"__meta_kubernetes_service_label_component",
							"__meta_kubernetes_service_port_name",
						},
						Regex:  "kube-state-metrics-runtime;metrics",
						Action: "keep",
					},
					{
						Action:      "replace",
						Replacement: ptr.To("kube-state-metrics"),
						TargetLabel: "job",
					},
					{
						TargetLabel: "instance",
						Replacement: ptr.To("kube-state-metrics"),
					},
				},
				MetricRelabelConfigs: []monitoringv1.RelabelConfig{
					{
						SourceLabels: []monitoringv1.LabelName{"namespace"},
						Regex:        `shoot-.+`,
						Action:       "drop",
					},
					{
						SourceLabels: []monitoringv1.LabelName{"pod"},
						Regex:        `^.+\.tf-pod.+$`,
						Action:       "drop",
					},
				},
			},
		}
		scrapeConfigShoot = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-kube-state-metrics",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "shoot"},
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
					Role:       "Service",
					Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{namespace}},
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{
					{
						SourceLabels: []monitoringv1.LabelName{
							"__meta_kubernetes_service_label_component",
							"__meta_kubernetes_service_port_name",
						},
						Regex:  "kube-state-metrics;metrics",
						Action: "keep",
					},
					{
						SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_label_type"},
						Regex:        `(.+)`,
						Replacement:  ptr.To(`${1}`),
						TargetLabel:  "type",
					},
					{
						Action:      "replace",
						Replacement: ptr.To("kube-state-metrics"),
						TargetLabel: "job",
					},
					{
						TargetLabel: "instance",
						Replacement: ptr.To("kube-state-metrics"),
					},
				},
				MetricRelabelConfigs: []monitoringv1.RelabelConfig{
					{
						SourceLabels: []monitoringv1.LabelName{"pod"},
						Regex:        `^.+\.tf-pod.+$`,
						Action:       "drop",
					},
				},
			},
		}
		prometheusRuleShoot = func() *monitoringv1.PrometheusRule {
			rules := []monitoringv1.Rule{
				{
					Alert: "KubeStateMetricsSeedDown",
					Expr:  intstr.FromString(`absent(count({exported_job="kube-state-metrics"}))`),
					For:   ptr.To(monitoringv1.Duration("15m")),
					Labels: map[string]string{
						"service":    "kube-state-metrics-seed",
						"severity":   "critical",
						"type":       "seed",
						"visibility": "operator",
					},
					Annotations: map[string]string{
						"summary":     "There are no kube-state-metrics metrics for the control plane",
						"description": "Kube-state-metrics is scraped by the cache prometheus and federated by the control plane prometheus. Something is broken in that process.",
					},
				},
				{
					Alert: "KubeStateMetricsShootDown",
					Expr:  intstr.FromString(`absent(up{job="kube-state-metrics", type="shoot"} == 1)`),
					For:   ptr.To(monitoringv1.Duration("15m")),
					Labels: map[string]string{
						"service":    "kube-state-metrics-shoot",
						"severity":   "info",
						"type":       "seed",
						"visibility": "operator",
					},
					Annotations: map[string]string{
						"summary":     "Kube-state-metrics for shoot cluster metrics is down.",
						"description": "There are no running kube-state-metric pods for the shoot cluster. No kubernetes resource metrics can be scraped.",
					},
				},
				{
					Alert: "NoWorkerNodes",
					Expr:  intstr.FromString(`sum(kube_node_spec_unschedulable) == count(kube_node_info) or absent(kube_node_info)`),
					For:   ptr.To(monitoringv1.Duration("25m")),
					Labels: map[string]string{
						"service":    "nodes",
						"severity":   "blocker",
						"visibility": "all",
					},
					Annotations: map[string]string{
						"summary":     "No nodes available. Possibly all workloads down.",
						"description": "There are no worker nodes in the cluster or all of the worker nodes in the cluster are not schedulable.",
					},
				},
				{
					Record: "shoot:kube_node_status_capacity_cpu_cores:sum",
					Expr:   intstr.FromString(`sum(kube_node_status_capacity{resource="cpu",unit="core"})`),
				},
				{
					Record: "shoot:kube_node_status_capacity_memory_bytes:sum",
					Expr:   intstr.FromString(`sum(kube_node_status_capacity{resource="memory",unit="byte"})`),
				},
				{
					Record: "shoot:machine_types:sum",
					Expr:   intstr.FromString(`sum(kube_node_labels) by (label_beta_kubernetes_io_instance_type)`),
				},
				{
					Record: "shoot:node_operating_system:sum",
					Expr:   intstr.FromString(`sum(kube_node_info) by (os_image, kernel_version)`),
				},
			}

			return &monitoringv1.PrometheusRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-kube-state-metrics",
					Namespace: namespace,
					Labels:    map[string]string{"prometheus": "shoot"},
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{{
						Name:  "kube-state-metrics.rules",
						Rules: rules,
					}},
				},
			}
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		ksm = New(c, namespace, sm, values)
		managedResourceName = ""
		managedResourceTargetName = ""

		selectorLabelsForClusterType := func(nameSuffix string) map[string]string {
			return map[string]string{
				"component": "kube-state-metrics" + nameSuffix,
				"type":      string(component.ClusterTypeSeed),
			}
		}

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())

		serviceAccountFor = func(nameSuffix string) *corev1.ServiceAccount {
			return &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-state-metrics" + nameSuffix,
					Namespace: namespace,
					Labels:    selectorLabelsForClusterType(nameSuffix),
				},
				AutomountServiceAccountToken: ptr.To(false),
			}
		}
		secretShootAccess = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-access-kube-state-metrics",
				Namespace: namespace,
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "kube-state-metrics",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
			},
			Type: corev1.SecretTypeOpaque,
		}
		vpaFor = func(nameSuffix string) *vpaautoscalingv1.VerticalPodAutoscaler {
			return &vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-state-metrics-vpa" + nameSuffix,
					Namespace: namespace,
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "kube-state-metrics" + nameSuffix,
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{UpdateMode: &vpaUpdateMode},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
							{
								ContainerName:    "*",
								ControlledValues: &vpaControlledValues,
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("32Mi"),
								},
							},
						},
					},
				},
			}
		}
		maxUnavailable := intstr.FromInt32(1)
		pdbFor = func(nameSuffix string) *policyv1.PodDisruptionBudget {
			return &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-state-metrics-pdb" + nameSuffix,
					Namespace: namespace,
					Labels:    selectorLabelsForClusterType(nameSuffix),
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: selectorLabelsForClusterType(nameSuffix),
					},
					MaxUnavailable:             &maxUnavailable,
					UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
				},
			}
		}
	})

	JustBeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceTarget = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceTargetName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		Context("cluster type garden-runtime", func() {
			var expectedObjects []client.Object

			BeforeEach(func() {
				values = Values{
					NameSuffix: "-runtime",
				}
				ksm = New(c, namespace, nil, Values{
					ClusterType:       component.ClusterTypeSeed,
					Image:             image,
					PriorityClassName: priorityClassName,
					NameSuffix:        "-runtime",
				})
				managedResourceName = "kube-state-metrics-runtime"

				customResourceStateConfigMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-custom-resource-state",
						Namespace: namespace,
					},
					Data: map[string]string{
						"custom-resource-state.yaml": expectedCustomResourceStateConfig(values.NameSuffix),
					},
				}
				Expect(kubernetesutils.MakeUnique(customResourceStateConfigMap)).To(Succeed())
			})

			It("should successfully deploy all resources", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

				Expect(ksm.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceName,
						Namespace: namespace,
						Labels: map[string]string{
							"gardener.cloud/role":                "seed-system-component",
							"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
						},
						ResourceVersion: "1",
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class: ptr.To("seed"),
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResource.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: ptr.To(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(DeepEqual(expectedMr))
				expectedObjects = []client.Object{
					serviceAccountFor("-runtime"),
					clusterRoleFor(component.ClusterTypeSeed, "-runtime"),
					clusterRoleBindingFor(component.ClusterTypeSeed, "-runtime"),
					serviceFor(component.ClusterTypeSeed),
					deploymentFor(component.ClusterTypeSeed),
					pdbFor("-runtime"),
					vpaFor("-runtime"),
					scrapeConfigGarden,
					customResourceStateConfigMap,
				}

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
				Expect(managedResource).To(consistOf(expectedObjects...))
			})
		})

		Context("cluster type seed", func() {
			var expectedObjects []client.Object

			BeforeEach(func() {
				values = Values{
					NameSuffix: "-seed",
				}
				ksm = New(c, namespace, nil, Values{
					ClusterType:       component.ClusterTypeSeed,
					Image:             image,
					PriorityClassName: priorityClassName,
					NameSuffix:        "-seed",
				})
				managedResourceName = "kube-state-metrics-seed"

				customResourceStateConfigMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-custom-resource-state",
						Namespace: namespace,
					},
					Data: map[string]string{
						"custom-resource-state.yaml": expectedCustomResourceStateConfig(values.NameSuffix),
					},
				}
				Expect(kubernetesutils.MakeUnique(customResourceStateConfigMap)).To(Succeed())
			})

			It("should successfully deploy all resources", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

				Expect(ksm.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceName,
						Namespace: namespace,
						Labels: map[string]string{
							"gardener.cloud/role":                "seed-system-component",
							"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
						},
						ResourceVersion: "1",
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class: ptr.To("seed"),
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResource.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: ptr.To(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(DeepEqual(expectedMr))
				expectedObjects = []client.Object{
					serviceAccountFor("-seed"),
					clusterRoleFor(component.ClusterTypeSeed, "-seed"),
					clusterRoleBindingFor(component.ClusterTypeSeed, "-seed"),
					serviceFor(component.ClusterTypeSeed),
					deploymentFor(component.ClusterTypeSeed),
					pdbFor("-seed"),
					vpaFor("-seed"),
					scrapeConfigCacheFor("-seed"),
					scrapeConfigSeed,
					customResourceStateConfigMap,
				}

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
				Expect(managedResource).To(consistOf(expectedObjects...))

			})
		})

		Context("cluster type shoot", func() {
			BeforeEach(func() {
				values = Values{
					ClusterType:       component.ClusterTypeShoot,
					Image:             image,
					PriorityClassName: priorityClassName,
				}
				managedResourceName = "shoot-core-kube-state-metrics"
				managedResourceTargetName = "shoot-core-kube-state-metrics-target"

				customResourceStateConfigMap = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-state-metrics-custom-resource-state",
						Namespace: namespace,
					},
					Data: map[string]string{
						"custom-resource-state.yaml": expectedCustomResourceStateConfig(values.NameSuffix),
					},
				}
				Expect(kubernetesutils.MakeUnique(customResourceStateConfigMap)).To(Succeed())
			})

			JustBeforeEach(func() {
				ksm = New(c, namespace, sm, values)
			})

			It("should successfully deploy all resources", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceTarget), managedResourceTarget)).To(BeNotFoundError())

				Expect(ksm.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceTarget), managedResourceTarget)).To(Succeed())

				expectedMrTarget := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceTargetName,
						Namespace:       namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"origin":                             "gardener",
							"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResourceTarget.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: ptr.To(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMrTarget))
				Expect(managedResourceTarget).To(DeepEqual(expectedMrTarget))
				Expect(managedResourceTarget).To(consistOf(
					clusterRoleFor(component.ClusterTypeShoot, ""),
					clusterRoleBindingFor(component.ClusterTypeShoot, ""),
				))

				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceName,
						Namespace: namespace,
						Labels: map[string]string{
							"gardener.cloud/role":                "seed-system-component",
							"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
						},
						ResourceVersion: "1",
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class: ptr.To("seed"),
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResource.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: ptr.To(false),
					},
				}

				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(DeepEqual(expectedMr))
				prometheusRule := prometheusRuleShoot()
				Expect(managedResource).To(consistOf(
					deploymentFor(component.ClusterTypeShoot),
					prometheusRule,
					scrapeConfigShoot,
					serviceFor(component.ClusterTypeShoot),
					vpaFor(""),
					customResourceStateConfigMap,
				))

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				actualSecretShootAccess := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(secretShootAccess), actualSecretShootAccess)).To(Succeed())
				expectedSecretShootAccess := secretShootAccess.DeepCopy()
				expectedSecretShootAccess.ResourceVersion = "1"
				Expect(actualSecretShootAccess).To(Equal(expectedSecretShootAccess))

				componenttest.PrometheusRule(prometheusRule, "testdata/shoot-kube-state-metrics.prometheusrule.test.yaml")
			})
		})
	})

	Describe("#Destroy", func() {
		Context("cluster type seed", func() {
			BeforeEach(func() {
				ksm = New(c, namespace, nil, Values{ClusterType: component.ClusterTypeSeed})
				managedResourceName = "kube-state-metrics"
			})

			It("should successfully destroy all resources", func() {
				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(ksm.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			})
		})

		Context("cluster type shoot", func() {
			BeforeEach(func() {
				ksm = New(c, namespace, sm, Values{ClusterType: component.ClusterTypeShoot})
				managedResourceName = "shoot-core-kube-state-metrics"
				managedResourceTargetName = "shoot-core-kube-state-metrics-target"
			})

			It("should successfully destroy all resources", func() {
				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceTarget)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
				Expect(c.Create(ctx, secretShootAccess)).To(Succeed())

				Expect(ksm.Destroy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceTarget), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(secretShootAccess), secretShootAccess)).To(BeNotFoundError())
			})
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
			tests := func(managedResourceName string) {
				It("should fail because reading the ManagedResource fails", func() {
					Expect(ksm.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
				})

				It("should fail because the ManagedResource doesn't become healthy", func() {
					fakeOps.MaxAttempts = 2

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceName,
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

					Expect(ksm.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
				})

				It("should successfully wait for the managed resource to become healthy", func() {
					fakeOps.MaxAttempts = 2

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceName,
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

					Expect(ksm.Wait(ctx)).To(Succeed())
				})
			}

			Context("cluster type seed", func() {
				BeforeEach(func() {
					ksm = New(c, namespace, nil, Values{ClusterType: component.ClusterTypeSeed})
				})

				tests("kube-state-metrics")
			})

			Context("cluster type shoot", func() {
				BeforeEach(func() {
					ksm = New(c, namespace, sm, Values{ClusterType: component.ClusterTypeShoot})
				})

				tests("shoot-core-kube-state-metrics")
			})
		})

		Describe("#WaitCleanup", func() {
			tests := func(managedResourceName string) {
				It("should fail when the wait for the managed resource deletion times out", func() {
					fakeOps.MaxAttempts = 2

					managedResource := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:      managedResourceName,
							Namespace: namespace,
						},
					}

					Expect(c.Create(ctx, managedResource)).To(Succeed())

					Expect(ksm.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
				})

				It("should not return an error when it's already removed", func() {
					Expect(ksm.WaitCleanup(ctx)).To(Succeed())
				})
			}

			Context("cluster type seed", func() {
				BeforeEach(func() {
					ksm = New(c, namespace, nil, Values{ClusterType: component.ClusterTypeSeed})
				})

				tests("kube-state-metrics")
			})

			Context("cluster type shoot", func() {
				BeforeEach(func() {
					ksm = New(c, namespace, sm, Values{ClusterType: component.ClusterTypeShoot})
				})

				tests("shoot-core-kube-state-metrics")
			})
		})
	})
})
