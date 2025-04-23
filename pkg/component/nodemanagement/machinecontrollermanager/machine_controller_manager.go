// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager

import (
	"context"
	"fmt"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	coordinationv1 "k8s.io/api/coordination/v1"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	portMetrics               = 10258
	portNameMetrics           = "metrics"
	containerName             = "machine-controller-manager"
	serviceName               = "machine-controller-manager"
	managedResourceTargetName = "shoot-core-machine-controller-manager"
	// VPAName is the name of the vertical pod autoscaler for the machine-controller-manager.
	VPAName = "machine-controller-manager-vpa"
)

// Interface contains functions for a machine-controller-manager deployer.
type Interface interface {
	component.DeployWaiter
	// SetNamespaceUID sets the UID of the namespace into which the machine-controller-manager shall be deployed.
	SetNamespaceUID(types.UID)
	// SetReplicas sets the replicas.
	SetReplicas(int32)
}

// New creates a new instance of DeployWaiter for the machine-controller-manager.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	return &machineControllerManager{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type machineControllerManager struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// Values is a set of configuration values for the machine-controller-manager component.
type Values struct {
	// Image is the container image used for machine-controller-manager.
	Image string
	// Replicas is the number of replicas for the deployment.
	Replicas int32

	namespaceUID types.UID
}

func (m *machineControllerManager) Deploy(ctx context.Context) error {
	var (
		shootAccessSecret   = m.newShootAccessSecret()
		serviceAccount      = m.emptyServiceAccount()
		roleBinding         = m.emptyRoleBindingRuntime()
		role                = m.emptyRole()
		service             = m.emptyService()
		deployment          = m.emptyDeployment()
		podDisruptionBudget = m.emptyPodDisruptionBudget()
		vpa                 = m.emptyVPA()
		prometheusRule      = m.emptyPrometheusRule()
		serviceMonitor      = m.emptyServiceMonitor()
	)

	genericTokenKubeconfigSecret, found := m.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, m.client, serviceAccount, func() error {
		serviceAccount.AutomountServiceAccountToken = ptr.To(false)
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, m.client, role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{machinev1alpha1.GroupName},
				Resources: []string{
					"machineclasses",
					"machineclasses/status",
					"machinedeployments",
					"machinedeployments/status",
					"machines",
					"machines/status",
					"machinesets",
					"machinesets/status",
				},
				Verbs: []string{"create", "get", "list", "patch", "update", "watch", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{corev1.GroupName},
				Resources: []string{"configmaps", "secrets", "endpoints", "events", "pods"},
				Verbs:     []string{"create", "get", "list", "patch", "update", "watch", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{coordinationv1.GroupName},
				Resources: []string{"leases"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups:     []string{coordinationv1.GroupName},
				Resources:     []string{"leases"},
				Verbs:         []string{"get", "watch", "update"},
				ResourceNames: []string{"machine-controller", "machine-controller-manager"},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, m.client, roleBinding, func() error {
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     role.Name,
		}
		roleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccount.Name,
			Namespace: m.namespace,
		}}
		return nil
	}); err != nil {
		return err
	}

	//TODO: @aaronfern Remove this after g/g:v1.119 is released
	if err := kubernetesutils.DeleteObjects(ctx, m.client,
		m.emptyClusterRoleBindingRuntime(),
	); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, service, func() error {
		service.Labels = utils.MergeStringMaps(service.Labels, getLabels())

		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service,
			networkingv1.NetworkPolicyPort{
				Port:     ptr.To(intstr.FromInt32(portMetrics)),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
			networkingv1.NetworkPolicyPort{
				Port:     ptr.To(intstr.FromInt32(portProviderMetrics)),
				Protocol: ptr.To(corev1.ProtocolTCP),
			}),
		)

		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
		desiredPorts := []corev1.ServicePort{
			{
				Name:     portNameMetrics,
				Protocol: corev1.ProtocolTCP,
				Port:     portMetrics,
			},
			{
				Name:     portNameProviderMetrics,
				Protocol: corev1.ProtocolTCP,
				Port:     portProviderMetrics,
			},
		}
		service.Spec.Ports = kubernetesutils.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
		return nil
	}); err != nil {
		return err
	}

	if err := shootAccessSecret.Reconcile(ctx, m.client); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(deployment.Labels, getLabels(), map[string]string{
			v1beta1constants.GardenRole:                                         v1beta1constants.GardenRoleControlPlane,
			resourcesv1alpha1.HighAvailabilityConfigType:                        resourcesv1alpha1.HighAvailabilityConfigTypeController,
			v1beta1constants.LabelExtensionProviderMutatedByControlplaneWebhook: "true",
		})
		deployment.Spec.Replicas = &m.values.Replicas
		deployment.Spec.RevisionHistoryLimit = ptr.To[int32](2)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                                                                                 v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelPodMaintenanceRestart:                                                                 "true",
					v1beta1constants.LabelNetworkPolicyToDNS:                                                                    v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToPublicNetworks:                                                         v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToPrivateNetworks:                                                        v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                                                       v1beta1constants.LabelNetworkPolicyAllowed,
					gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:            containerName,
					Image:           m.values.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"./machine-controller-manager",
						"--control-kubeconfig=inClusterConfig",
						"--machine-safety-overshooting-period=1m",
						"--namespace=" + m.namespace,
						fmt.Sprintf("--port=%d", portMetrics),
						"--safety-up=2",
						"--safety-down=1",
						"--target-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
						"--concurrent-syncs=30",
						"--kube-api-qps=150",
						"--kube-api-burst=200",
						"--v=3",
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path:   "/healthz",
								Port:   intstr.FromInt32(portMetrics),
								Scheme: corev1.URISchemeHTTP,
							},
						},
						FailureThreshold:    3,
						InitialDelaySeconds: 30,
						PeriodSeconds:       10,
						SuccessThreshold:    1,
						TimeoutSeconds:      5,
					},
					Ports: []corev1.ContainerPort{{
						Name:          portNameMetrics,
						ContainerPort: portMetrics,
						Protocol:      corev1.ProtocolTCP,
					}},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("5m"),
							corev1.ResourceMemory: resource.MustParse("20M"),
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

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, podDisruptionBudget, func() error {
		podDisruptionBudget.Labels = utils.MergeStringMaps(podDisruptionBudget.Labels, getLabels())
		podDisruptionBudget.Spec = policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable:             ptr.To(intstr.FromInt32(1)),
			Selector:                   deployment.Spec.Selector,
			UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
		}

		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, vpa, func() error {
		metav1.SetMetaDataLabel(&vpa.ObjectMeta, v1beta1constants.LabelExtensionProviderMutatedByControlplaneWebhook, "true")
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       deployment.Name,
		}
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto)}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
				ContainerName:    containerName,
				ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, prometheusRule, func() error {
		metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", shoot.Label)
		prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "machine-controller-manager.rules",
				Rules: []monitoringv1.Rule{{
					Alert: "MachineControllerManagerDown",
					Expr:  intstr.FromString(`absent(up{job="machine-controller-manager"} == 1)`),
					For:   ptr.To(monitoringv1.Duration("15m")),
					Labels: map[string]string{
						"service":    v1beta1constants.DeploymentNameMachineControllerManager,
						"severity":   "critical",
						"type":       "seed",
						"visibility": "operator",
					},
					Annotations: map[string]string{
						"summary":     "Machine controller manager is down.",
						"description": "There are no running machine controller manager instances. No shoot nodes can be created/maintained.",
					},
				}},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, m.client, serviceMonitor, func() error {
		metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", shoot.Label)
		serviceMonitor.Spec = monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getLabels()},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: portNameMetrics,
					RelabelConfigs: []monitoringv1.RelabelConfig{{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					}},
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
						// Machine Deployment related metrics
						"mcm_machine_deployment_items_total",
						"mcm_machine_deployment_info",
						"mcm_machine_deployment_info_spec_paused",
						"mcm_machine_deployment_info_spec_replicas",
						"mcm_machine_deployment_info_spec_min_ready_seconds",
						"mcm_machine_deployment_info_spec_rolling_update_max_surge",
						"mcm_machine_deployment_info_spec_rolling_update_max_unavailable",
						"mcm_machine_deployment_info_spec_revision_history_limit",
						"mcm_machine_deployment_info_spec_progress_deadline_seconds",
						"mcm_machine_deployment_info_spec_rollback_to_revision",
						"mcm_machine_deployment_status_condition",
						"mcm_machine_deployment_status_available_replicas",
						"mcm_machine_deployment_status_unavailable_replicas",
						"mcm_machine_deployment_status_ready_replicas",
						"mcm_machine_deployment_status_updated_replicas",
						"mcm_machine_deployment_status_collision_count",
						"mcm_machine_deployment_status_replicas",
						"mcm_machine_deployment_failed_machines",
						// Machine Set related metrics
						"mcm_machine_set_items_total",
						"mcm_machine_set_info",
						"mcm_machine_set_failed_machines",
						"mcm_machine_set_info_spec_replicas",
						"mcm_machine_set_info_spec_min_ready_seconds",
						"mcm_machine_set_status_condition",
						"mcm_machine_set_status_available_replicas",
						"mcm_machine_set_status_fully_labelled_replicas",
						"mcm_machine_set_status_replicas",
						"mcm_machine_set_status_ready_replicas",
						// Machine related metrics
						"mcm_machine_stale_machines_total",
						// misc metrics
						"mcm_misc_scrape_failure_total",
						"process_max_fds",
						"process_open_fds",
						// workqueue related metrics
						"mcm_workqueue_adds_total",
						"mcm_workqueue_depth",
						"mcm_workqueue_queue_duration_seconds_bucket",
						"mcm_workqueue_queue_duration_seconds_sum",
						"mcm_workqueue_queue_duration_seconds_count",
						"mcm_workqueue_work_duration_seconds_bucket",
						"mcm_workqueue_work_duration_seconds_sum",
						"mcm_workqueue_work_duration_seconds_count",
						"mcm_workqueue_unfinished_work_seconds",
						"mcm_workqueue_longest_running_processor_seconds",
						"mcm_workqueue_retries_total",
					),
				},
				{
					Port: portNameProviderMetrics,
					RelabelConfigs: []monitoringv1.RelabelConfig{{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					}},
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
						// Machine related metrics
						"mcm_machine_items_total",
						"mcm_machine_current_status_phase",
						"mcm_machine_info",
						"mcm_machine_status_condition",
						// Cloud API related metrics
						"mcm_cloud_api_requests_total",
						"mcm_cloud_api_requests_failed_total",
						"mcm_cloud_api_api_request_duration_seconds_bucket",
						"mcm_cloud_api_api_request_duration_seconds_sum",
						"mcm_cloud_api_api_request_duration_seconds_count",
						"mcm_cloud_api_driver_request_duration_seconds_sum",
						"mcm_cloud_api_driver_request_duration_seconds_count",
						"mcm_cloud_api_driver_request_duration_seconds_bucket",
						"mcm_cloud_api_driver_request_failed_total",
						// misc metrics
						"mcm_machine_controller_frozen",
						"process_max_fds",
						"process_open_fds",
						// workqueue related metrics
						"mcm_workqueue_adds_total",
						"mcm_workqueue_depth",
						"mcm_workqueue_queue_duration_seconds_bucket",
						"mcm_workqueue_queue_duration_seconds_sum",
						"mcm_workqueue_queue_duration_seconds_count",
						"mcm_workqueue_work_duration_seconds_bucket",
						"mcm_workqueue_work_duration_seconds_sum",
						"mcm_workqueue_work_duration_seconds_count",
						"mcm_workqueue_unfinished_work_seconds",
						"mcm_workqueue_longest_running_processor_seconds",
						"mcm_workqueue_retries_total",
					),
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	data, err := m.computeShootResourcesData(shootAccessSecret.ServiceAccountName)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, m.client, m.namespace, managedResourceTargetName, managedresources.LabelValueGardener, false, data)
}

func (m *machineControllerManager) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(ctx, m.client,
		m.emptyManagedResource(),
		m.emptyServiceMonitor(),
		m.emptyPrometheusRule(),
		m.emptyVPA(),
		m.emptyPodDisruptionBudget(),
		m.emptyDeployment(),
		m.newShootAccessSecret().Secret,
		m.emptyService(),
		m.emptyClusterRoleBindingRuntime(), //TODO: @aaronfern Remove this after g/g:v1.119 is released
		m.emptyRoleBindingRuntime(),
		m.emptyRole(),
		m.emptyServiceAccount(),
	)
}

var (
	// DefaultInterval is the default interval.
	DefaultInterval = 5 * time.Second
	// DefaultTimeout is the default timeout.
	DefaultTimeout = 5 * time.Minute
)

func (m *machineControllerManager) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	deployment := m.emptyDeployment()
	return retry.Until(timeoutCtx, DefaultInterval, health.IsDeploymentUpdated(m.client, deployment))
}

func (m *machineControllerManager) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	return retry.Until(timeoutCtx, DefaultInterval, func(ctx context.Context) (done bool, err error) {
		deploy := m.emptyDeployment()
		err = m.client.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)

		switch {
		case apierrors.IsNotFound(err):
			return retry.Ok()
		case err == nil:
			return retry.MinorError(err)
		default:
			return retry.SevereError(err)
		}
	})
}

func (m *machineControllerManager) SetNamespaceUID(uid types.UID) { m.values.namespaceUID = uid }
func (m *machineControllerManager) SetReplicas(replicas int32)    { m.values.Replicas = replicas }

func (m *machineControllerManager) computeShootResourcesData(serviceAccountName string) (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:machine-controller-manager",
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"nodes", "nodes/status", "endpoints", "replicationcontrollers", "pods", "persistentvolumes", "persistentvolumeclaims"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods/eviction"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"replicasets", "statefulsets", "daemonsets", "deployments"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{"jobs", "cronjobs"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"policy"},
					Resources: []string{"poddisruptionbudgets"},
					Verbs:     []string{"list", "watch"},
				},
				{
					APIGroups: []string{"storage.k8s.io"},
					Resources: []string{"volumeattachments"},
					Verbs:     []string{"delete", "get", "list", "watch"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:machine-controller-manager",
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
				Name:      "gardener.cloud:target:machine-controller-manager",
				Namespace: metav1.NamespaceSystem,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"create", "delete", "get", "list"},
				},
			},
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:target:machine-controller-manager",
				Namespace: metav1.NamespaceSystem,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     role.Name,
			},
		}
	)

	return registry.AddAllAndSerialize(
		clusterRole,
		clusterRoleBinding,
		role,
		roleBinding,
	)
}

func (m *machineControllerManager) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyRole() *rbacv1.Role {
	return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyRoleBindingRuntime() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: m.namespace}}
}

// TODO: @aaronfern Remove this after g/g:v1.119 is released
func (m *machineControllerManager) emptyClusterRoleBindingRuntime() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager-" + m.namespace}}
}

func (m *machineControllerManager) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: m.namespace}}
}

func (m *machineControllerManager) newShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(v1beta1constants.DeploymentNameMachineControllerManager, m.namespace)
}

func (m *machineControllerManager) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameMachineControllerManager, Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyPodDisruptionBudget() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameMachineControllerManager, Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: VPAName, Namespace: m.namespace}}
}

func (m *machineControllerManager) emptyPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{ObjectMeta: monitoringutils.ConfigObjectMeta(v1beta1constants.DeploymentNameMachineControllerManager, m.namespace, shoot.Label)}
}

func (m *machineControllerManager) emptyServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{ObjectMeta: monitoringutils.ConfigObjectMeta(v1beta1constants.DeploymentNameMachineControllerManager, m.namespace, shoot.Label)}
}

func (m *machineControllerManager) emptyManagedResource() *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceTargetName, Namespace: m.namespace}}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: "machine-controller-manager",
	}
}
