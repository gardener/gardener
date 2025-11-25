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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceNameTarget  = "logging-target"
	managedResourceName        = "opentelemetry-collector"
	serviceMonitorName         = "opentelemetry-collector"
	openTelemetryCollectorName = "gardener-opentelemetry-collector"

	kubeRBACProxyName = "rbac-proxy"

	metricsPort                    = 8888
	timeoutWaitForManagedResources = 2 * time.Minute
)

// Values is the values for OpenTelemetry Collector configurations
type Values struct {
	// Image is the collector image.
	Image string
	// KubeRBACProxyImage is the kube-rbac-proxy image.
	KubeRBACProxyImage string
	// WithRBACProxy indicates whether the collector should be deployed with kube-rbac-proxy.
	WithRBACProxy bool
	// LokiEndpoint is the endpoint of the Loki instance to which logs should be sent.
	LokiEndpoint string
	// Replicas is the number of replicas for the OpenTelemetry Collector deployment.
	Replicas int32
	// ShootNodeLoggingEnabled indicates whether the necessary resources for scraping logs from shoot nodes should be created.
	ShootNodeLoggingEnabled bool
	// IngressHost is the name for the ingress to access the OpenTelemetry Collector.
	IngressHost string
	// ValiHost is the name for the ingress to access the Vali instance.
	ValiHost string
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
	// WithAuthenticationProxy acts as a setter for the WithRBACProxy field in Values.
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
		loggingAgentShootAccessSecret    = o.newLoggingAgentShootAccessSecret()
		kubeRBACProxyShootAccessSecret   = o.newKubeRBACProxyShootAccessSecret()
		objects                          = []client.Object{}
	)

	if o.values.ShootNodeLoggingEnabled {
		if err := loggingAgentShootAccessSecret.Reconcile(ctx, o.client); err != nil {
			return err
		}
		if err := kubeRBACProxyShootAccessSecret.Reconcile(ctx, o.client); err != nil {
			return err
		}
		ingressTLSSecret, err := o.secretsManager.Generate(ctx, &secrets.CertificateSecretConfig{
			Name:                        "logging-tls",
			CommonName:                  o.values.IngressHost,
			Organization:                []string{"gardener.cloud:monitoring:ingress"},
			DNSNames:                    []string{o.values.IngressHost, o.values.ValiHost},
			CertType:                    secrets.ServerCert,
			Validity:                    ptr.To(v1beta1constants.IngressTLSCertificateValidity),
			SkipPublishingCACertificate: true,
		}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCACluster))
		if err != nil {
			return err
		}

		genericTokenKubeconfigSecret, found := o.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
		}
		genericTokenKubeconfigSecretName = genericTokenKubeconfigSecret.Name

		objects = append(objects, o.getIngress(ingressTLSSecret.Name))

		kubeRBACProxyClusterRoleBinding := o.getKubeRBACProxyClusterRoleBinding(kubeRBACProxyShootAccessSecret.ServiceAccountName)
		loggingAgentClusterRole := o.getLoggingAgentClusterRole()
		loggingAgentClusterRoleBinding := o.getLoggingAgentClusterRoleBinding(loggingAgentShootAccessSecret.ServiceAccountName, loggingAgentClusterRole.Name)

		resourcesTarget, err := managedresources.
			NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer).
			AddAllAndSerialize(
				kubeRBACProxyClusterRoleBinding,
				loggingAgentClusterRole,
				loggingAgentClusterRoleBinding,
			)
		if err != nil {
			return err
		}

		if err := managedresources.CreateForShoot(ctx, o.client, o.namespace, managedResourceNameTarget, managedresources.LabelValueGardener, false, resourcesTarget); err != nil {
			return err
		}
	} else {
		if err := managedresources.DeleteForShoot(ctx, o.client, o.namespace, managedResourceNameTarget); err != nil {
			return err
		}

		if err := kubernetesutils.DeleteObjects(ctx, o.client,
			loggingAgentShootAccessSecret.Secret,
			kubeRBACProxyShootAccessSecret.Secret,
		); err != nil {
			return err
		}
	}

	objects = append(objects, o.openTelemetryCollector(o.namespace, o.values.LokiEndpoint, genericTokenKubeconfigSecretName))
	objects = append(objects, o.serviceMonitor())
	objects = append(objects, o.serviceAccount())

	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	serializedResources, err := registry.AddAllAndSerialize(objects...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, o.client, o.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, serializedResources)
}

func (o *otelCollector) getKubeRBACProxyClusterRoleBinding(serviceAccountName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gardener.cloud:logging:rbac-proxy",
			Labels: map[string]string{v1beta1constants.LabelApp: kubeRBACProxyName},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:auth-delegator",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: metav1.NamespaceSystem,
		}},
	}
}

func (o *otelCollector) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, o.client, o.namespace, managedResourceNameTarget); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, o.client, o.namespace, managedResourceName); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, o.client,
		o.newLoggingAgentShootAccessSecret().Secret,
		o.newKubeRBACProxyShootAccessSecret().Secret,
	)
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

func (o *otelCollector) serviceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      collectorconstants.ServiceAccountName,
			Namespace: o.namespace,
			Labels:    getLabels(),
		},
		AutomountServiceAccountToken: ptr.To(false),
	}
}

func (o *otelCollector) serviceMonitor() *monitoringv1.ServiceMonitor {
	allowedMetrics := []string{
		"otelcol_exporter_enqueue_failed_log_records",
		"otelcol_exporter_enqueue_failed_metric_points",
		"otelcol_exporter_enqueue_failed_spans",
		"otelcol_exporter_queue_capacity",
		"otelcol_exporter_queue_size",
		"otelcol_exporter_send_failed_log_records_total",
		"otelcol_exporter_send_failed_metric_points",
		"otelcol_exporter_send_failed_spans",
		"otelcol_exporter_sent_log_records",
		"otelcol_exporter_sent_log_records_total",
		"otelcol_exporter_sent_metric_points",
		"otelcol_exporter_sent_spans",
		"otelcol_process_cpu_seconds",
		"otelcol_process_cpu_seconds_total",
		"otelcol_process_memory_rss",
		"otelcol_process_memory_rss_bytes",
		"otelcol_process_runtime_heap_alloc_bytes",
		"otelcol_process_runtime_total_alloc_bytes_total",
		"otelcol_process_runtime_total_sys_memory_bytes",
		"otelcol_process_uptime",
		"otelcol_process_uptime_seconds_total",
		"otelcol_processor_incoming_items",
		"otelcol_processor_incoming_items_total",
		"otelcol_processor_outgoing_items",
		"otelcol_processor_outgoing_items_total",
		"otelcol_receiver_accepted_log_records",
		"otelcol_receiver_accepted_log_records_total",
		"otelcol_receiver_accepted_metric_points",
		"otelcol_receiver_accepted_spans",
		"otelcol_receiver_refused_log_records",
		"otelcol_receiver_refused_log_records_total",
		"otelcol_receiver_refused_metric_points",
		"otelcol_receiver_refused_spans",
		"otelcol_scraper_errored_metric_points",
		"otelcol_scraper_scraped_metric_points",
	}

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(serviceMonitorName, o.namespace, shoot.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: getLabels()},
			Endpoints: []monitoringv1.Endpoint{{
				// Value must be "monitoring" since the OpenTelemetry operator creates a 'monitoring' service
				// that exposes the port via the name 'monitoring'. This currently cannot be configured with a different name.
				Port: "monitoring",
				RelabelConfigs: []monitoringv1.RelabelConfig{
					// This service monitor is targeting the logging service. Without explicitly overriding the
					// job label, prometheus-operator would choose job=logging (service name).
					{
						Action:      "replace",
						Replacement: ptr.To("opentelemetry-collector"),
						TargetLabel: "job",
					},
					{
						Action: "labelmap",
						Regex:  `__meta_kubernetes_service_label_(.+)`,
					},
				},
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(allowedMetrics...),
			}},
		},
	}
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
				Image:             o.values.Image,
				Replicas:          ptr.To(o.values.Replicas),
				PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane100,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
				},
				ServiceAccount: collectorconstants.ServiceAccountName,
			},
			Config: otelv1beta1.Config{
				Receivers: otelv1beta1.AnyConfig{
					Object: map[string]any{
						"otlp": map[string]any{
							"protocols": map[string]any{
								"grpc": map[string]any{
									"endpoint": "127.0.0.1:" + strconv.Itoa(collectorconstants.PushPort),
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
						"resource/vali": map[string]any{
							"attributes": []any{
								map[string]any{
									"key":            "nodename",
									"from_attribute": "k8s.node.name",
									"action":         "insert",
								},
								map[string]any{
									"key":            "pod_name",
									"from_attribute": "k8s.pod.name",
									"action":         "insert",
								},
								map[string]any{
									"key":            "container_name",
									"from_attribute": "k8s.container.name",
									"action":         "insert",
								},
								map[string]any{
									"key":    "loki.resource.labels",
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
													"port": metricsPort,
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
						"logs/vali": {
							Exporters: []string{
								"loki",
							},
							Receivers: []string{
								"otlp",
							},
							Processors: []string{
								"resource/vali",
								"batch",
							},
						},
					},
				},
			},
		},
	}

	if o.values.WithRBACProxy {
		// TODO(rrhubenov): Remove the rbac-proxy container when the `OpenTelemetryCollector` feature gate is promoted to GA.
		obj.Spec.Ports = append(obj.Spec.Ports, otelv1beta1.PortsSpec{
			ServicePort: corev1.ServicePort{
				Name: kubeRBACProxyName + "-vali",
				Port: collectorconstants.KubeRBACProxyValiPort,
			},
		})
		obj.Spec.Ports = append(obj.Spec.Ports, otelv1beta1.PortsSpec{
			ServicePort: corev1.ServicePort{
				Name: kubeRBACProxyName + "-otlp",
				Port: collectorconstants.KubeRBACProxyOTLPReceiverPort,
			},
		})
		obj.Spec.AdditionalContainers = []corev1.Container{
			// TODO(rrhubenov): Remove the rbac-proxy container when the `OpenTelemetryCollector` feature gate is promoted to GA.
			{
				Name:  kubeRBACProxyName + "-vali",
				Image: o.values.KubeRBACProxyImage,
				Args: []string{
					fmt.Sprintf("--insecure-listen-address=0.0.0.0:%d", collectorconstants.KubeRBACProxyValiPort),
					fmt.Sprintf("--upstream=http://logging:%d/", valiconstants.ValiPort),
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
			},
			{
				Name:  kubeRBACProxyName + "-otlp",
				Image: o.values.KubeRBACProxyImage,
				Args: []string{
					fmt.Sprintf("--insecure-listen-address=0.0.0.0:%d", collectorconstants.KubeRBACProxyOTLPReceiverPort),
					fmt.Sprintf("--upstream=http://127.0.0.1:%d/", collectorconstants.PushPort),
					"--kubeconfig=" + gardenerutils.VolumeMountPathGenericKubeconfig + "/kubeconfig",
					"--logtostderr=true",
					// The OTLP exporter uses gRPC, which operates over HTTP/2. To support HTTP/2 over cleartext (h2c),
					// we must explicitly enable h2c in kube-rbac-proxy. By default, kube-rbac-proxy enforces HTTP/2 over TLS
					// as per the HTTP/2 specification. However, since kube-rbac-proxy forwards to Vali over an unencrypted channel,
					// h2c support must be enforced.
					"--upstream-force-h2c",
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
			},
		}

		obj.Spec.Volumes = []corev1.Volume{gardenerutils.GenerateGenericKubeconfigVolume(genericTokenKubeconfigSecretName, "shoot-access-"+kubeRBACProxyName, "kubeconfig")}
		obj.Spec.AdditionalContainers[0].VolumeMounts = []corev1.VolumeMount{gardenerutils.GenerateGenericKubeconfigVolumeMount("kubeconfig", gardenerutils.VolumeMountPathGenericKubeconfig)}
		obj.Spec.AdditionalContainers[1].VolumeMounts = []corev1.VolumeMount{gardenerutils.GenerateGenericKubeconfigVolumeMount("kubeconfig", gardenerutils.VolumeMountPathGenericKubeconfig)}
	}

	return obj
}

func (o *otelCollector) newLoggingAgentShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret("opentelemetry-collector", o.namespace).
		WithServiceAccountName(openTelemetryCollectorName).
		WithTokenExpirationDuration("720h").
		WithTargetSecret(collectorconstants.OpenTelemetryCollectorSecretName, metav1.NamespaceSystem)
}

func (o *otelCollector) getIngress(secretName string) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "logging",
			Namespace: o.namespace,
			// TODO(rrrhubenov): Research whether this annotation is required before promoting the `OpenTelemetryCollector` feature gate to GA.
			Annotations: map[string]string{"nginx.ingress.kubernetes.io/backend-protocol": "GRPC"},
			Labels:      getLabels(),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To(v1beta1constants.SeedNginxIngressClass),
			TLS: []networkingv1.IngressTLS{{
				SecretName: secretName,
				Hosts:      []string{o.values.IngressHost, o.values.ValiHost},
			}},
			Rules: []networkingv1.IngressRule{
				{
					Host: o.values.IngressHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: collectorconstants.ServiceName,
										Port: networkingv1.ServiceBackendPort{Number: collectorconstants.KubeRBACProxyOTLPReceiverPort},
									},
								},
								Path:     collectorconstants.PushEndpoint,
								PathType: ptr.To(networkingv1.PathTypePrefix),
							}},
						},
					},
				},
				{
					Host: o.values.ValiHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: collectorconstants.ServiceName,
										Port: networkingv1.ServiceBackendPort{Number: collectorconstants.KubeRBACProxyValiPort},
									},
								},
								Path:     valiconstants.PushEndpoint,
								PathType: ptr.To(networkingv1.PathTypePrefix),
							}},
						},
					},
				},
			},
		},
	}
}

func (o *otelCollector) getLoggingAgentClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gardener.cloud:logging:opentelemetry-collector",
			Labels: map[string]string{v1beta1constants.LabelApp: openTelemetryCollectorName},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"", "apps"},
				Resources: []string{
					"nodes",
					"nodes/proxy",
					"services",
					"endpoints",
					"pods",
					"replicasets",
				},
				Verbs: []string{
					"get",
					"list",
					"watch",
				},
			},
			{
				NonResourceURLs: []string{collectorconstants.PushEndpoint},
				Verbs:           []string{"create"},
			},
		},
	}
}

func (o *otelCollector) getLoggingAgentClusterRoleBinding(serviceAccountName, clusterRoleName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: map[string]string{v1beta1constants.LabelApp: openTelemetryCollectorName},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: metav1.NamespaceSystem,
		}},
	}
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
