// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"fmt"
	"strconv"
	"time"

	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	collectorconstants "github.com/gardener/gardener/pkg/component/observability/opentelemetry/collector/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceName     = "opentelemetry-collector"
	scrapeJobName           = "opentelemetry-collector"
	scrapeConfigName        = "opentelemetry-collector"
	otelCollectorConfigName = "opentelemetry-collector-config"
	kubeRBACProxyName       = "kube-rbac-proxy"

	metricsEndpointName            = "metrics"
	metricsPort                    = 8888
	timeoutWaitForManagedResources = 2 * time.Minute
)

// Values is the values for OpenTelemetry Collector configurations
type Values struct {
	// Image is the collector image.
	Image              string
	KubeRBACProxyImage string
	WithRBACProxy      bool
	LokiEndpoint       string
}

type otelCollector struct {
	client         client.Client
	namespace      string
	values         Values
	secretsManager secretsmanager.Interface
}

// Interface is the interface for the OpenTelemetry Collector deployer.
type Interface interface {
	component.DeployWaiter
	WithAuthenticationProxy(bool)
}

// New creates a new instance of OpenTelemetry Collector deployer.
func New(
	client client.Client,
	namespace string,
	values Values,
	secretsManager secretsmanager.Interface,
) Interface {
	return &otelCollector{
		client:         client,
		namespace:      namespace,
		values:         values,
		secretsManager: secretsManager,
	}
}

func (o *otelCollector) WithAuthenticationProxy(b bool) {
	o.values.WithRBACProxy = b
}

func (o *otelCollector) newKubeRBACProxyShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(kubeRBACProxyName, o.namespace)
}

func (o *otelCollector) Deploy(ctx context.Context) error {
	var (
		genericTokenKubeconfigSecretName string
		kubeRBACProxyShootAccessSecret   = o.newKubeRBACProxyShootAccessSecret()
		objects                          = []client.Object{}
	)

	if err := kubeRBACProxyShootAccessSecret.Reconcile(ctx, o.client); err != nil {
		return err
	}

	genericTokenKubeconfigSecret, found := o.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}
	genericTokenKubeconfigSecretName = genericTokenKubeconfigSecret.Name

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	objects = append(objects, o.openTelemetryCollector(o.namespace, o.values.LokiEndpoint, genericTokenKubeconfigSecretName))

	scrapeConfig, err := o.scrapeConfig(ctx)
	if err != nil {
		return err
	}
	objects = append(objects, scrapeConfig)

	serializedResources, err := registry.AddAllAndSerialize(objects...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, o.client, o.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (o *otelCollector) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, o.client, o.namespace, managedResourceName)
}

func (o *otelCollector) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, o.client, o.namespace, managedResourceName)
}

func (o *otelCollector) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, o.client, o.namespace, managedResourceName)
}

func (o *otelCollector) emptyScrapeConfig() *monitoringv1alpha1.ScrapeConfig {
	return &monitoringv1alpha1.ScrapeConfig{ObjectMeta: monitoringutils.ConfigObjectMeta(scrapeConfigName, o.namespace, shoot.Label)}
}

func (o *otelCollector) scrapeConfig(ctx context.Context) (*monitoringv1alpha1.ScrapeConfig, error) {
	allowedMetrics := []string{
		"otelcol_exporter_enqueue_failed_log_records",
		"otelcol_exporter_enqueue_failed_metric_points",
		"otelcol_exporter_enqueue_failed_spans",
		"otelcol_exporter_queue_capacity",
		"otelcol_exporter_queue_size",
		"otelcol_exporter_send_failed_log_records",
		"otelcol_exporter_send_failed_metric_points",
		"otelcol_exporter_send_failed_spans",
		"otelcol_exporter_sent_log_records",
		"otelcol_exporter_sent_metric_points",
		"otelcol_exporter_sent_spans",
		"otelcol_process_cpu_seconds",
		"otelcol_process_memory_rss",
		"otelcol_process_runtime_heap_alloc_bytes",
		"otelcol_process_runtime_total_alloc_bytes",
		"otelcol_process_runtime_total_sys_memory_bytes",
		"otelcol_process_uptime",
		"otelcol_processor_incoming_items",
		"otelcol_processor_outgoing_items",
		"otelcol_receiver_accepted_log_records",
		"otelcol_receiver_accepted_metric_points",
		"otelcol_receiver_accepted_spans",
		"otelcol_receiver_refused_log_records",
		"otelcol_receiver_refused_metric_points",
		"otelcol_receiver_refused_spans",
		"otelcol_scraper_errored_metric_points",
		"otelcol_scraper_scraped_metric_points",
	}

	scrapeConfig := o.emptyScrapeConfig()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, o.client, scrapeConfig, func() error {
		metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", shoot.Label)
		scrapeConfig.Spec = monitoringv1alpha1.ScrapeConfigSpec{
			KubernetesSDConfigs: []monitoringv1alpha1.KubernetesSDConfig{{
				Role:       monitoringv1alpha1.KubernetesRoleService,
				Namespaces: &monitoringv1alpha1.NamespaceDiscovery{Names: []string{o.namespace}},
			}},
			RelabelConfigs: []monitoringv1.RelabelConfig{
				{
					Action:      "replace",
					Replacement: ptr.To(scrapeJobName),
					TargetLabel: "job",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_service_name", "__meta_kubernetes_service_port_name"},
					Action:       "keep",
					Regex:        collectorconstants.ServiceName + `;` + metricsEndpointName,
				},
			},
			MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(allowedMetrics...),
		}

		return nil
	})

	return scrapeConfig, err
}

func (o *otelCollector) openTelemetryCollector(namespace, lokiEndpoint, genericTokenKubeconfigSecretName string) *otelv1beta1.OpenTelemetryCollector {
	obj := &otelv1beta1.OpenTelemetryCollector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      collectorconstants.OpenTelemetryCollectorResourceName,
			Namespace: namespace,
			Labels:    getLabels(),
			// We want this annotation to be passed down to the service that will be created by the OpenTelemetry Operator.
			// Currently, there is no other way to define the annotations on the service other than adding them to the OpenTelemetryCollector resource.
			// All annotations that exist here will be passed down to every resource that gets created by the OpenTelemetry Operator.
			Annotations: map[string]string{
				"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": fmt.Sprintf(`[{"protocol":"TCP","port":%d}]`, metricsPort),
			},
		},
		Spec: otelv1beta1.OpenTelemetryCollectorSpec{
			Mode:            "deployment",
			UpgradeStrategy: "none",
			OpenTelemetryCommonFields: otelv1beta1.OpenTelemetryCommonFields{
				Image: o.values.Image,
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
				},
				Ports: []otelv1beta1.PortsSpec{
					{
						ServicePort: corev1.ServicePort{
							Name: kubeRBACProxyName,
							Port: collectorconstants.KubeRBACProxyPort,
						},
					},
					{
						ServicePort: corev1.ServicePort{
							Name: metricsEndpointName,
							Port: metricsPort,
						},
					},
				},
			},
			Config: otelv1beta1.Config{
				Receivers: otelv1beta1.AnyConfig{
					Object: map[string]any{
						"loki": map[string]any{
							"protocols": map[string]any{
								"http": map[string]any{
									"endpoint": "0.0.0.0:" + strconv.Itoa(collectorconstants.PushPort),
								},
							},
						},
					},
				},
				Processors: &otelv1beta1.AnyConfig{
					Object: map[string]any{
						"batch": map[string]any{
							"timeout": "10s",
						},
						"attributes/labels": map[string]any{
							"actions": []any{
								map[string]any{
									"key":    "loki.attribute.labels",
									"value":  "job, unit, nodename, origin, pod_name, container_name, namespace_name, gardener_cloud_role",
									"action": "insert",
								},
								map[string]any{
									"key":    "loki.format",
									"value":  "logfmt",
									"action": "insert",
								},
							},
						},
					},
				},
				Exporters: otelv1beta1.AnyConfig{
					Object: map[string]any{
						"loki": map[string]any{
							"endpoint": lokiEndpoint,
						},
					},
				},
				Service: otelv1beta1.Service{
					Telemetry: &otelv1beta1.AnyConfig{
						Object: map[string]any{
							"metrics": map[string]any{
								"level": "basic",
								"readers": []any{
									map[string]any{
										"pull": map[string]any{
											"exporter": map[string]any{
												"prometheus": map[string]any{
													"host": "0.0.0.0",
													// Field needs to be cast to `float64` due to an issue with serialization during tests.
													"port": float64(metricsPort),
												},
											},
										},
									},
								},
							},
							"logs": map[string]any{
								"level":    "info",
								"encoding": "json",
							},
						},
					},
					Pipelines: map[string]*otelv1beta1.Pipeline{
						"logs": {
							Exporters: []string{
								"loki",
							},
							Receivers: []string{
								"loki",
							},
							Processors: []string{
								"attributes/labels",
								"batch",
							},
						},
					},
				},
			},
		},
	}

	if o.values.WithRBACProxy {
		obj.Spec.AdditionalContainers = []corev1.Container{
			{
				Name:  kubeRBACProxyName,
				Image: o.values.KubeRBACProxyImage,
				Args: []string{
					fmt.Sprintf("--insecure-listen-address=0.0.0.0:%d", collectorconstants.KubeRBACProxyPort),
					fmt.Sprintf("--upstream=http://127.0.0.1:%d/", collectorconstants.PushPort),
					"--kubeconfig=" + gardenerutils.VolumeMountPathGenericKubeconfig + "/kubeconfig",
					"--logtostderr=true",
					"--v=6",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("5m"),
						corev1.ResourceMemory: resource.MustParse("30Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					RunAsUser:                ptr.To[int64](65532),
					RunAsGroup:               ptr.To[int64](65534),
					RunAsNonRoot:             ptr.To(true),
					ReadOnlyRootFilesystem:   ptr.To(true),
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          kubeRBACProxyName,
						ContainerPort: collectorconstants.KubeRBACProxyPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
			},
		}

		obj.Spec.Volumes = []corev1.Volume{gardenerutils.GenerateGenericKubeconfigVolume(genericTokenKubeconfigSecretName, "shoot-access-"+kubeRBACProxyName, "kubeconfig")}
		obj.Spec.AdditionalContainers[0].VolumeMounts = []corev1.VolumeMount{gardenerutils.GenerateGenericKubeconfigVolumeMount("kubeconfig", gardenerutils.VolumeMountPathGenericKubeconfig)}
	}

	return obj
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelRole:  v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleObservability,
		gardenerutils.NetworkPolicyLabel(valiconstants.ServiceName, valiconstants.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToDNS:                                            v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                               v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelObservabilityApplication:                                      "opentelemetry-collector",
	}
}
