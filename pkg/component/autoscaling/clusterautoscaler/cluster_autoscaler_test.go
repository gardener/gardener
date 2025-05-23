// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("ClusterAutoscaler", func() {
	var (
		ctrl              *gomock.Controller
		c                 *mockclient.MockClient
		fakeClient        client.Client
		sm                secretsmanager.Interface
		clusterAutoscaler Interface

		ctx          = context.Background()
		consistOf    func(...client.Object) gomegatypes.GomegaMatcher
		fakeErr            = fmt.Errorf("fake error")
		namespace          = "shoot--foo--bar"
		namespaceUID       = types.UID("1234567890")
		image              = "registry.k8s.io/cluster-autoscaler:v1.2.3"
		replicas     int32 = 1

		machineDeployment1Name           = "pool1"
		machineDeployment1Min      int32 = 2
		machineDeployment1Max      int32 = 4
		machineDeployment1Priority       = ptr.To(int32(0))
		machineDeployment2Name           = "pool2"
		machineDeployment2Min      int32 = 3
		machineDeployment2Max      int32 = 5
		machineDeployment2Priority       = ptr.To(int32(40))
		machineDeployment3Name           = "pool3"
		machineDeployment3Min      int32 = 3
		machineDeployment3Max      int32 = 5
		machineDeployment4Name           = "pool4"
		machineDeployment4Min      int32 = 3
		machineDeployment4Max      int32 = 5
		workerPool4Priority              = ptr.To(int32(50))
		machineDeployment5Name           = "irregular-machine-deployment-name"
		machineDeployment5Min      int32 = 3
		machineDeployment5Max      int32 = 5
		workerPool5Priority              = ptr.To(int32(60))
		machineDeployments               = []extensionsv1alpha1.MachineDeployment{
			{Name: machineDeployment1Name, Minimum: machineDeployment1Min, Maximum: machineDeployment1Max, Priority: machineDeployment1Priority},
			{Name: machineDeployment2Name, Minimum: machineDeployment2Min, Maximum: machineDeployment2Max, Priority: machineDeployment2Priority},
			{Name: machineDeployment3Name, Minimum: machineDeployment3Min, Maximum: machineDeployment3Max},
			{Name: machineDeployment4Name, Minimum: machineDeployment4Min, Maximum: machineDeployment4Max},
			{Name: machineDeployment5Name, Minimum: machineDeployment5Min, Maximum: machineDeployment5Max},
		}

		workerConfig = []gardencorev1beta1.Worker{
			{
				Name:     machineDeployment1Name,
				Minimum:  machineDeployment1Min,
				Maximum:  machineDeployment1Max,
				Priority: machineDeployment1Priority,
			},
			{
				Name:     machineDeployment2Name,
				Minimum:  machineDeployment2Min,
				Maximum:  machineDeployment2Max,
				Priority: machineDeployment2Priority,
			},
			{
				Name:    machineDeployment3Name,
				Minimum: machineDeployment3Min,
				Maximum: machineDeployment3Max,
			},
			{
				Name:     machineDeployment4Name,
				Minimum:  machineDeployment4Min,
				Maximum:  machineDeployment4Max,
				Priority: workerPool4Priority,
			},
			{
				Name:     "pool-5-that-has-no-matching-machine-deployment-name",
				Minimum:  machineDeployment5Min,
				Maximum:  machineDeployment5Max,
				Priority: workerPool5Priority,
			},
		}

		configExpander                            = gardencorev1beta1.ClusterAutoscalerExpanderRandom
		configMaxGracefulTerminationSeconds int32 = 60 * 60 * 24
		configMaxNodeProvisionTime                = &metav1.Duration{Duration: time.Second}
		configScaleDownDelayAfterAdd              = &metav1.Duration{Duration: time.Second}
		configScaleDownDelayAfterDelete           = &metav1.Duration{Duration: time.Second}
		configScaleDownDelayAfterFailure          = &metav1.Duration{Duration: time.Second}
		configScaleDownUnneededTime               = &metav1.Duration{Duration: time.Second}
		configScaleDownUtilizationThreshold       = ptr.To(float64(1.2345))
		configScanInterval                        = &metav1.Duration{Duration: time.Second}
		configIgnoreDaemonsetsUtilization         = true
		configVerbosity                     int32 = 4
		configMaxEmptyBulkDelete                  = ptr.To[int32](20)
		configNewPodScaleUpDelay                  = &metav1.Duration{Duration: time.Second}
		configTaints                              = []string{"taint-1", "taint-2"}
		configFull                                = &gardencorev1beta1.ClusterAutoscaler{
			Expander:                      &configExpander,
			MaxGracefulTerminationSeconds: &configMaxGracefulTerminationSeconds,
			MaxNodeProvisionTime:          configMaxNodeProvisionTime,
			ScaleDownDelayAfterAdd:        configScaleDownDelayAfterAdd,
			ScaleDownDelayAfterDelete:     configScaleDownDelayAfterDelete,
			ScaleDownDelayAfterFailure:    configScaleDownDelayAfterFailure,
			ScaleDownUnneededTime:         configScaleDownUnneededTime,
			ScaleDownUtilizationThreshold: configScaleDownUtilizationThreshold,
			ScanInterval:                  configScanInterval,
			StartupTaints:                 configTaints,
			StatusTaints:                  configTaints,
			IgnoreTaints:                  configTaints,
			IgnoreDaemonsetsUtilization:   &configIgnoreDaemonsetsUtilization,
			Verbosity:                     &configVerbosity,
			MaxEmptyBulkDelete:            configMaxEmptyBulkDelete,
			NewPodScaleUpDelay:            configNewPodScaleUpDelay,
		}

		genericTokenKubeconfigSecretName = "generic-token-kubeconfig"
		serviceAccountName               = "cluster-autoscaler"
		secretName                       = "shoot-access-cluster-autoscaler"
		clusterRoleBindingName           = "cluster-autoscaler-" + namespace
		vpaName                          = "cluster-autoscaler-vpa"
		pdbName                          = "cluster-autoscaler"
		serviceName                      = "cluster-autoscaler"
		deploymentName                   = "cluster-autoscaler"
		managedResourceName              = "shoot-core-cluster-autoscaler"

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace, ResourceVersion: "1"},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       deploymentName,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
						ContainerName:    vpaautoscalingv1.DefaultContainerResourcePolicy,
						ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					}},
				},
			},
		}
		pdbMaxUnavailable = intstr.FromInt32(1)
		pdb               = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pdbName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "cluster-autoscaler",
				},
				ResourceVersion: "1",
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &pdbMaxUnavailable,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app":  "kubernetes",
						"role": "cluster-autoscaler",
					},
				},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleBindingName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "v1",
					Kind:               "Namespace",
					Name:               namespace,
					UID:                namespaceUID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				}},
				ResourceVersion: "1",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:cluster-autoscaler-seed",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			}},
		}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:            serviceAccountName,
				Namespace:       namespace,
				ResourceVersion: "1",
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "cluster-autoscaler",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8085}]`,
				},
				ResourceVersion: "1",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app":  "kubernetes",
					"role": "cluster-autoscaler",
				},
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports: []corev1.ServicePort{
					{
						Name:     "metrics",
						Protocol: corev1.ProtocolTCP,
						Port:     8085,
					},
				},
			},
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "cluster-autoscaler",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				ResourceVersion: "1",
			},
			Type: corev1.SecretTypeOpaque,
		}
		deploymentFor = func(withConfig, withWorkerPriority bool) *appsv1.Deployment {
			var commandConfigFlags []string

			expander := string(configExpander)
			if withWorkerPriority {
				expander = "priority," + expander
			}

			if !withConfig {
				commandConfigFlags = append(commandConfigFlags,
					"--expander=least-waste",
					"--max-graceful-termination-sec=600",
					"--max-node-provision-time=20m0s",
					"--scale-down-utilization-threshold=0.500000",
					"--scale-down-unneeded-time=30m0s",
					"--scale-down-delay-after-add=1h0m0s",
					"--scale-down-delay-after-delete=0s",
					"--scale-down-delay-after-failure=3m0s",
					"--scan-interval=10s",
					"--ignore-daemonsets-utilization=false",
					"--v=2",
					"--max-empty-bulk-delete=10",
					"--new-pod-scale-up-delay=0s",
					"--max-nodes-total=0",
				)
			} else {
				commandConfigFlags = append(commandConfigFlags,
					"--expander="+expander,
					fmt.Sprintf("--max-graceful-termination-sec=%d", configMaxGracefulTerminationSeconds),
					fmt.Sprintf("--max-node-provision-time=%s", configMaxNodeProvisionTime.Duration),
					fmt.Sprintf("--scale-down-utilization-threshold=%f", *configScaleDownUtilizationThreshold),
					fmt.Sprintf("--scale-down-unneeded-time=%s", configScaleDownUnneededTime.Duration),
					fmt.Sprintf("--scale-down-delay-after-add=%s", configScaleDownDelayAfterAdd.Duration),
					fmt.Sprintf("--scale-down-delay-after-delete=%s", configScaleDownDelayAfterDelete.Duration),
					fmt.Sprintf("--scale-down-delay-after-failure=%s", configScaleDownDelayAfterFailure.Duration),
					fmt.Sprintf("--scan-interval=%s", configScanInterval.Duration),
					fmt.Sprintf("--ignore-daemonsets-utilization=%t", configIgnoreDaemonsetsUtilization),
					fmt.Sprintf("--v=%d", configVerbosity),
					fmt.Sprintf("--max-empty-bulk-delete=%d", *configMaxEmptyBulkDelete),
					fmt.Sprintf("--new-pod-scale-up-delay=%s", configNewPodScaleUpDelay.Duration),
					"--max-nodes-total=0",
					fmt.Sprintf("--startup-taint=%s", configTaints[0]),
					fmt.Sprintf("--startup-taint=%s", configTaints[1]),
					fmt.Sprintf("--status-taint=%s", configTaints[0]),
					fmt.Sprintf("--status-taint=%s", configTaints[1]),
					fmt.Sprintf("--ignore-taint=%s", configTaints[0]),
					fmt.Sprintf("--ignore-taint=%s", configTaints[1]),
				)
			}

			command := append([]string{
				"./cluster-autoscaler",
				"--address=:8085",
				"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
				"--cloud-provider=mcm",
				"--stderrthreshold=info",
				"--skip-nodes-with-system-pods=false",
				"--skip-nodes-with-local-storage=false",
				"--expendable-pods-priority-cutoff=-10",
				"--balance-similar-node-groups=true",
				"--ignore-taint=node.gardener.cloud/critical-components-not-ready",
			}, commandConfigFlags...)
			command = append(command,
				fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment1Min, machineDeployment1Max, namespace, machineDeployment1Name),
				fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment2Min, machineDeployment2Max, namespace, machineDeployment2Name),
				fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment3Min, machineDeployment3Max, namespace, machineDeployment3Name),
				fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment4Min, machineDeployment4Max, namespace, machineDeployment4Name),
				fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment5Min, machineDeployment5Max, namespace, machineDeployment5Name),
			)

			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "kubernetes",
						"role":                "cluster-autoscaler",
						"gardener.cloud/role": "controlplane",
						"high-availability-config.resources.gardener.cloud/type":             "controller",
						"provider.extensions.gardener.cloud/mutated-by-controlplane-webhook": "true",
					},
					ResourceVersion: "1",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas:             &replicas,
					RevisionHistoryLimit: ptr.To[int32](1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "kubernetes",
							"role": "cluster-autoscaler",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":                                "kubernetes",
								"role":                               "cluster-autoscaler",
								"gardener.cloud/role":                "controlplane",
								"maintenance.gardener.cloud/restart": "true",
								"networking.gardener.cloud/to-dns":   "allowed",
								"networking.gardener.cloud/to-runtime-apiserver":                "allowed",
								"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "cluster-autoscaler",
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command:         command,
									Ports: []corev1.ContainerPort{
										{
											Name:          "metrics",
											ContainerPort: 8085,
											Protocol:      corev1.ProtocolTCP,
										},
									},
									Env: []corev1.EnvVar{
										{
											Name:  "CONTROL_NAMESPACE",
											Value: namespace,
										},
										{
											Name:  "TARGET_KUBECONFIG",
											Value: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
										},
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("10m"),
											corev1.ResourceMemory: resource.MustParse("30M"),
										},
									},
									SecurityContext: &corev1.SecurityContext{
										AllowPrivilegeEscalation: ptr.To(false),
									},
								},
							},
							PriorityClassName:             v1beta1constants.PriorityClassNameShootControlPlane300,
							ServiceAccountName:            serviceAccountName,
							TerminationGracePeriodSeconds: ptr.To[int64](5),
						},
					},
				},
			}

			Expect(gardenerutils.InjectGenericKubeconfig(deploy, genericTokenKubeconfigSecretName, secret.Name)).To(Succeed())
			return deploy
		}
		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-cluster-autoscaler",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "cluster-autoscaler.rules",
					Rules: []monitoringv1.Rule{{
						Alert: "ClusterAutoscalerDown",
						Expr:  intstr.FromString(`absent(up{job="cluster-autoscaler"} == 1)`),
						For:   ptr.To(monitoringv1.Duration("15m")),
						Labels: map[string]string{
							"service":  "cluster-autoscaler",
							"severity": "critical",
							"type":     "seed",
						},
						Annotations: map[string]string{
							"summary":     "Cluster autoscaler is down",
							"description": "There is no running cluster autoscaler. Shoot's Nodes wont be scaled dynamically, based on the load.",
						},
					}},
				}},
			},
		}
		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "shoot-cluster-autoscaler",
				Namespace:       namespace,
				Labels:          map[string]string{"prometheus": "shoot"},
				ResourceVersion: "1",
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "kubernetes",
					"role": "cluster-autoscaler",
				}},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "metrics",
					RelabelConfigs: []monitoringv1.RelabelConfig{{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					}},
					MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
						SourceLabels: []monitoringv1.LabelName{"__name__"},
						Action:       "keep",
						Regex:        `^(process_max_fds|process_open_fds|cluster_autoscaler_cluster_safe_to_autoscale|cluster_autoscaler_nodes_count|cluster_autoscaler_unschedulable_pods_count|cluster_autoscaler_node_groups_count|cluster_autoscaler_max_nodes_count|cluster_autoscaler_cluster_cpu_current_cores|cluster_autoscaler_cpu_limits_cores|cluster_autoscaler_cluster_memory_current_bytes|cluster_autoscaler_memory_limits_bytes|cluster_autoscaler_last_activity|cluster_autoscaler_function_duration_seconds|cluster_autoscaler_errors_total|cluster_autoscaler_scaled_up_nodes_total|cluster_autoscaler_scaled_down_nodes_total|cluster_autoscaler_scaled_up_gpu_nodes_total|cluster_autoscaler_scaled_down_gpu_nodes_total|cluster_autoscaler_failed_scale_ups_total|cluster_autoscaler_evicted_pods_total|cluster_autoscaler_unneeded_nodes_count|cluster_autoscaler_old_unregistered_nodes_removed_count|cluster_autoscaler_skipped_scale_events_count)$`,
					}},
				}},
			},
		}

		clusterRoleShoot = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:cluster-autoscaler",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"events", "endpoints"},
					Verbs:     []string{"create", "patch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods/eviction"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods/status"},
					Verbs:     []string{"update"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"endpoints"},
					ResourceNames: []string{"cluster-autoscaler"},
					Verbs:         []string{"get", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"watch", "list", "get", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces", "pods", "services", "replicationcontrollers", "persistentvolumeclaims", "persistentvolumes"},
					Verbs:     []string{"watch", "list", "get"},
				},
				{
					APIGroups: []string{"apps", "extensions"},
					Resources: []string{"daemonsets", "replicasets", "statefulsets"},
					Verbs:     []string{"watch", "list", "get"},
				},
				{
					APIGroups: []string{"policy"},
					Resources: []string{"poddisruptionbudgets"},
					Verbs:     []string{"watch", "list"},
				},
				{
					APIGroups: []string{"storage.k8s.io"},
					Resources: []string{"storageclasses", "csinodes", "csidrivers", "csistoragecapacities"},
					Verbs:     []string{"watch", "list", "get"},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{"coordination.k8s.io"},
					ResourceNames: []string{"cluster-autoscaler"},
					Resources:     []string{"leases"},
					Verbs:         []string{"get", "update"},
				},
				{
					APIGroups: []string{"batch", "extensions"},
					Resources: []string{"jobs"},
					Verbs:     []string{"get", "list", "patch", "watch"},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{"jobs", "cronjobs"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}

		clusterRoleBindingShoot = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:cluster-autoscaler",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:target:cluster-autoscaler",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      serviceAccountName,
					Namespace: "kube-system",
				},
			},
		}

		roleShoot = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:target:cluster-autoscaler",
				Namespace: "kube-system",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"watch", "list", "get", "create"},
				},
				{
					APIGroups:     []string{""},
					ResourceNames: []string{"cluster-autoscaler-status"},
					Resources:     []string{"configmaps"},
					Verbs:         []string{"delete", "update"},
				},
			},
		}

		roleBindingShoot = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:target:cluster-autoscaler",
				Namespace: "kube-system",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "gardener.cloud:target:cluster-autoscaler",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: serviceAccountName,
				},
			},
		}

		priorityExpanderConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-autoscaler-priority-expander",
				Namespace: metav1.NamespaceSystem,
			},
			Data: map[string]string{
				"priorities": "0:\n- pool1\n- pool3\n- irregular-machine-deployment-name\n40:\n- pool2\n50:\n- pool4\n",
			},
		}

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:            managedResourceName,
				Namespace:       namespace,
				Labels:          map[string]string{"origin": "gardener"},
				ResourceVersion: "1",
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs:   []corev1.LocalObjectReference{},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  ptr.To(false),
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())

		clusterAutoscaler = New(c, namespace, sm, image, replicas, nil, workerConfig, nil)
		clusterAutoscaler.SetNamespaceUID(namespaceUID)
		clusterAutoscaler.SetMachineDeployments(machineDeployments)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("should successfully deploy all the resources", func() {
			test := func(withConfig bool, withWorkerConfig bool, withPriorityExpander bool) {
				var config *gardencorev1beta1.ClusterAutoscaler
				var shootWorkerConfig []gardencorev1beta1.Worker
				if withConfig {
					// Copy `configFull` so that te test doesn't overwrite it.
					config = configFull.DeepCopy()
					if withPriorityExpander {
						config.Expander = ptr.To(gardencorev1beta1.ClusterAutoscalerExpanderPriority + "," + configExpander)
					}
				}

				if withWorkerConfig {
					shootWorkerConfig = workerConfig
				}

				clusterAutoscaler = New(fakeClient, namespace, sm, image, replicas, config, shootWorkerConfig, semver.MustParse("1.28.1"))
				clusterAutoscaler.SetNamespaceUID(namespaceUID)
				clusterAutoscaler.SetMachineDeployments(machineDeployments)

				Expect(clusterAutoscaler.Deploy(ctx)).To(Succeed())

				actualMr := &resourcesv1alpha1.ManagedResource{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), actualMr)).To(Succeed())
				managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: actualMr.Spec.SecretRefs[0].Name}}

				utilruntime.Must(references.InjectAnnotations(managedResource))
				Expect(actualMr).To(DeepEqual(managedResource))

				if withWorkerConfig {
					Expect(managedResource).To(consistOf(clusterRoleShoot, clusterRoleBindingShoot, roleShoot, roleBindingShoot, priorityExpanderConfigMap))
				} else {
					Expect(managedResource).To(consistOf(clusterRoleShoot, clusterRoleBindingShoot, roleShoot, roleBindingShoot))
				}

				actualServiceAccount := &corev1.ServiceAccount{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), actualServiceAccount)).To(Succeed())
				Expect(actualServiceAccount).To(DeepEqual(serviceAccount))

				actualClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(clusterRoleBinding), actualClusterRoleBinding)).To(Succeed())
				Expect(actualClusterRoleBinding).To(DeepEqual(clusterRoleBinding))

				actualService := &corev1.Service{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(service), actualService)).To(Succeed())
				Expect(actualService).To(DeepEqual(service))

				actualSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), actualSecret)).To(Succeed())
				Expect(actualSecret).To(DeepEqual(secret))

				actualDeployment := &appsv1.Deployment{}
				deploy := deploymentFor(withConfig, withWorkerConfig)
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), actualDeployment)).To(Succeed())
				Expect(actualDeployment).To(DeepEqual(deploy))

				actualPDB := &policyv1.PodDisruptionBudget{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(pdb), actualPDB)).To(Succeed())
				Expect(actualPDB).To(DeepEqual(pdb))

				actualVPA := &vpaautoscalingv1.VerticalPodAutoscaler{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(vpa), actualVPA)).To(Succeed())
				Expect(actualVPA).To(DeepEqual(vpa))

				actualPrometheusRule := &monitoringv1.PrometheusRule{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(prometheusRule), actualPrometheusRule)).To(Succeed())
				Expect(actualPrometheusRule).To(DeepEqual(prometheusRule))

				actualServiceMonitor := &monitoringv1.ServiceMonitor{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serviceMonitor), actualServiceMonitor)).To(Succeed())
				Expect(actualServiceMonitor).To(DeepEqual(serviceMonitor))
			}

			It("w/o config", func() { test(false, false, false) })
			It("w/ config", func() { test(true, false, false) })
			It("w/ config, w/ workerConfig", func() { test(true, true, false) })
			It("w/ config, w/ workerConfig, w/ 'priority' expander already configured", func() { test(true, true, true) })
		})
	})

	Describe("#Destroy", func() {
		It("should fail because the managed resource cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the vpa cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the pod disruption budget cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the deployment cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the cluster role binding cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the service cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the service account cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}),
				c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceAccountName}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the prometheus rule cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}),
				c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceAccountName}}),
				c.EXPECT().Delete(ctx, &monitoringv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "shoot-cluster-autoscaler", Labels: map[string]string{"prometheus": "shoot"}}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the service monitor cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}),
				c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceAccountName}}),
				c.EXPECT().Delete(ctx, &monitoringv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "shoot-cluster-autoscaler", Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "shoot-cluster-autoscaler", Labels: map[string]string{"prometheus": "shoot"}}}).Return(fakeErr),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully delete all the resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceName}}),
				c.EXPECT().Delete(ctx, &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: vpaName}}),
				c.EXPECT().Delete(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: pdbName}}),
				c.EXPECT().Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: deploymentName}}),
				c.EXPECT().Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName}}),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}}),
				c.EXPECT().Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceName}}),
				c.EXPECT().Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: serviceAccountName}}),
				c.EXPECT().Delete(ctx, &monitoringv1.PrometheusRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "shoot-cluster-autoscaler", Labels: map[string]string{"prometheus": "shoot"}}}),
				c.EXPECT().Delete(ctx, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "shoot-cluster-autoscaler", Labels: map[string]string{"prometheus": "shoot"}}}),
			)

			Expect(clusterAutoscaler.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(clusterAutoscaler.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(clusterAutoscaler.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
