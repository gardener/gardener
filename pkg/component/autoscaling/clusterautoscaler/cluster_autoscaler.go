// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"context"
	"fmt"
	"strings"

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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	serviceName               = "cluster-autoscaler"
	managedResourceTargetName = "shoot-core-cluster-autoscaler"
	containerName             = v1beta1constants.DeploymentNameClusterAutoscaler

	portNameMetrics       = "metrics"
	portMetrics     int32 = 8085
)

// Interface contains functions for a cluster-autoscaler deployer.
type Interface interface {
	component.DeployWaiter
	// SetNamespaceUID sets the UID of the namespace into which the cluster-autoscaler shall be deployed.
	SetNamespaceUID(types.UID)
	// SetMachineDeployments sets the machine deployments.
	SetMachineDeployments([]extensionsv1alpha1.MachineDeployment)
	// SetMaxNodesTotal sets the maximum number of nodes that can be created in the cluster.
	SetMaxNodesTotal(int64)
}

// New creates a new instance of DeployWaiter for the cluster-autoscaler.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	image string,
	replicas int32,
	config *gardencorev1beta1.ClusterAutoscaler,
	workerConfig []gardencorev1beta1.Worker,
	maxNodesTotal int64,
	runtimeVersion *semver.Version,
) Interface {
	return &clusterAutoscaler{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		image:          image,
		replicas:       replicas,
		config:         config,
		workerConfig:   workerConfig,
		maxNodesTotal:  maxNodesTotal,
		runtimeVersion: runtimeVersion,
	}
}

type clusterAutoscaler struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	image          string
	replicas       int32
	config         *gardencorev1beta1.ClusterAutoscaler
	workerConfig   []gardencorev1beta1.Worker
	maxNodesTotal  int64
	runtimeVersion *semver.Version

	namespaceUID       types.UID
	machineDeployments []extensionsv1alpha1.MachineDeployment
}

func (c *clusterAutoscaler) Deploy(ctx context.Context) error {
	var (
		shootAccessSecret             = c.newShootAccessSecret()
		serviceAccount                = c.emptyServiceAccount()
		clusterRoleBinding            = c.emptyClusterRoleBinding()
		vpa                           = c.emptyVPA()
		service                       = c.emptyService()
		deployment                    = c.emptyDeployment()
		podDisruptionBudget           = c.emptyPodDisruptionBudget()
		serviceMonitor                = c.emptyServiceMonitor()
		prometheusRule                = c.emptyPrometheusRule()
		workersHavePriorityConfigured = c.workersHavePriorityConfigured()
	)

	genericTokenKubeconfigSecret, found := c.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c.client, serviceAccount, func() error {
		serviceAccount.AutomountServiceAccountToken = ptr.To(false)
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c.client, clusterRoleBinding, func() error {
		clusterRoleBinding.OwnerReferences = []metav1.OwnerReference{{
			APIVersion:         "v1",
			Kind:               "Namespace",
			Name:               c.namespace,
			UID:                c.namespaceUID,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		}}
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleControlName,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccount.Name,
			Namespace: c.namespace,
		}}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, service, func() error {
		service.Labels = getLabels()

		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, networkingv1.NetworkPolicyPort{
			Port:     ptr.To(intstr.FromInt32(portMetrics)),
			Protocol: ptr.To(corev1.ProtocolTCP),
		}))

		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
		desiredPorts := []corev1.ServicePort{
			{
				Name:     portNameMetrics,
				Protocol: corev1.ProtocolTCP,
				Port:     portMetrics,
			},
		}
		service.Spec.Ports = kubernetesutils.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
		return nil
	}); err != nil {
		return err
	}

	if err := shootAccessSecret.Reconcile(ctx, c.client); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(getLabels(), map[string]string{
			v1beta1constants.GardenRole:                                         v1beta1constants.GardenRoleControlPlane,
			resourcesv1alpha1.HighAvailabilityConfigType:                        resourcesv1alpha1.HighAvailabilityConfigTypeController,
			v1beta1constants.LabelExtensionProviderMutatedByControlplaneWebhook: "true",
		})
		deployment.Spec.Replicas = &c.replicas
		deployment.Spec.RevisionHistoryLimit = ptr.To[int32](1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                           v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelPodMaintenanceRestart:           "true",
					v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
					gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            containerName,
						Image:           c.image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         c.computeCommand(workersHavePriorityConfigured),
						Ports: []corev1.ContainerPort{
							{
								Name:          portNameMetrics,
								ContainerPort: portMetrics,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						Env: []corev1.EnvVar{
							{
								Name:  "CONTROL_NAMESPACE",
								Value: c.namespace,
							},
							{
								Name:  "TARGET_KUBECONFIG",
								Value: gardenerutils.PathGenericKubeconfig,
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("5m"),
								corev1.ResourceMemory: resource.MustParse("30M"),
							},
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
						},
					},
				},
				PriorityClassName:             v1beta1constants.PriorityClassNameShootControlPlane300,
				ServiceAccountName:            serviceAccount.Name,
				TerminationGracePeriodSeconds: ptr.To[int64](5),
			},
		}

		utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecret.Name, shootAccessSecret.Secret.Name))
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, podDisruptionBudget, func() error {
		podDisruptionBudget.Labels = getLabels()
		podDisruptionBudget.Spec = policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: ptr.To(intstr.FromInt32(1)),
			Selector:       deployment.Spec.Selector,
		}
		kubernetesutils.SetAlwaysAllowEviction(podDisruptionBudget, c.runtimeVersion)

		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       v1beta1constants.DeploymentNameClusterAutoscaler,
		}
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
			UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
		}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
				ContainerName:    vpaautoscalingv1.DefaultContainerResourcePolicy,
				ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, prometheusRule, func() error {
		metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", shoot.Label)
		prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "cluster-autoscaler.rules",
				Rules: []monitoringv1.Rule{{
					Alert: "ClusterAutoscalerDown",
					Expr:  intstr.FromString(`absent(up{job="cluster-autoscaler"} == 1)`),
					For:   ptr.To(monitoringv1.Duration("15m")),
					Labels: map[string]string{
						"service":  v1beta1constants.DeploymentNameClusterAutoscaler,
						"severity": "critical",
						"type":     "seed",
					},
					Annotations: map[string]string{
						"summary":     "Cluster autoscaler is down",
						"description": "There is no running cluster autoscaler. Shoot's Nodes wont be scaled dynamically, based on the load.",
					},
				}},
			}},
		}

		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c.client, serviceMonitor, func() error {
		metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", shoot.Label)
		serviceMonitor.Spec = monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				Port: portNameMetrics,
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_service_label_(.+)`,
				}},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"process_max_fds",
					"process_open_fds",
					"cluster_autoscaler_cluster_safe_to_autoscale",
					"cluster_autoscaler_nodes_count",
					"cluster_autoscaler_unschedulable_pods_count",
					"cluster_autoscaler_node_groups_count",
					"cluster_autoscaler_max_nodes_count",
					"cluster_autoscaler_cluster_cpu_current_cores",
					"cluster_autoscaler_cpu_limits_cores",
					"cluster_autoscaler_cluster_memory_current_bytes",
					"cluster_autoscaler_memory_limits_bytes",
					"cluster_autoscaler_last_activity",
					"cluster_autoscaler_function_duration_seconds",
					"cluster_autoscaler_errors_total",
					"cluster_autoscaler_scaled_up_nodes_total",
					"cluster_autoscaler_scaled_down_nodes_total",
					"cluster_autoscaler_scaled_up_gpu_nodes_total",
					"cluster_autoscaler_scaled_down_gpu_nodes_total",
					"cluster_autoscaler_failed_scale_ups_total",
					"cluster_autoscaler_evicted_pods_total",
					"cluster_autoscaler_unneeded_nodes_count",
					"cluster_autoscaler_old_unregistered_nodes_removed_count",
					"cluster_autoscaler_skipped_scale_events_count",
				),
			}},
		}

		return nil
	}); err != nil {
		return err
	}

	data, err := c.computeShootResourcesData(shootAccessSecret.ServiceAccountName, workersHavePriorityConfigured)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, c.client, c.namespace, managedResourceTargetName, managedresources.LabelValueGardener, false, data)
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.DeploymentNameClusterAutoscaler,
	}
}

func (c *clusterAutoscaler) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(
		ctx,
		c.client,
		c.emptyManagedResource(),
		c.emptyVPA(),
		c.emptyPodDisruptionBudget(),
		c.emptyDeployment(),
		c.emptyClusterRoleBinding(),
		c.newShootAccessSecret().Secret,
		c.emptyService(),
		c.emptyServiceAccount(),
		c.emptyPrometheusRule(),
		c.emptyServiceMonitor(),
	)
}

func (c *clusterAutoscaler) Wait(_ context.Context) error        { return nil }
func (c *clusterAutoscaler) WaitCleanup(_ context.Context) error { return nil }
func (c *clusterAutoscaler) SetNamespaceUID(uid types.UID)       { c.namespaceUID = uid }
func (c *clusterAutoscaler) SetMachineDeployments(machineDeployments []extensionsv1alpha1.MachineDeployment) {
	c.machineDeployments = machineDeployments
}

func (c *clusterAutoscaler) SetMaxNodesTotal(maxNodesTotal int64) {
	c.maxNodesTotal = maxNodesTotal
}

func (c *clusterAutoscaler) emptyClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler-" + c.namespace}}
}

func (c *clusterAutoscaler) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler", Namespace: c.namespace}}
}

func (c *clusterAutoscaler) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "cluster-autoscaler-vpa", Namespace: c.namespace}}
}

func (c *clusterAutoscaler) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: c.namespace}}
}

func (c *clusterAutoscaler) newShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(v1beta1constants.DeploymentNameClusterAutoscaler, c.namespace)
}

func (c *clusterAutoscaler) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameClusterAutoscaler, Namespace: c.namespace}}
}

func (c *clusterAutoscaler) emptyPodDisruptionBudget() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameClusterAutoscaler, Namespace: c.namespace}}
}

func (c *clusterAutoscaler) emptyPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{ObjectMeta: monitoringutils.ConfigObjectMeta(v1beta1constants.DeploymentNameClusterAutoscaler, c.namespace, shoot.Label)}
}

func (c *clusterAutoscaler) emptyServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{ObjectMeta: monitoringutils.ConfigObjectMeta(v1beta1constants.DeploymentNameClusterAutoscaler, c.namespace, shoot.Label)}
}

func (c *clusterAutoscaler) emptyManagedResource() *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceTargetName, Namespace: c.namespace}}
}

func (c *clusterAutoscaler) workersHavePriorityConfigured() bool {
	for _, worker := range c.workerConfig {
		if worker.Priority != nil {
			return true
		}
	}
	return false
}

func (c *clusterAutoscaler) computeCommand(workersHavePriorityConfigured bool) []string {
	var (
		command = []string{
			"./cluster-autoscaler",
			fmt.Sprintf("--address=:%d", portMetrics),
			"--kubeconfig=" + gardenerutils.PathGenericKubeconfig,
			"--cloud-provider=mcm",
			"--stderrthreshold=info",
			"--skip-nodes-with-system-pods=false",
			"--skip-nodes-with-local-storage=false",
			"--expendable-pods-priority-cutoff=-10",
			"--balance-similar-node-groups=true",
			// Ignore our taint for nodes with unready critical components.
			// Otherwise, cluster-autoscaler would continue to scale up worker groups even if new Nodes already joined the
			// cluster (with the taint).
			"--ignore-taint=" + v1beta1constants.TaintNodeCriticalComponentsNotReady,
		}
	)

	if c.config == nil {
		c.config = &gardencorev1beta1.ClusterAutoscaler{}
	}
	gardencorev1beta1.SetDefaults_ClusterAutoscaler(c.config)

	expanderMode := *c.config.Expander
	if workersHavePriorityConfigured {
		expanderMode = ensureExpanderInExpanderConfig(string(gardencorev1beta1.ClusterAutoscalerExpanderPriority), expanderMode)
	}

	command = append(command,
		fmt.Sprintf("--expander=%s", expanderMode),
		fmt.Sprintf("--max-graceful-termination-sec=%d", *c.config.MaxGracefulTerminationSeconds),
		fmt.Sprintf("--max-node-provision-time=%s", c.config.MaxNodeProvisionTime.Duration),
		fmt.Sprintf("--scale-down-utilization-threshold=%f", *c.config.ScaleDownUtilizationThreshold),
		fmt.Sprintf("--scale-down-unneeded-time=%s", c.config.ScaleDownUnneededTime.Duration),
		fmt.Sprintf("--scale-down-delay-after-add=%s", c.config.ScaleDownDelayAfterAdd.Duration),
		fmt.Sprintf("--scale-down-delay-after-delete=%s", c.config.ScaleDownDelayAfterDelete.Duration),
		fmt.Sprintf("--scale-down-delay-after-failure=%s", c.config.ScaleDownDelayAfterFailure.Duration),
		fmt.Sprintf("--scan-interval=%s", c.config.ScanInterval.Duration),
		fmt.Sprintf("--ignore-daemonsets-utilization=%t", *c.config.IgnoreDaemonsetsUtilization),
		fmt.Sprintf("--v=%d", *c.config.Verbosity),
		fmt.Sprintf("--max-empty-bulk-delete=%d", *c.config.MaxEmptyBulkDelete),
		fmt.Sprintf("--new-pod-scale-up-delay=%s", c.config.NewPodScaleUpDelay.Duration),
		fmt.Sprintf("--max-nodes-total=%d", c.maxNodesTotal),
	)

	for _, taint := range c.config.StartupTaints {
		command = append(command, "--startup-taint="+taint)
	}

	for _, taint := range c.config.StatusTaints {
		command = append(command, "--status-taint="+taint)
	}

	for _, taint := range c.config.IgnoreTaints {
		command = append(command, "--ignore-taint="+taint)
	}

	for _, machineDeployment := range c.machineDeployments {
		command = append(command, fmt.Sprintf("--nodes=%d:%d:%s.%s", machineDeployment.Minimum, machineDeployment.Maximum, c.namespace, machineDeployment.Name))
	}

	return command
}

func (c *clusterAutoscaler) computeShootResourcesData(serviceAccountName string, workersHavePrioritiesConfigured bool) (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		clusterRole = &rbacv1.ClusterRole{
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
					ResourceNames: []string{serviceName},
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
					Resources:     []string{"leases"},
					ResourceNames: []string{"cluster-autoscaler"},
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

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:cluster-autoscaler",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}

		role = &rbacv1.Role{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:target:cluster-autoscaler",
				Namespace: metav1.NamespaceSystem,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"watch", "list", "get", "create"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					ResourceNames: []string{"cluster-autoscaler-status"},
					Verbs:         []string{"delete", "update"},
				},
			},
		}

		rolebinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:target:cluster-autoscaler",
				Namespace: metav1.NamespaceSystem,
			},
			Subjects: []rbacv1.Subject{{
				Kind: rbacv1.ServiceAccountKind,
				Name: serviceAccountName,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     role.Name,
			},
		}
	)
	objects := []client.Object{clusterRole, clusterRoleBinding, role, rolebinding}

	if workersHavePrioritiesConfigured {
		configMap, err := c.generatePriorityExpanderConfigMap()
		if err != nil {
			return nil, err
		}
		objects = append(objects, configMap)
	}
	return registry.AddAllAndSerialize(objects...)
}

type poolPriorityDefaults struct {
	namespace string
	poolMap   map[string]int32
}

func buildPoolPriorityDefaultsMap(workerConfig []gardencorev1beta1.Worker, namespace string) *poolPriorityDefaults {
	fallbackMap := &poolPriorityDefaults{
		poolMap:   make(map[string]int32, len(workerConfig)),
		namespace: namespace,
	}
	for _, pool := range workerConfig {
		fallbackMap.poolMap[pool.Name] = ptr.Deref(pool.Priority, 0)
	}
	return fallbackMap
}

func (p *poolPriorityDefaults) forDeployment(machineDeploymentName string) int32 {
	name := strings.TrimPrefix(machineDeploymentName, p.namespace+"-")
	zoneIndex := strings.LastIndex(name, "-z")
	if zoneIndex != -1 {
		name = name[:zoneIndex]
	}
	return p.poolMap[name]
}

func (c *clusterAutoscaler) generatePriorityExpanderConfigMap() (*corev1.ConfigMap, error) {
	priorities := map[int32][]string{}
	priorityDefaults := buildPoolPriorityDefaultsMap(c.workerConfig, c.namespace)

	for _, machineDeployment := range c.machineDeployments {
		// TODO(tobschli): Remove this once all well-known extensions have revendored to use the generic actuator that sets the priorities.
		// In the case the priority is nil, the extension did not set the priorities that were configured in the worker.
		// Fall back to try to determine the pool name.
		priority := ptr.Deref(machineDeployment.Priority, priorityDefaults.forDeployment(machineDeployment.Name))
		priorities[priority] = append(priorities[priority], fmt.Sprintf("%s\\.%s", c.namespace, machineDeployment.Name))
	}

	priorityConfig, err := yaml.Marshal(priorities)
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-autoscaler-priority-expander",
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{
			"priorities": string(priorityConfig),
		},
	}

	return configMap, nil
}

func ensureExpanderInExpanderConfig(expander string, expanderConfig gardencorev1beta1.ExpanderMode) gardencorev1beta1.ExpanderMode {
	if strings.Contains(string(expanderConfig), expander) {
		return expanderConfig
	}
	return gardencorev1beta1.ExpanderMode(fmt.Sprintf("%s,%s", expander, string(expanderConfig)))
}
