// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllermanager

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	netutils "github.com/gardener/gardener/pkg/utils/net"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

const (
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName = "shoot-core-kube-controller-manager"

	serviceName      = "kube-controller-manager"
	containerName    = v1beta1constants.DeploymentNameKubeControllerManager
	secretNameServer = "kube-controller-manager-server"
	portNameMetrics  = "metrics"

	volumeNameServer            = "server"
	volumeNameServiceAccountKey = "service-account-key"
	volumeNameCA                = "ca"
	volumeNameCAClient          = "ca-client"
	volumeNameCAKubelet         = "ca-kubelet"

	volumeMountPathCA                = "/srv/kubernetes/ca"
	volumeMountPathCAClient          = "/srv/kubernetes/ca-client"
	volumeMountPathCAKubelet         = "/srv/kubernetes/ca-kubelet"
	volumeMountPathServiceAccountKey = "/srv/kubernetes/service-account-key"
	volumeMountPathServer            = "/var/lib/kube-controller-manager-server"

	nodeMonitorGraceDuration = 2 * time.Minute
	// NodeMonitorGraceDurationK8sGreaterEqual127 is the default node monitoring grace duration used with k8s versions >= 1.27
	NodeMonitorGraceDurationK8sGreaterEqual127 = 40 * time.Second
)

// Interface contains functions for a kube-controller-manager deployer.
type Interface interface {
	component.DeployWaiter
	// SetReplicaCount sets the replica count for the kube-controller-manager.
	SetReplicaCount(replicas int32)
	// SetRuntimeConfig sets the runtime config for the kube-controller-manager.
	SetRuntimeConfig(runtimeConfig map[string]bool)
	// WaitForControllerToBeActive checks whether kube-controller-manager has
	// recently written to the Endpoint object holding the leader information. If yes, it is active.
	WaitForControllerToBeActive(ctx context.Context) error
	// SetShootClient sets the shoot client used to deploy resources into the Shoot API server.
	SetShootClient(c client.Client)
	// SetServiceNetworks sets the service CIDRs of the shoot network.
	SetServiceNetworks([]net.IPNet)
	// SetPodNetworks sets the pod CIDRs of the shoot network.
	SetPodNetworks([]net.IPNet)
}

// New creates a new instance of DeployWaiter for the kube-controller-manager.
func New(
	log logr.Logger,
	seedClient kubernetes.Interface,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	return &kubeControllerManager{
		log:            log,
		seedClient:     seedClient,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type kubeControllerManager struct {
	log            logr.Logger
	seedClient     kubernetes.Interface
	shootClient    client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// Values are the values for the kube-controller-manager deployment.
type Values struct {
	// RuntimeVersion is the Kubernetes version of the runtime cluster.
	RuntimeVersion *semver.Version
	// TargetVersion is the Kubernetes version of the target cluster.
	TargetVersion *semver.Version
	// Image is the image of the kube-controller-manager.
	Image string
	// Replicas is the number of replicas for the kube-controller-manager deployment.
	Replicas int32
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// Config is the configuration of the kube-controller-manager.
	Config *gardencorev1beta1.KubeControllerManagerConfig
	// NamePrefix is the prefix for the resource names.
	NamePrefix string
	// IsScaleDownDisabled - if true, pod requests can be scaled up, but never down
	IsScaleDownDisabled bool
	// IsWorkerless specifies whether the cluster has worker nodes.
	IsWorkerless bool
	// PodNetworks are the pod CIDRs of the target cluster.
	PodNetworks []net.IPNet
	// ServiceNetworks are the service CIDRs of the target cluster.
	ServiceNetworks []net.IPNet
	// ClusterSigningDuration is the value for the `--cluster-signing-duration` flag.
	ClusterSigningDuration *time.Duration
	// ControllerWorkers is used for configuring the workers for controllers.
	ControllerWorkers ControllerWorkers
	// ControllerSyncPeriods is used for configuring the sync periods for controllers.
	ControllerSyncPeriods ControllerSyncPeriods
	// RuntimeConfig contains information about enabled or disabled APIs.
	RuntimeConfig map[string]bool
	// ManagedResourceLabels are labels added to the ManagedResource.
	ManagedResourceLabels map[string]string
}

// ControllerWorkers is used for configuring the workers for controllers.
type ControllerWorkers struct {
	// StatefulSet is the number of workers for the StatefulSet controller.
	StatefulSet *int
	// Deployment is the number of workers for the Deployment controller.
	Deployment *int
	// ReplicaSet is the number of workers for the ReplicaSet controller.
	ReplicaSet *int
	// Endpoint is the number of workers for the Endpoint controller.
	Endpoint *int
	// GarbageCollector is the number of workers for the GarbageCollector controller.
	GarbageCollector *int
	// Namespace is the number of workers for the Namespace controller. Set it to '0' in order to disable the controller
	// (only works when cluster is workerless).
	Namespace *int
	// ResourceQuota is the number of workers for the ResourceQuota controller. Set it to '0' in order to disable the
	// controller (only works when cluster is workerless).
	ResourceQuota *int
	// ServiceEndpoint is the number of workers for the ServiceEndpoint controller.
	ServiceEndpoint *int
	// ServiceAccountToken is the number of workers for the ServiceAccountToken controller. Set it to '0' in order to
	// disable the controller (only works when cluster is workerless).
	ServiceAccountToken *int
}

// ControllerSyncPeriods is used for configuring the sync periods for controllers.
type ControllerSyncPeriods struct {
	// ResourceQuota is the sync period for the ResourceQuota controller.
	ResourceQuota *time.Duration
}

const (
	defaultControllerWorkersDeployment          = 50
	defaultControllerWorkersReplicaSet          = 50
	defaultControllerWorkersStatefulSet         = 15
	defaultControllerWorkersEndpoint            = 15
	defaultControllerWorkersGarbageCollector    = 30
	defaultControllerWorkersServiceEndpoint     = 15
	defaultControllerWorkersNamespace           = 30
	defaultControllerWorkersResourceQuota       = 15
	defaultControllerWorkersServiceAccountToken = 15
)

func (k *kubeControllerManager) Deploy(ctx context.Context) error {
	serverSecret, err := k.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
		Name:                        secretNameServer,
		CommonName:                  k.values.NamePrefix + v1beta1constants.DeploymentNameKubeControllerManager,
		DNSNames:                    kubernetesutils.DNSNamesForService(k.values.NamePrefix+serviceName, k.namespace),
		CertType:                    secrets.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	secretCACluster, found := k.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	secretCAClient, found := k.secretsManager.Get(v1beta1constants.SecretNameCAClient, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAClient)
	}

	var secretCAKubelet *corev1.Secret
	if !k.values.IsWorkerless {
		secretCAKubelet, found = k.secretsManager.Get(v1beta1constants.SecretNameCAKubelet, secretsmanager.Current)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAKubelet)
		}
	}

	genericTokenKubeconfigSecret, found := k.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	serviceAccountKeySecret, found := k.secretsManager.Get(v1beta1constants.SecretNameServiceAccountKey, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameServiceAccountKey)
	}

	var (
		vpa                 = k.emptyVPA()
		service             = k.emptyService()
		shootAccessSecret   = k.newShootAccessSecret()
		deployment          = k.emptyDeployment()
		podDisruptionBudget = k.emptyPodDisruptionBudget()
		serviceMonitor      = k.emptyServiceMonitor()
		prometheusRule      = k.emptyPrometheusRule()

		port           int32 = 10257
		probeURIScheme       = corev1.URISchemeHTTPS
		command              = k.computeCommand(port)
	)

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), service, func() error {
		service.Labels = getLabels()

		networkPolicyPort := networkingv1.NetworkPolicyPort{
			Port:     ptr.To(intstr.FromInt32(port)),
			Protocol: ptr.To(corev1.ProtocolTCP),
		}

		if k.values.NamePrefix != "" {
			// controller-manager deployed for garden cluster
			utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForGardenScrapeTargets(service, networkPolicyPort))
		} else {
			// controller-manager deployed for shoot cluster
			utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, networkPolicyPort))
		}

		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.ClusterIP = corev1.ClusterIPNone
		desiredPorts := []corev1.ServicePort{
			{
				Name:     portNameMetrics,
				Protocol: corev1.ProtocolTCP,
				Port:     port,
			},
		}
		service.Spec.Ports = kubernetesutils.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)
		return nil
	}); err != nil {
		return err
	}

	if err := shootAccessSecret.Reconcile(ctx, k.seedClient.Client()); err != nil {
		return err
	}

	if err := netutils.CheckDualStackForKubeComponents(k.values.PodNetworks, "pod"); err != nil {
		return err
	}
	if err := netutils.CheckDualStackForKubeComponents(k.values.ServiceNetworks, "service"); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(getLabels(), map[string]string{
			v1beta1constants.GardenRole:                                         v1beta1constants.GardenRoleControlPlane,
			resourcesv1alpha1.HighAvailabilityConfigType:                        resourcesv1alpha1.HighAvailabilityConfigTypeController,
			v1beta1constants.LabelExtensionProviderMutatedByControlplaneWebhook: "true",
		})
		deployment.Spec.Replicas = &k.values.Replicas
		deployment.Spec.RevisionHistoryLimit = ptr.To[int32](1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                 v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelPodMaintenanceRestart: "true",
					v1beta1constants.LabelNetworkPolicyToDNS:    v1beta1constants.LabelNetworkPolicyAllowed,
					gardenerutils.NetworkPolicyLabel(k.values.NamePrefix+v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				AutomountServiceAccountToken: ptr.To(false),
				PriorityClassName:            k.values.PriorityClassName,
				SecurityContext: &corev1.PodSecurityContext{
					// use the nonroot user from a distroless container
					// https://github.com/GoogleContainerTools/distroless/blob/1a8918fcaa7313fd02ae08089a57a701faea999c/base/base.bzl#L8
					RunAsNonRoot: ptr.To(true),
					RunAsUser:    ptr.To[int64](65532),
					RunAsGroup:   ptr.To[int64](65532),
					FSGroup:      ptr.To[int64](65532),
				},
				Containers: []corev1.Container{
					{
						Name:            containerName,
						Image:           k.values.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         command,
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Scheme: probeURIScheme,
									Port:   intstr.FromInt32(port),
								},
							},
							SuccessThreshold:    1,
							FailureThreshold:    2,
							InitialDelaySeconds: 15,
							PeriodSeconds:       10,
							TimeoutSeconds:      15,
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          portNameMetrics,
								ContainerPort: port,
								Protocol:      corev1.ProtocolTCP,
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
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      volumeNameCA,
								MountPath: volumeMountPathCA,
							},
							{
								Name:      volumeNameCAClient,
								MountPath: volumeMountPathCAClient,
							},
							{
								Name:      volumeNameServiceAccountKey,
								MountPath: volumeMountPathServiceAccountKey,
							},
							{
								Name:      volumeNameServer,
								MountPath: volumeMountPathServer,
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: volumeNameCA,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: secretCACluster.Name,
							},
						},
					},
					{
						Name: volumeNameCAClient,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  secretCAClient.Name,
								DefaultMode: ptr.To[int32](0640),
							},
						},
					},
					{
						Name: volumeNameServiceAccountKey,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  serviceAccountKeySecret.Name,
								DefaultMode: ptr.To[int32](0640),
							},
						},
					},
					{
						Name: volumeNameServer,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  serverSecret.Name,
								DefaultMode: ptr.To[int32](0640),
							},
						},
					},
				},
			},
		}

		if !k.values.IsWorkerless {
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				Name:      volumeNameCAKubelet,
				MountPath: volumeMountPathCAKubelet,
			})

			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: volumeNameCAKubelet,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  secretCAKubelet.Name,
						DefaultMode: ptr.To[int32](0640),
					},
				},
			})
		}

		utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecret.Name, shootAccessSecret.Secret.Name))
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), podDisruptionBudget, func() error {
		podDisruptionBudget.Labels = getLabels()
		podDisruptionBudget.Spec = policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: ptr.To(intstr.FromInt32(1)),
			Selector:       deployment.Spec.Selector,
		}

		kubernetesutils.SetAlwaysAllowEviction(podDisruptionBudget, k.values.RuntimeVersion)

		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       k.values.NamePrefix + v1beta1constants.DeploymentNameKubeControllerManager,
		}
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
			UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
		}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
				ContainerName:    containerName,
				ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
			}},
		}

		if k.values.IsScaleDownDisabled {
			metav1.SetMetaDataLabel(&vpa.ObjectMeta, v1beta1constants.LabelVPAEvictionRequirementsController, v1beta1constants.EvictionRequirementManagedByController)
			metav1.SetMetaDataAnnotation(&vpa.ObjectMeta, v1beta1constants.AnnotationVPAEvictionRequirementDownscaleRestriction, v1beta1constants.EvictionRequirementNever)
		} else {
			delete(vpa.GetLabels(), v1beta1constants.LabelVPAEvictionRequirementsController)
			delete(vpa.GetAnnotations(), v1beta1constants.AnnotationVPAEvictionRequirementDownscaleRestriction)
		}

		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), prometheusRule, func() error {
		labels := map[string]string{
			"service":    v1beta1constants.DeploymentNameKubeControllerManager,
			"severity":   "critical",
			"visibility": "all",
		}

		if k.values.NamePrefix != "" {
			labels["topology"] = "garden"
		} else {
			labels["type"] = "seed"
		}

		metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", k.prometheusLabel())
		prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "kube-controller-manager.rules",
				Rules: []monitoringv1.Rule{{
					Alert:  "KubeControllerManagerDown",
					Expr:   intstr.FromString(`absent(up{job="` + service.Name + `"} == 1)`),
					For:    ptr.To(monitoringv1.Duration("15m")),
					Labels: labels,
					Annotations: map[string]string{
						"summary":     "Kube Controller Manager is down.",
						"description": "Deployments and replication controllers are not making progress.",
					},
				}},
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.seedClient.Client(), serviceMonitor, func() error {
		serviceMonitor.Labels = monitoringutils.Labels(k.prometheusLabel())
		serviceMonitor.Spec = monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				Port:      portNameMetrics,
				Scheme:    "https",
				TLSConfig: &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)}},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: k.prometheusAccessSecretName()},
					Key:                  resourcesv1alpha1.DataKeyToken,
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_service_label_(.+)`,
				}},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"rest_client_requests_total",
					"process_max_fds",
					"process_open_fds",
				),
			}},
		}
		return nil
	}); err != nil {
		return err
	}

	return k.reconcileShootResources(ctx, shootAccessSecret.ServiceAccountName)
}

func (k *kubeControllerManager) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(ctx, k.seedClient.Client(),
		k.emptyManagedResource(),
		k.emptyVPA(),
		k.emptyService(),
		k.emptyPodDisruptionBudget(),
		k.emptyDeployment(),
		k.newShootAccessSecret().Secret,
		k.emptyServiceMonitor(),
		k.emptyPrometheusRule(),
	)
}

func (k *kubeControllerManager) SetShootClient(c client.Client) { k.shootClient = c }
func (k *kubeControllerManager) SetReplicaCount(replicas int32) { k.values.Replicas = replicas }
func (k *kubeControllerManager) SetRuntimeConfig(runtimeConfig map[string]bool) {
	k.values.RuntimeConfig = runtimeConfig
}

func (k *kubeControllerManager) SetPodNetworks(pods []net.IPNet) {
	k.values.PodNetworks = pods
}

func (k *kubeControllerManager) SetServiceNetworks(services []net.IPNet) {
	k.values.ServiceNetworks = services
}

func (k *kubeControllerManager) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + "kube-controller-manager-vpa", Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + serviceName, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + v1beta1constants.DeploymentNameKubeControllerManager, Namespace: k.namespace}}
}

func (k *kubeControllerManager) emptyPodDisruptionBudget() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + v1beta1constants.DeploymentNameKubeControllerManager, Namespace: k.namespace}}
}

func (k *kubeControllerManager) newShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(v1beta1constants.DeploymentNameKubeControllerManager, k.namespace)
}

func (k *kubeControllerManager) emptyManagedResource() *resourcesv1alpha1.ManagedResource {
	return &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: ManagedResourceName, Namespace: k.namespace}}
}

func (k *kubeControllerManager) prometheusAccessSecretName() string {
	if k.values.NamePrefix != "" {
		return garden.AccessSecretName
	}
	return shoot.AccessSecretName
}

func (k *kubeControllerManager) prometheusLabel() string {
	if k.values.NamePrefix != "" {
		return garden.Label
	}
	return shoot.Label
}

func (k *kubeControllerManager) emptyServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{ObjectMeta: monitoringutils.ConfigObjectMeta(k.values.NamePrefix+v1beta1constants.DeploymentNameKubeControllerManager, k.namespace, k.prometheusLabel())}
}

func (k *kubeControllerManager) emptyPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{ObjectMeta: monitoringutils.ConfigObjectMeta(k.values.NamePrefix+v1beta1constants.DeploymentNameKubeControllerManager, k.namespace, k.prometheusLabel())}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelControllerManager,
	}
}

func (k *kubeControllerManager) isDualStack() bool {
	hasIPv4 := false
	hasIPv6 := false

	for _, podNetwork := range k.values.PodNetworks {
		ip := podNetwork.IP

		if ip.To4() != nil {
			hasIPv4 = true
		} else {
			hasIPv6 = true
		}

		if hasIPv4 && hasIPv6 {
			return true
		}
	}
	return false
}

func (k *kubeControllerManager) computeCommand(port int32) []string {
	var (
		defaultHorizontalPodAutoscalerConfig = k.getHorizontalPodAutoscalerConfig()
		podEvictionTimeout                   = metav1.Duration{Duration: 2 * time.Minute}
		nodeMonitorGracePeriod               = metav1.Duration{Duration: nodeMonitorGraceDuration}
		command                              = []string{
			"/usr/local/bin/kube-controller-manager",
			"--authentication-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
			"--authorization-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
			"--kubeconfig=" + gardenerutils.PathGenericKubeconfig,
		}

		controllersToEnable  = sets.New("*", "bootstrapsigner", "tokencleaner")
		controllersToDisable = sets.New[string]()
	)

	if versionutils.ConstraintK8sGreaterEqual127.Check(k.values.TargetVersion) {
		nodeMonitorGracePeriod = metav1.Duration{Duration: NodeMonitorGraceDurationK8sGreaterEqual127}
	}

	if !k.values.IsWorkerless {
		if v := k.values.Config.NodeMonitorGracePeriod; v != nil {
			nodeMonitorGracePeriod = *v
		}

		if k.values.Config.NodeCIDRMaskSize != nil {
			if k.isDualStack() {
				command = append(command, fmt.Sprintf("--node-cidr-mask-size-ipv4=%d", *k.values.Config.NodeCIDRMaskSize))
				command = append(command, fmt.Sprintf("--node-cidr-mask-size-ipv6=%d", 64))
			} else {
				command = append(command, fmt.Sprintf("--node-cidr-mask-size=%d", *k.values.Config.NodeCIDRMaskSize))
			}
		}

		command = append(command,
			"--allocate-node-cidrs=true",
			"--attach-detach-reconcile-sync-period=1m0s",
			"--cluster-cidr="+netutils.JoinByComma(k.values.PodNetworks),
			fmt.Sprintf("--cluster-signing-kubelet-client-cert-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyCertificateCA),
			fmt.Sprintf("--cluster-signing-kubelet-client-key-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyPrivateKeyCA),
			fmt.Sprintf("--cluster-signing-kubelet-serving-cert-file=%s/%s", volumeMountPathCAKubelet, secrets.DataKeyCertificateCA),
			fmt.Sprintf("--cluster-signing-kubelet-serving-key-file=%s/%s", volumeMountPathCAKubelet, secrets.DataKeyPrivateKeyCA),
			"--horizontal-pod-autoscaler-downscale-stabilization="+defaultHorizontalPodAutoscalerConfig.DownscaleStabilization.Duration.String(),
			"--horizontal-pod-autoscaler-initial-readiness-delay="+defaultHorizontalPodAutoscalerConfig.InitialReadinessDelay.Duration.String(),
			"--horizontal-pod-autoscaler-cpu-initialization-period="+defaultHorizontalPodAutoscalerConfig.CPUInitializationPeriod.Duration.String(),
			"--horizontal-pod-autoscaler-sync-period="+defaultHorizontalPodAutoscalerConfig.SyncPeriod.Duration.String(),
			fmt.Sprintf("--horizontal-pod-autoscaler-tolerance=%v", *defaultHorizontalPodAutoscalerConfig.Tolerance),
			"--leader-elect=true",
			fmt.Sprintf("--node-monitor-grace-period=%s", nodeMonitorGracePeriod.Duration),
		)

		if versionutils.ConstraintK8sLess127.Check(k.values.TargetVersion) {
			if v := k.values.Config.PodEvictionTimeout; v != nil {
				podEvictionTimeout = *v
			}

			command = append(command, fmt.Sprintf("--pod-eviction-timeout=%s", podEvictionTimeout.Duration))
		}

		command = append(command,
			fmt.Sprintf("--concurrent-deployment-syncs=%d", ptr.Deref(k.values.ControllerWorkers.Deployment, defaultControllerWorkersDeployment)),
			fmt.Sprintf("--concurrent-replicaset-syncs=%d", ptr.Deref(k.values.ControllerWorkers.ReplicaSet, defaultControllerWorkersReplicaSet)),
			fmt.Sprintf("--concurrent-statefulset-syncs=%d", ptr.Deref(k.values.ControllerWorkers.StatefulSet, defaultControllerWorkersStatefulSet)),
		)
	} else {
		if v := ptr.Deref(k.values.ControllerWorkers.Namespace, defaultControllerWorkersNamespace); v == 0 {
			controllersToDisable.Insert("namespace")
		}

		if v := ptr.Deref(k.values.ControllerWorkers.ServiceAccountToken, defaultControllerWorkersServiceAccountToken); v == 0 {
			controllersToDisable.Insert("serviceaccount-token")
		}

		if v := ptr.Deref(k.values.ControllerWorkers.ResourceQuota, defaultControllerWorkersResourceQuota); v == 0 {
			controllersToDisable.Insert("resourcequota")
		}

		controllersToDisable.Insert(
			"nodeipam",
			"nodelifecycle",
			"cloud-node-lifecycle",
			"attachdetach",
			"persistentvolume-binder",
			"persistentvolume-expander",
			"ttl",
		)
	}

	command = append(command,
		"--cluster-name="+k.namespace,
		fmt.Sprintf("--cluster-signing-kube-apiserver-client-cert-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyCertificateCA),
		fmt.Sprintf("--cluster-signing-kube-apiserver-client-key-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyPrivateKeyCA),
		fmt.Sprintf("--cluster-signing-legacy-unknown-cert-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyCertificateCA),
		fmt.Sprintf("--cluster-signing-legacy-unknown-key-file=%s/%s", volumeMountPathCAClient, secrets.DataKeyPrivateKeyCA),
		"--cluster-signing-duration="+ptr.Deref(k.values.ClusterSigningDuration, 720*time.Hour).String(),
		fmt.Sprintf("--concurrent-endpoint-syncs=%d", ptr.Deref(k.values.ControllerWorkers.Endpoint, defaultControllerWorkersEndpoint)),
		fmt.Sprintf("--concurrent-gc-syncs=%d", ptr.Deref(k.values.ControllerWorkers.GarbageCollector, defaultControllerWorkersGarbageCollector)),
		fmt.Sprintf("--concurrent-service-endpoint-syncs=%d", ptr.Deref(k.values.ControllerWorkers.ServiceEndpoint, defaultControllerWorkersServiceEndpoint)),
	)

	for api, enabled := range k.values.RuntimeConfig {
		if enabled {
			continue
		}

		if controllerVersionRange, present := kubernetesutils.APIGroupControllerMap[getTrimmedAPI(api)]; present {
			for controller, versionRange := range controllerVersionRange {
				if contains, err := versionRange.Contains(k.values.TargetVersion.String()); err == nil && contains {
					controllersToDisable.Insert(controller)
				}
			}
		}
	}

	cmdControllers := "--controllers=" + strings.Join(sets.List(controllersToEnable.Difference(controllersToDisable)), ",")
	if controllersToDisable.Len() > 0 {
		cmdControllers += ",-" + strings.Join(sets.List(controllersToDisable), ",-")
	}
	command = append(command, cmdControllers)

	if v := ptr.Deref(k.values.ControllerWorkers.Namespace, defaultControllerWorkersNamespace); v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-namespace-syncs=%d", v))
	}

	if v := ptr.Deref(k.values.ControllerWorkers.ResourceQuota, defaultControllerWorkersResourceQuota); v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-resource-quota-syncs=%d", v))
		if k.values.ControllerSyncPeriods.ResourceQuota != nil {
			command = append(command, "--resource-quota-sync-period="+k.values.ControllerSyncPeriods.ResourceQuota.String())
		}
	}

	if v := ptr.Deref(k.values.ControllerWorkers.ServiceAccountToken, defaultControllerWorkersServiceAccountToken); v != 0 {
		command = append(command, fmt.Sprintf("--concurrent-serviceaccount-token-syncs=%d", v))
	}

	if k.values.Config != nil && len(k.values.Config.FeatureGates) > 0 {
		command = append(command, kubernetesutils.FeatureGatesToCommandLineParameter(k.values.Config.FeatureGates))
	}

	command = append(command,
		fmt.Sprintf("--root-ca-file=%s/%s", volumeMountPathCA, secrets.DataKeyCertificateBundle),
		fmt.Sprintf("--service-account-private-key-file=%s/%s", volumeMountPathServiceAccountKey, secrets.DataKeyRSAPrivateKey),
		fmt.Sprintf("--secure-port=%d", port),
	)

	if len(k.values.ServiceNetworks) > 0 {
		command = append(command,
			fmt.Sprintf("--service-cluster-ip-range=%s", netutils.JoinByComma(k.values.ServiceNetworks)),
		)
	}

	command = append(command,
		"--profiling=false",
		fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathServer, secrets.DataKeyCertificate),
		fmt.Sprintf("--tls-private-key-file=%s/%s", volumeMountPathServer, secrets.DataKeyPrivateKey),
		fmt.Sprintf("--tls-cipher-suites=%s", strings.Join(kubernetesutils.TLSCipherSuites, ",")),
		"--use-service-account-credentials=true",
		"--v=2",
	)

	return command
}

func (k *kubeControllerManager) getHorizontalPodAutoscalerConfig() gardencorev1beta1.HorizontalPodAutoscalerConfig {
	defaultHPATolerance := gardencorev1beta1.DefaultHPATolerance
	horizontalPodAutoscalerConfig := gardencorev1beta1.HorizontalPodAutoscalerConfig{
		CPUInitializationPeriod: &metav1.Duration{Duration: gardencorev1beta1.DefaultCPUInitializationPeriod},
		DownscaleStabilization:  &metav1.Duration{Duration: gardencorev1beta1.DefaultDownscaleStabilization},
		InitialReadinessDelay:   &metav1.Duration{Duration: gardencorev1beta1.DefaultInitialReadinessDelay},
		SyncPeriod:              &metav1.Duration{Duration: gardencorev1beta1.DefaultHPASyncPeriod},
		Tolerance:               &defaultHPATolerance,
	}

	if k.values.Config != nil && k.values.Config.HorizontalPodAutoscalerConfig != nil {
		if v := k.values.Config.HorizontalPodAutoscalerConfig.CPUInitializationPeriod; v != nil {
			horizontalPodAutoscalerConfig.CPUInitializationPeriod = v
		}
		if v := k.values.Config.HorizontalPodAutoscalerConfig.DownscaleStabilization; v != nil {
			horizontalPodAutoscalerConfig.DownscaleStabilization = v
		}
		if v := k.values.Config.HorizontalPodAutoscalerConfig.InitialReadinessDelay; v != nil {
			horizontalPodAutoscalerConfig.InitialReadinessDelay = v
		}
		if v := k.values.Config.HorizontalPodAutoscalerConfig.SyncPeriod; v != nil {
			horizontalPodAutoscalerConfig.SyncPeriod = v
		}
		if v := k.values.Config.HorizontalPodAutoscalerConfig.Tolerance; v != nil {
			horizontalPodAutoscalerConfig.Tolerance = v
		}
	}
	return horizontalPodAutoscalerConfig
}

func getTrimmedAPI(api string) string {
	// The order of the suffixes are important because we exit right after we do the first replacement.
	// .k8s.io should therefore always be the very last suffix.
	knownGroupSuffixes := []string{
		".authorization.k8s.io",
		".apiserver.k8s.io",
		".k8s.io",
	}

	for _, s := range knownGroupSuffixes {
		if strings.Contains(api, s) {
			api = strings.Replace(api, s, "", 1)
			return api
		}
	}

	return api
}
