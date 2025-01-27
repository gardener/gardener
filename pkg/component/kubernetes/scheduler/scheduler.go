// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// BinPackingSchedulerName is the scheduler name that is used when the "bin-packing"
	// scheduling profile is configured.
	BinPackingSchedulerName = "bin-packing-scheduler"

	serviceName         = "kube-scheduler"
	secretNameServer    = "kube-scheduler-server" // #nosec G101 -- No credential.
	managedResourceName = "shoot-core-kube-scheduler"

	containerName   = v1beta1constants.DeploymentNameKubeScheduler
	portNameMetrics = "metrics"

	volumeNameClientCA      = "client-ca"
	volumeMountPathClientCA = "/var/lib/kube-scheduler-client-ca"
	fileNameClientCA        = "bundle.crt"

	volumeNameServer      = "kube-scheduler-server"
	volumeMountPathServer = "/var/lib/kube-scheduler-server"

	volumeNameConfig      = "kube-scheduler-config"
	volumeMountPathConfig = "/var/lib/kube-scheduler-config"

	dataKeyComponentConfig = "config.yaml"

	componentConfigTmpl = `apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: ` + gardenerutils.PathGenericKubeconfig + `
leaderElection:
  leaderElect: true
{{- if eq .profile "bin-packing" }}
profiles:
- schedulerName: ` + corev1.DefaultSchedulerName + `
- schedulerName: ` + BinPackingSchedulerName + `
  pluginConfig:
  - name: NodeResourcesFit
    args:
      scoringStrategy:
        type: MostAllocated
  plugins:
    score:
      disabled:
      - name: NodeResourcesBalancedAllocation
{{- end }}`
)

// New creates a new instance of DeployWaiter for the kube-scheduler.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	runtimeVersion *semver.Version,
	targetVersion *semver.Version,
	image string,
	replicas int32,
	config *gardencorev1beta1.KubeSchedulerConfig,
) component.DeployWaiter {
	return &kubeScheduler{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		runtimeVersion: runtimeVersion,
		targetVersion:  targetVersion,
		image:          image,
		replicas:       replicas,
		config:         config,
	}
}

type kubeScheduler struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	runtimeVersion *semver.Version
	targetVersion  *semver.Version
	image          string
	replicas       int32
	config         *gardencorev1beta1.KubeSchedulerConfig
}

func (k *kubeScheduler) Deploy(ctx context.Context) error {
	serverSecret, err := k.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
		Name:                        secretNameServer,
		CommonName:                  v1beta1constants.DeploymentNameKubeScheduler,
		DNSNames:                    kubernetesutils.DNSNamesForService(serviceName, k.namespace),
		CertType:                    secrets.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	genericTokenKubeconfigSecret, found := k.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	clientCASecret, found := k.secretsManager.Get(v1beta1constants.SecretNameCAClient)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAClient)
	}

	componentConfigYAML, err := k.computeComponentConfig()
	if err != nil {
		return err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-scheduler-config",
			Namespace: k.namespace,
		},
		Data: map[string]string{dataKeyComponentConfig: componentConfigYAML},
	}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	var (
		vpa                 = k.emptyVPA()
		service             = k.emptyService()
		shootAccessSecret   = k.newShootAccessSecret()
		deployment          = k.emptyDeployment()
		podDisruptionBudget = k.emptyPodDisruptionBudget()
		serviceMonitor      = k.emptyServiceMonitor()
		prometheusRule      = k.emptyPrometheusRule()

		port           int32 = 10259
		probeURIScheme       = corev1.URISchemeHTTPS
		env                  = k.computeEnvironmentVariables()
		command              = k.computeCommand(port)
	)

	if err := k.client.Create(ctx, configMap); client.IgnoreAlreadyExists(err) != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, service, func() error {
		service.Labels = getLabels()

		utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForScrapeTargets(service, networkingv1.NetworkPolicyPort{
			Port:     ptr.To(intstr.FromInt32(port)),
			Protocol: ptr.To(corev1.ProtocolTCP),
		}))

		service.Spec.Selector = getLabels()
		service.Spec.Type = corev1.ServiceTypeClusterIP
		desiredPorts := []corev1.ServicePort{{
			Name:     portNameMetrics,
			Protocol: corev1.ProtocolTCP,
			Port:     port,
		}}
		service.Spec.Ports = kubernetesutils.ReconcileServicePorts(service.Spec.Ports, desiredPorts, corev1.ServiceTypeClusterIP)

		return nil
	}); err != nil {
		return err
	}

	if err := shootAccessSecret.Reconcile(ctx, k.client); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, deployment, func() error {
		deployment.Labels = utils.MergeStringMaps(getLabels(), map[string]string{
			v1beta1constants.GardenRole:                                         v1beta1constants.GardenRoleControlPlane,
			resourcesv1alpha1.HighAvailabilityConfigType:                        resourcesv1alpha1.HighAvailabilityConfigTypeController,
			v1beta1constants.LabelExtensionProviderMutatedByControlplaneWebhook: "true",
		})
		deployment.Spec.Replicas = &k.replicas
		deployment.Spec.RevisionHistoryLimit = ptr.To[int32](1)
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: getLabels()}
		deployment.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: utils.MergeStringMaps(getLabels(), map[string]string{
					v1beta1constants.GardenRole:                 v1beta1constants.GardenRoleControlPlane,
					v1beta1constants.LabelPodMaintenanceRestart: "true",
					v1beta1constants.LabelNetworkPolicyToDNS:    v1beta1constants.LabelNetworkPolicyAllowed,
					gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			Spec: corev1.PodSpec{
				AutomountServiceAccountToken: ptr.To(false),
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
						Image:           k.image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         command,
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
						},
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
						Env: env,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("5m"),
								corev1.ResourceMemory: resource.MustParse("30M"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      volumeNameClientCA,
								MountPath: volumeMountPathClientCA,
							},
							{
								Name:      volumeNameServer,
								MountPath: volumeMountPathServer,
							},
							{
								Name:      volumeNameConfig,
								MountPath: volumeMountPathConfig,
							},
						},
					},
				},
				PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane300,
				Volumes: []corev1.Volume{
					{
						Name: volumeNameClientCA,
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								DefaultMode: ptr.To[int32](420),
								Sources: []corev1.VolumeProjection{
									{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: clientCASecret.Name,
											},
											Items: []corev1.KeyToPath{{
												Key:  secrets.DataKeyCertificateBundle,
												Path: fileNameClientCA,
											}},
										},
									},
								},
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
					{
						Name: volumeNameConfig,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					},
				},
			},
		}

		utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecret.Name, shootAccessSecret.Secret.Name))
		utilruntime.Must(references.InjectAnnotations(deployment))
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, podDisruptionBudget, func() error {
		podDisruptionBudget.Labels = getLabels()
		podDisruptionBudget.Spec = policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable: ptr.To(intstr.FromInt32(1)),
			Selector:       deployment.Spec.Selector,
		}

		kubernetesutils.SetAlwaysAllowEviction(podDisruptionBudget, k.runtimeVersion)

		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       v1beta1constants.DeploymentNameKubeScheduler,
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

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, prometheusRule, func() error {
		metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", shoot.Label)
		prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "kube-scheduler.rules",
				Rules: []monitoringv1.Rule{
					{
						Alert: "KubeSchedulerDown",
						Expr:  intstr.FromString(`absent(up{job="kube-scheduler"} == 1)`),
						For:   ptr.To(monitoringv1.Duration("15m")),
						Labels: map[string]string{
							"service":    v1beta1constants.DeploymentNameKubeScheduler,
							"severity":   "critical",
							"type":       "seed",
							"visibility": "all",
						},
						Annotations: map[string]string{
							"summary":     "Kube Scheduler is down.",
							"description": "New pods are not being assigned to nodes.",
						},
					},
					// Scheduling duration
					{
						Record: "cluster:scheduler_e2e_scheduling_duration_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.99, sum(scheduler_e2e_scheduling_duration_seconds_bucket) BY (le, cluster))`),
						Labels: map[string]string{"quantile": "0.99"},
					},
					{
						Record: "cluster:scheduler_e2e_scheduling_duration_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.9, sum(scheduler_e2e_scheduling_duration_seconds_bucket) BY (le, cluster))`),
						Labels: map[string]string{"quantile": "0.9"},
					},
					{
						Record: "cluster:scheduler_e2e_scheduling_duration_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.5, sum(scheduler_e2e_scheduling_duration_seconds_bucket) BY (le, cluster))`),
						Labels: map[string]string{"quantile": "0.5"},
					},
					{
						Record: "cluster:scheduler_scheduling_algorithm_duration_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.99, sum(scheduler_scheduling_algorithm_duration_seconds_bucket) BY (le, cluster))`),
						Labels: map[string]string{"quantile": "0.99"},
					},
					{
						Record: "cluster:scheduler_scheduling_algorithm_duration_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.9, sum(scheduler_scheduling_algorithm_duration_seconds_bucket) BY (le, cluster))`),
						Labels: map[string]string{"quantile": "0.9"},
					},
					{
						Record: "cluster:scheduler_scheduling_algorithm_duration_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.5, sum(scheduler_scheduling_algorithm_duration_seconds_bucket) BY (le, cluster))`),
						Labels: map[string]string{"quantile": "0.5"},
					},
					{
						Record: "cluster:scheduler_binding_duration_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.99, sum(scheduler_binding_duration_seconds_bucket) BY (le, cluster))`),
						Labels: map[string]string{"quantile": "0.99"},
					},
					{
						Record: "cluster:scheduler_binding_duration_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.9, sum(scheduler_binding_duration_seconds_bucket) BY (le, cluster))`),
						Labels: map[string]string{"quantile": "0.9"},
					},
					{
						Record: "cluster:scheduler_binding_duration_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.5, sum(scheduler_binding_duration_seconds_bucket) BY (le, cluster))`),
						Labels: map[string]string{"quantile": "0.5"},
					},
				},
			}},
		}

		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, serviceMonitor, func() error {
		metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", shoot.Label)
		serviceMonitor.Spec = monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				Port:      portNameMetrics,
				Scheme:    "https",
				TLSConfig: &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)}},
				Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: shoot.AccessSecretName},
					Key:                  resourcesv1alpha1.DataKeyToken,
				}},
				RelabelConfigs: []monitoringv1.RelabelConfig{{
					Action: "labelmap",
					Regex:  `__meta_kubernetes_service_label_(.+)`,
				}},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"scheduler_binding_duration_seconds_bucket",
					"scheduler_e2e_scheduling_duration_seconds_bucket",
					"scheduler_scheduling_algorithm_duration_seconds_bucket",
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

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelScheduler,
	}
}

func (k *kubeScheduler) Destroy(_ context.Context) error     { return nil }
func (k *kubeScheduler) Wait(_ context.Context) error        { return nil }
func (k *kubeScheduler) WaitCleanup(_ context.Context) error { return nil }

func (k *kubeScheduler) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-vpa", Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyPodDisruptionBudget() *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeScheduler, Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: serviceName, Namespace: k.namespace}}
}

func (k *kubeScheduler) emptyPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{ObjectMeta: monitoringutils.ConfigObjectMeta(v1beta1constants.DeploymentNameKubeScheduler, k.namespace, shoot.Label)}
}

func (k *kubeScheduler) emptyServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{ObjectMeta: monitoringutils.ConfigObjectMeta(v1beta1constants.DeploymentNameKubeScheduler, k.namespace, shoot.Label)}
}

func (k *kubeScheduler) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeScheduler, Namespace: k.namespace}}
}

func (k *kubeScheduler) newShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(v1beta1constants.DeploymentNameKubeScheduler, k.namespace)
}

func (k *kubeScheduler) reconcileShootResources(ctx context.Context, serviceAccountName string) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		clusterRoleBinding1 = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:kube-scheduler",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:kube-scheduler",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}
		clusterRoleBinding2 = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:kube-scheduler-volume",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "system:volume-scheduler",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}
	)

	data, err := registry.AddAllAndSerialize(clusterRoleBinding1, clusterRoleBinding2)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, k.client, k.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (k *kubeScheduler) computeEnvironmentVariables() []corev1.EnvVar {
	if k.config != nil && k.config.KubeMaxPDVols != nil {
		return []corev1.EnvVar{{
			Name:  "KUBE_MAX_PD_VOLS",
			Value: *k.config.KubeMaxPDVols,
		}}
	}
	return nil
}

func (k *kubeScheduler) computeComponentConfig() (string, error) {
	profile := gardencorev1beta1.SchedulingProfileBalanced
	if k.config != nil && k.config.Profile != nil {
		profile = *k.config.Profile
	}

	var (
		componentConfigYAML bytes.Buffer
		values              = map[string]string{
			"profile": string(profile),
		}
	)
	if err := componentConfigTemplate.Execute(&componentConfigYAML, values); err != nil {
		return "", err
	}

	return componentConfigYAML.String(), nil
}

func (k *kubeScheduler) computeCommand(port int32) []string {
	var command []string

	command = append(command,
		"/usr/local/bin/kube-scheduler",
		fmt.Sprintf("--config=%s/%s", volumeMountPathConfig, dataKeyComponentConfig),
		"--authentication-kubeconfig="+gardenerutils.PathGenericKubeconfig,
		"--authorization-kubeconfig="+gardenerutils.PathGenericKubeconfig,
		fmt.Sprintf("--client-ca-file=%s/%s", volumeMountPathClientCA, fileNameClientCA),
		fmt.Sprintf("--tls-cert-file=%s/%s", volumeMountPathServer, secrets.DataKeyCertificate),
		fmt.Sprintf("--tls-private-key-file=%s/%s", volumeMountPathServer, secrets.DataKeyPrivateKey),
		fmt.Sprintf("--secure-port=%d", port),
	)

	if k.config != nil {
		command = append(command, kubernetesutils.FeatureGatesToCommandLineParameter(k.config.FeatureGates))
	}

	command = append(command, "--v=2")
	return command
}

var componentConfigTemplate *template.Template

func init() {
	var err error

	componentConfigTemplate, err = template.New("config").Parse(componentConfigTmpl)
	utilruntime.Must(err)
}
