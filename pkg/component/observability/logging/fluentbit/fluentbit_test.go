// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentbit_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/logging/fluentbit"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Fluent Bit", func() {
	var (
		ctx = context.Background()

		namespace         = "some-namespace"
		image             = "some-image:some-tag"
		priorityClassName = "some-priority-class"
		values            = Values{
			Image:              image,
			InitContainerImage: image,
			ValiEnabled:        true,
			PriorityClassName:  priorityClassName,
		}

		c         client.Client
		component component.DeployWaiter
		contains  func(...client.Object) types.GomegaMatcher

		customResourcesManagedResourceName   = "fluent-bit"
		customResourcesManagedResource       *resourcesv1alpha1.ManagedResource
		customResourcesManagedResourceSecret *corev1.Secret

		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "aggregate-fluent-bit",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "aggregate"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app":                              "fluent-bit",
					"role":                             "logging",
					"gardener.cloud/role":              "logging",
					"networking.gardener.cloud/to-dns": "allowed",
					"networking.gardener.cloud/to-runtime-apiserver":                     "allowed",
					"networking.resources.gardener.cloud/to-all-shoots-logging-tcp-3100": "allowed",
					"networking.resources.gardener.cloud/to-logging-tcp-3100":            "allowed",
				}},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "metrics",
					RelabelConfigs: []monitoringv1.RelabelConfig{
						{
							TargetLabel: "__metrics_path__",
							Replacement: ptr.To("/api/v1/metrics/prometheus"),
						},
						{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_pod_label_(.+)`,
						},
					},
					MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
						SourceLabels: []monitoringv1.LabelName{"__name__"},
						Action:       "keep",
						Regex:        `^(fluentbit_input_bytes_total|fluentbit_input_records_total|fluentbit_output_proc_bytes_total|fluentbit_output_proc_records_total|fluentbit_output_errors_total|fluentbit_output_retries_total|fluentbit_output_retries_failed_total|fluentbit_filter_add_records_total|fluentbit_filter_drop_records_total)$`,
					}},
				}},
			},
		}
		serviceMonitorPlugin = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "aggregate-fluent-bit-output-plugin",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "aggregate"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app":                              "fluent-bit",
					"role":                             "logging",
					"gardener.cloud/role":              "logging",
					"networking.gardener.cloud/to-dns": "allowed",
					"networking.gardener.cloud/to-runtime-apiserver":                     "allowed",
					"networking.resources.gardener.cloud/to-all-shoots-logging-tcp-3100": "allowed",
					"networking.resources.gardener.cloud/to-logging-tcp-3100":            "allowed",
				}},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "metrics-plugin",
					RelabelConfigs: []monitoringv1.RelabelConfig{
						{
							Action:      "replace",
							Replacement: ptr.To("fluent-bit-output-plugin"),
							TargetLabel: "job",
						},
						{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_pod_label_(.+)`,
						},
					},
					MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
						SourceLabels: []monitoringv1.LabelName{"__name__"},
						Action:       "keep",
						Regex:        `^(valitail_dropped_entries_total|fluentbit_vali_gardener_errors_total|fluentbit_vali_gardener_logs_without_metadata_total|fluentbit_vali_gardener_incoming_logs_total|fluentbit_vali_gardener_incoming_logs_with_endpoint_total|fluentbit_vali_gardener_forwarded_logs_total|fluentbit_vali_gardener_dropped_logs_total)$`,
					}},
				}},
			},
		}
		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "aggregate-fluent-bit",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "aggregate"},
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "fluent-bit.rules",
					Rules: []monitoringv1.Rule{
						{
							Alert: "FluentBitDown",
							Expr:  intstr.FromString(`absent(up{job="fluent-bit"} == 1)`),
							For:   ptr.To(monitoringv1.Duration("15m")),
							Labels: map[string]string{
								"service":    "logging",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "There are no fluent-bit pods running on seed: {{$externalLabels.seed}}. No logs will be collected.",
								"summary":     "Fluent-bit is down",
							},
						},
						{
							Alert: "FluentBitIdleInputPlugins",
							Expr:  intstr.FromString(`sum by (pod) (increase(fluentbit_input_bytes_total{pod=~"fluent-bit.*"}[4m])) == 0`),
							For:   ptr.To(monitoringv1.Duration("6h")),
							Labels: map[string]string{
								"service":    "logging",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "The input plugins of Fluent-bit pod {{$labels.pod}} running on seed {{$externalLabels.seed}} haven't collected any logs for the last 6 hours.",
								"summary":     "Fluent-bit input plugins haven't process any data for the past 6 hours",
							},
						},
						{
							Alert: "FluentBitReceivesLogsWithoutMetadata",
							Expr:  intstr.FromString(`sum by (pod) (increase(fluentbit_vali_gardener_logs_without_metadata_total[4m])) > 0`),
							Labels: map[string]string{
								"service":    "logging",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "{{$labels.pod}} receives logs without metadata on seed: {{$externalLabels.seed}}. These logs will be dropped.",
								"summary":     "Fluent-bit receives logs without metadata",
							},
						},
						{
							Alert: "FluentBitSendsOoOLogs",
							Expr:  intstr.FromString(`sum by (pod) (increase(prometheus_target_scrapes_sample_out_of_order_total[4m])) > 0`),
							Labels: map[string]string{
								"service":    "logging",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "{{$labels.pod}} on seed: {{$externalLabels.seed}} sends OutOfOrder logs to the Vali. These logs will be dropped.",
								"summary":     "Fluent-bit sends OoO logs",
							},
						},
						{
							Alert: "FluentBitGardenerValiPluginErrors",
							Expr:  intstr.FromString(`sum by (pod) (increase(fluentbit_vali_gardener_errors_total[4m])) > 0`),
							Labels: map[string]string{
								"service":    "logging",
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "There are errors in the {{$labels.pod}} GardenerVali plugin on seed: {{$externalLabels.seed}}.",
								"summary":     "Errors in Fluent-bit GardenerVali plugin",
							},
						},
					},
				}},
			},
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		component = New(c, namespace, values)
		contains = NewManagedResourceContainsObjectsMatcher(c)
	})

	JustBeforeEach(func() {
		customResourcesManagedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fluent-bit",
				Namespace: namespace,
			},
		}
		customResourcesManagedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + customResourcesManagedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(BeNotFoundError())

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fluent-bit",
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
			Expect(customResourcesManagedResource).To(contains(
				serviceMonitor,
				serviceMonitorPlugin,
				prometheusRule,
			))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(Succeed())
			manifests, err := test.ExtractManifestsFromManagedResourceData(customResourcesManagedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())
			Expect(manifests).To(HaveLen(12))
			Expect(customResourcesManagedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(customResourcesManagedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(customResourcesManagedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			test.ExpectKindWithNameAndNamespace(manifests, "ConfigMap", "fluent-bit-lua-config", namespace)
			test.ExpectKindWithNameAndNamespace(manifests, "FluentBit", "fluent-bit-8259c", namespace)
			test.ExpectKindWithNameAndNamespace(manifests, "ClusterFluentBitConfig", "fluent-bit-config", "")
			test.ExpectKindWithNameAndNamespace(manifests, "ClusterInput", "tail-kubernetes", "")
			test.ExpectKindWithNameAndNamespace(manifests, "ClusterFilter", "02-containerd", "")
			test.ExpectKindWithNameAndNamespace(manifests, "ClusterFilter", "03-add-tag-to-record", "")
			test.ExpectKindWithNameAndNamespace(manifests, "ClusterFilter", "zz-modify-severity", "")
			test.ExpectKindWithNameAndNamespace(manifests, "ClusterParser", "containerd-parser", "")
			test.ExpectKindWithNameAndNamespace(manifests, "ClusterOutput", "journald", "")

			componenttest.PrometheusRule(prometheusRule, "testdata/fluent-bit.prometheusrule.test.yaml")
		})

		Context("with vali disabled", func() {
			JustBeforeEach(func() {
				values.ValiEnabled = false
			})
			It("should not deploy vali ClusterOutputs", func() {
				Expect(component.Deploy(ctx)).To(Succeed())
				Expect(customResourcesManagedResourceSecret.Data).NotTo(HaveKey("clusteroutput____journald.yaml"))
			})

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
