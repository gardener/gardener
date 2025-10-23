// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package collector_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	. "github.com/gardener/gardener/pkg/component/observability/opentelemetry/collector"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	ingressName                     = "logging"
	namespace                       = "some-namespace"
	valiHost                        = "vali.foo.bar"
	managedResourceNameTarget       = "logging-target"
	managedResourceSecretNameTarget = "managedresource-logging-target"
)

var _ = Describe("OpenTelemetry Collector", func() {
	var (
		ctx = context.Background()

		image                                       = "some-image:some-tag"
		lokiEndpoint                                = "logging"
		genericTokenKubeconfigSecretName            = "generic-token-kubeconfig"
		kubeRBACProxyImage                          = "kube-rbac-proxy:latest"
		kubeRBACProxyShootAccessSecretName          = "shoot-access-rbac-proxy"
		opentelemetryCollectorShootAccessSecretName = "shoot-access-opentelemetry-collector"
		values                                      = Values{
			Image:                   image,
			KubeRBACProxyImage:      kubeRBACProxyImage,
			LokiEndpoint:            lokiEndpoint,
			Replicas:                1,
			ShootNodeLoggingEnabled: true,
			IngressHost:             valiHost,
		}

		c         client.Client
		component Interface
		consistOf func(...client.Object) types.GomegaMatcher

		customResourcesManagedResourceName   = "opentelemetry-collector"
		customResourcesManagedResource       *resourcesv1alpha1.ManagedResource
		customResourcesManagedResourceSecret *corev1.Secret
		managedResourceSecretTarget          *corev1.Secret
		fakeSecretManager                    secretsmanager.Interface
		kubeRBACProxyContainer               corev1.Container

		volume                 corev1.Volume
		volumeMount            corev1.VolumeMount
		managedResourceTarget  *resourcesv1alpha1.ManagedResource
		openTelemetryCollector *otelv1beta1.OpenTelemetryCollector
		serviceMonitor         *monitoringv1.ServiceMonitor
		serviceAccount         *corev1.ServiceAccount
		kubeRBACServicePort    corev1.ServicePort
	)

	BeforeEach(func() {
		format.MaxDepth = 100000
		format.MaxLength = 100000
		format.TruncateThreshold = 100000
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(c, namespace)
		component = New(c, namespace, values, fakeSecretManager)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		By("Create secrets managed outside of this package for which secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	JustBeforeEach(func() {
		customResourcesManagedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "opentelemetry-collector",
				Namespace: namespace,
			},
		}
		managedResourceTarget = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameTarget,
				Namespace: namespace,
			},
		}
		customResourcesManagedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + customResourcesManagedResource.Name,
				Namespace: namespace,
			},
		}
		managedResourceSecretTarget = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSecretNameTarget,
				Namespace: namespace,
			},
		}

		volume = corev1.Volume{
			Name: "kubeconfig",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](420),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: genericTokenKubeconfigSecretName,
								},
								Items: []corev1.KeyToPath{{
									Key:  secrets.DataKeyKubeconfig,
									Path: secrets.DataKeyKubeconfig,
								}},
								Optional: ptr.To(false),
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "shoot-access-rbac-proxy",
								},
								Items: []corev1.KeyToPath{{
									Key:  resourcesv1alpha1.DataKeyToken,
									Path: resourcesv1alpha1.DataKeyToken,
								}},
								Optional: ptr.To(false),
							},
						},
					},
				},
			},
		}

		volumeMount = corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
			ReadOnly:  true,
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "opentelemetry-collector",
				Namespace: namespace,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: ptr.To(false),
		}

		kubeRBACProxyContainer = corev1.Container{
			Name:  "rbac-proxy",
			Image: kubeRBACProxyImage,
			Args: []string{
				"--insecure-listen-address=0.0.0.0:8080",
				"--upstream=http://127.0.0.1:4317/",
				"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
				"--logtostderr=true",
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
			VolumeMounts: []corev1.VolumeMount{
				volumeMount,
			},
		}

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

		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-opentelemetry-collector",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "shoot"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: getLabels()},
				Endpoints: []monitoringv1.Endpoint{{
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

		kubeRBACServicePort = corev1.ServicePort{
			Name: "rbac-proxy",
			Port: 8080,
		}

		openTelemetryCollector = &otelv1beta1.OpenTelemetryCollector{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "opentelemetry-collector",
				Namespace: namespace,
				Labels:    getLabels(),
				Annotations: map[string]string{
					`networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports`: `[{"protocol":"TCP","port":8888}]`,
				},
			},
			Spec: otelv1beta1.OpenTelemetryCollectorSpec{
				Mode:            "deployment",
				UpgradeStrategy: "none",
				OpenTelemetryCommonFields: otelv1beta1.OpenTelemetryCommonFields{
					Image:             image,
					Replicas:          ptr.To[int32](1),
					PriorityClassName: "gardener-system-100",
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("50Mi"),
						},
					},
					ServiceAccount: "opentelemetry-collector",
				},
				Config: otelv1beta1.Config{
					Receivers: otelv1beta1.AnyConfig{
						Object: map[string]any{
							"otlp": map[string]any{
								"protocols": map[string]any{
									"grpc": map[string]any{
										"endpoint": "127.0.0.1:4317",
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
														// Field needs to be cast to `float64` due to an issue with serialization during tests.
														// When fetching the object from the apiserver, since there's no type information regarding this field.
														// the deserializer will interpret it as a `float64`. By setting the value to `float64` here, we ensure that
														// when this object is compared to the fetched one, the types match.
														"port": float64(8888),
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
								Exporters:  []string{"loki"},
								Receivers:  []string{"otlp"},
								Processors: []string{"resource/vali", "batch"},
							},
						},
					},
				},
			},
		}

	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources without kubeRBACProxy when AuthenticationProxy is false", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: opentelemetryCollectorShootAccessSecretName, Namespace: namespace}, &corev1.Secret{})).To(BeNotFoundError())

			component.WithAuthenticationProxy(false)
			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "opentelemetry-collector",
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole:          "seed-system-component",
						"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
					},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: customResourcesManagedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(customResourcesManagedResource).To(DeepEqual(expectedMr))

			customResourcesManagedResourceSecret.Name = customResourcesManagedResource.Spec.SecretRefs[0].Name
			Expect(customResourcesManagedResource).To(consistOf(
				openTelemetryCollector,
				getIngress("/opentelemetry.proto.collector.logs.v1.LogsService/Export", "opentelemetry-collector-collector", 8080),
				serviceMonitor,
				serviceAccount,
			))

			// Expect(c.Get(ctx, client.ObjectKey{Name: kubeRBACProxyShootAccessSecretName, Namespace: namespace}, &corev1.Secret{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(Succeed())
			Expect(customResourcesManagedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(customResourcesManagedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(customResourcesManagedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
		})

		It("should successfully deploy all resources with kubeRBACProxy when AuthenticationProxy is true", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: opentelemetryCollectorShootAccessSecretName, Namespace: namespace}, &corev1.Secret{})).To(BeNotFoundError())

			component.WithAuthenticationProxy(true)
			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "opentelemetry-collector",
					Namespace: namespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole:          "seed-system-component",
						"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
					},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: customResourcesManagedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(customResourcesManagedResource).To(DeepEqual(expectedMr))

			customResourcesManagedResourceSecret.Name = customResourcesManagedResource.Spec.SecretRefs[0].Name
			openTelemetryCollector.Spec.AdditionalContainers = []corev1.Container{kubeRBACProxyContainer}
			openTelemetryCollector.Spec.Volumes = []corev1.Volume{volume}
			openTelemetryCollector.Spec.Ports = append(openTelemetryCollector.Spec.Ports, otelv1beta1.PortsSpec{
				ServicePort: kubeRBACServicePort,
			})
			Expect(customResourcesManagedResource).To(consistOf(
				openTelemetryCollector,
				getIngress("/opentelemetry.proto.collector.logs.v1.LogsService/Export", "opentelemetry-collector-collector", 8080),
				serviceMonitor,
				serviceAccount,
			))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(Succeed())
			Expect(customResourcesManagedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(customResourcesManagedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(customResourcesManagedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceTarget), managedResourceTarget)).To(Succeed())
			expectedTargetMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceNameTarget,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceTarget.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedTargetMr))
			Expect(managedResourceTarget).To(DeepEqual(expectedTargetMr))
			Expect(managedResourceTarget).To(consistOf(
				getKubeRBACProxyClusterRoleBinding(),
				getValitailClusterRole("gardener.cloud:logging:opentelemetry-collector", "gardener-opentelemetry-collector", "/opentelemetry.proto.collector.logs.v1.LogsService/Export"),
				getValitailClusterRoleBinding("gardener.cloud:logging:opentelemetry-collector", "gardener-opentelemetry-collector", "gardener.cloud:logging:opentelemetry-collector", "gardener-opentelemetry-collector"),
			))

			managedResourceSecretTarget.Name = managedResourceTarget.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKey{Name: kubeRBACProxyShootAccessSecretName, Namespace: namespace}, &corev1.Secret{})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretTarget), managedResourceSecretTarget)).To(Succeed())
			Expect(managedResourceSecretTarget.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecretTarget.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecretTarget.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

		})

	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, customResourcesManagedResource)).To(Succeed())
			Expect(c.Create(ctx, customResourcesManagedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(BeNotFoundError())
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
			It("should fail because reading the ManagedResources fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResources doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       customResourcesManagedResourceName,
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

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resources to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       customResourcesManagedResourceName,
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

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resources deletion times out", func() {
				fakeOps.MaxAttempts = 2

				customResourcesManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      customResourcesManagedResourceName,
						Namespace: namespace,
					},
				}
				Expect(c.Create(ctx, customResourcesManagedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

func getIngress(path, serviceName string, port int32) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	annotations := map[string]string{"nginx.ingress.kubernetes.io/backend-protocol": "GRPC"}

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   namespace,
			Annotations: annotations,
			Labels:      getLabels(),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx-ingress-gardener"),
			TLS: []networkingv1.IngressTLS{
				{
					SecretName: "logging-tls",
					Hosts:      []string{valiHost},
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: valiHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1.ServiceBackendPort{
												Number: port,
											},
										},
									},
									Path:     path,
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
		},
	}
}

func getKubeRBACProxyClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "gardener.cloud:logging:rbac-proxy",
			Labels: map[string]string{"app": "rbac-proxy"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:auth-delegator",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "rbac-proxy",
			Namespace: "kube-system",
		}},
	}
}

func getValitailClusterRole(name, appName, path string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"app": appName},
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
				NonResourceURLs: []string{path},
				Verbs:           []string{"create"},
			},
		},
	}
}

func getValitailClusterRoleBinding(name, appName, roleName, subjectName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"app": appName},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      subjectName,
			Namespace: "kube-system",
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
