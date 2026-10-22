// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenermetricscollector_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/gardenermetricscollector"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("GardenerMetricsCollector", func() {
	var (
		ctx context.Context

		managedResourceNameRuntime = "gardener-metrics-collector-runtime"
		managedResourceNameVirtual = "gardener-metrics-collector-virtual"
		namespace                  = "some-namespace"
		image                      = "some-image:some-tag"

		fakeClient        client.Client
		fakeSecretManager secretsmanager.Interface
		deployer          component.DeployWaiter
		values            Values

		fakeOps   *retryfake.Ops
		consistOf func(...client.Object) gomegatypes.GomegaMatcher

		managedResourceRuntime       *resourcesv1alpha1.ManagedResource
		managedResourceVirtual       *resourcesv1alpha1.ManagedResource
		managedResourceSecretRuntime *corev1.Secret
		managedResourceSecretVirtual *corev1.Secret

		labels = map[string]string{
			"app":  "gardener",
			"role": "gardener-metrics",
		}

		serviceAccount                *corev1.ServiceAccount
		openTelemetryCollector        *otelv1beta1.OpenTelemetryCollector
		serviceMonitor                *monitoringv1.ServiceMonitor
		internalMetricsServiceMonitor *monitoringv1.ServiceMonitor
		vpa                           *vpaautoscalingv1.VerticalPodAutoscaler

		clusterRole        *rbacv1.ClusterRole
		clusterRoleBinding *rbacv1.ClusterRoleBinding
	)

	BeforeEach(func() {
		ctx = context.TODO()

		fakeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(fakeClient, namespace)
		values = Values{Image: image}

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

		managedResourceRuntime = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameRuntime,
				Namespace: namespace,
			},
		}
		managedResourceVirtual = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceNameVirtual,
				Namespace: namespace,
			},
		}
		managedResourceSecretRuntime = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceRuntime.Name,
				Namespace: namespace,
			},
		}
		managedResourceSecretVirtual = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceVirtual.Name,
				Namespace: namespace,
			},
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-metrics",
				Namespace: namespace,
				Labels:    labels,
			},
			AutomountServiceAccountToken: new(false),
		}

		openTelemetryCollector = &otelv1beta1.OpenTelemetryCollector{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-metrics",
				Namespace: namespace,
				Labels: map[string]string{
					"app":                              "gardener",
					"role":                             "gardener-metrics",
					"networking.gardener.cloud/to-dns": "allowed",
					"networking.resources.gardener.cloud/to-virtual-garden-kube-apiserver-tcp-443": "allowed",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":2723},{"protocol":"TCP","port":8888}]`,
				},
			},
			Spec: otelv1beta1.OpenTelemetryCollectorSpec{
				Mode:            "deployment",
				UpgradeStrategy: "none",
				Observability: otelv1beta1.ObservabilitySpec{
					Metrics: otelv1beta1.MetricsConfigSpec{
						DisablePrometheusAnnotations: true,
					},
				},
				OpenTelemetryCommonFields: otelv1beta1.OpenTelemetryCommonFields{
					Image:             image,
					Replicas:          new(int32(1)),
					PriorityClassName: "gardener-garden-system-100",
					ServiceAccount:    "gardener-metrics",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("500Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: new(false),
						ReadOnlyRootFilesystem:   new(true),
					},
					Volumes: []corev1.Volume{
						{
							Name: "kubeconfig",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{
									DefaultMode: new(int32(420)),
									Sources: []corev1.VolumeProjection{
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{Name: "generic-token-kubeconfig"},
												Items:                []corev1.KeyToPath{{Key: "kubeconfig", Path: "kubeconfig"}},
												Optional:             new(false),
											},
										},
										{
											Secret: &corev1.SecretProjection{
												LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-gardener-metrics"},
												Items:                []corev1.KeyToPath{{Key: "token", Path: "token"}},
												Optional:             new(false),
											},
										},
									},
								},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "kubeconfig",
							MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
							ReadOnly:  true,
						},
					},
					Ports: []otelv1beta1.PortsSpec{{
						ServicePort: corev1.ServicePort{
							Name:     "metrics",
							Port:     2723,
							Protocol: corev1.ProtocolTCP,
						},
					}},
				},
				Config: otelv1beta1.Config{
					Receivers: otelv1beta1.AnyConfig{
						Object: map[string]any{
							"gardener": map[string]any{
								"kubeconfig": "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig"},
						},
					},
					Exporters: otelv1beta1.AnyConfig{
						Object: map[string]any{
							"prometheus": map[string]any{
								"endpoint": "[::]:2723",
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
														"host": "[::]",
														"port": float64(8888),
													},
												},
											},
										},
									},
								},
							},
						},
						Pipelines: map[string]*otelv1beta1.Pipeline{
							"metrics": {
								Receivers: []string{"gardener"},
								Exporters: []string{"prometheus"},
							},
						},
					},
				},
			},
		}

		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "garden-gardener-metrics",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "garden"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app": "gardener",
					"operator.opentelemetry.io/collector-service-type": "base",
					"role": "gardener-metrics",
				}},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "metrics",
					RelabelConfigs: []monitoringv1.RelabelConfig{{
						Action:      "replace",
						Replacement: new("gardener-metrics"),
						TargetLabel: "job",
					}},
				}},
			},
		}

		internalMetricsServiceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "garden-gardener-metrics-monitoring",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "garden"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app": "gardener",
					"operator.opentelemetry.io/collector-service-type": "monitoring",
					"role": "gardener-metrics",
				}},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "monitoring",
					RelabelConfigs: []monitoringv1.RelabelConfig{{
						Action:      "replace",
						Replacement: new("gardener-metrics"),
						TargetLabel: "job",
					}},
					MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
						"otelcol_exporter_enqueue_failed_metric_points",
						"otelcol_exporter_queue_capacity",
						"otelcol_exporter_queue_size",
						"otelcol_exporter_send_failed_metric_points",
						"otelcol_exporter_sent_metric_points",
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
						"otelcol_receiver_accepted_metric_points",
						"otelcol_receiver_refused_metric_points",
						"otelcol_scraper_errored_metric_points",
						"otelcol_scraper_scraped_metric_points",
					),
				}},
			},
		}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-metrics",
				Namespace: namespace,
				Labels:    labels,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "opentelemetry.io/v1beta1",
					Kind:       "OpenTelemetryCollector",
					Name:       "gardener-metrics",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: new(vpaautoscalingv1.UpdateModeInPlaceOrRecreate),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "otc-container",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
							ControlledValues: new(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
						},
						{
							ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
							Mode:          new(vpaautoscalingv1.ContainerScalingModeOff),
						},
					},
				},
			},
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:gardener-metrics",
				Labels: labels,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{gardencorev1beta1.GroupName},
					Resources: []string{
						"secretbindings",
						"seeds",
						"shoots",
						"projects",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{seedmanagementv1alpha1.GroupName},
					Resources: []string{
						"managedseeds",
						"gardenlets",
					},
					Verbs: []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{securityv1alpha1.GroupName},
					Resources: []string{
						"credentialsbindings",
					},
					Verbs: []string{"get", "list", "watch"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "gardener.cloud:gardener-metrics",
				Labels: labels,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:gardener-metrics",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "gardener-metrics",
				Namespace: "kube-system",
			}},
		}

		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	JustBeforeEach(func() {
		deployer = New(fakeClient, namespace, fakeSecretManager, values)
	})

	Describe("#Deploy", func() {
		Context("resources generation", func() {
			BeforeEach(func() {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())
			})

			It("should successfully deploy all resources", func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
				expectedRuntimeMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceRuntime.Name,
						Namespace:       managedResourceRuntime.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels: map[string]string{
							"gardener.cloud/role":                "seed-system-component",
							"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class:       new("seed"),
						SecretRefs:  []corev1.LocalObjectReference{{Name: managedResourceRuntime.Spec.SecretRefs[0].Name}},
						KeepObjects: new(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedRuntimeMr))
				Expect(managedResourceRuntime).To(Equal(expectedRuntimeMr))
				Expect(managedResourceRuntime).To(consistOf(
					serviceAccount,
					openTelemetryCollector,
					serviceMonitor,
					internalMetricsServiceMonitor,
					vpa,
				))

				managedResourceSecretRuntime.Name = managedResourceRuntime.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
				expectedVirtualMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceVirtual.Name,
						Namespace:       managedResourceVirtual.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels: map[string]string{
							"origin":                             "gardener",
							"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceVirtual.Spec.SecretRefs[0].Name}},
						KeepObjects:  new(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedVirtualMr))
				Expect(managedResourceVirtual).To(Equal(expectedVirtualMr))

				managedResourceSecretVirtual.Name = expectedVirtualMr.Spec.SecretRefs[0].Name
				Expect(managedResourceVirtual).To(consistOf(clusterRole, clusterRoleBinding))
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())

				Expect(managedResourceSecretRuntime.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretRuntime.Immutable).To(Equal(new(true)))
				Expect(managedResourceSecretRuntime.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				Expect(managedResourceSecretVirtual.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretVirtual.Immutable).To(Equal(new(true)))
				Expect(managedResourceSecretVirtual.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			})
		})

		Context("secrets", func() {
			It("should successfully deploy the access secret for the virtual garden", func() {
				accessSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-access-gardener-metrics",
						Namespace: namespace,
						Labels: map[string]string{
							"resources.gardener.cloud/purpose": "token-requestor",
							"resources.gardener.cloud/class":   "shoot",
						},
						Annotations: map[string]string{
							"serviceaccount.resources.gardener.cloud/name":      "gardener-metrics",
							"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
						},
					},
					Type: corev1.SecretTypeOpaque,
				}

				Expect(deployer.Deploy(ctx)).To(Succeed())

				actualShootAccessSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(accessSecret), actualShootAccessSecret)).To(Succeed())
				accessSecret.ResourceVersion = "1"
				Expect(actualShootAccessSecret).To(Equal(accessSecret))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecretVirtual)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntime), managedResourceRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceVirtual), managedResourceVirtual)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretRuntime), managedResourceSecretRuntime)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretVirtual), managedResourceSecretVirtual)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the runtime and virtual ManagedResources are unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should succeed because the runtime and virtual ManagedResource are healthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameRuntime,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceNameVirtual,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the runtime managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceRuntime)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should fail when the wait for the virtual managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResourceVirtual)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when they are already removed", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

var (
	healthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
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
	}
	unhealthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
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
	}
)
