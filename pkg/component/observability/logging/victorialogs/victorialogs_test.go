// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package victorialogs_test

import (
	"context"

	vmv1 "github.com/VictoriaMetrics/operator/api/operator/v1"
	vmv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/logging/victorialogs"
	victorialogsconstants "github.com/gardener/gardener/pkg/component/observability/logging/victorialogs/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("VictoriaLogs", func() {
	var (
		ctx = context.Background()

		namespace = "some-namespace"
		image     = "europe-docker.pkg.dev/gardener-project/releases/some-image:some-tag"
		values    = Values{
			Image: image,
		}

		c         client.Client
		component component.DeployWaiter
		consistOf func(...client.Object) types.GomegaMatcher

		customResourcesManagedResourceName   = "victorialogs"
		customResourcesManagedResource       *resourcesv1alpha1.ManagedResource
		customResourcesManagedResourceSecret *corev1.Secret

		vlSingle       *vmv1.VLSingle
		vpa            *vpaautoscalingv1.VerticalPodAutoscaler
		serviceMonitor *monitoringv1.ServiceMonitor
		prometheusRule *monitoringv1.PrometheusRule
	)

	BeforeEach(func() {
		scheme := kubernetes.SeedScheme
		// Register VictoriaMetrics VLSingle types
		utilruntime.Must(vmv1.AddToScheme(scheme))

		c = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		component = New(c, namespace, values)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)
	})

	JustBeforeEach(func() {
		customResourcesManagedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "victorialogs",
				Namespace: namespace,
			},
		}
		customResourcesManagedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + customResourcesManagedResource.Name,
				Namespace: namespace,
			},
		}

		vlSingle = &vmv1.VLSingle{
			ObjectMeta: metav1.ObjectMeta{
				Name:      victorialogsconstants.VLSingleResourceName,
				Namespace: namespace,
				Labels:    getLabels(),
			},
			Spec: vmv1.VLSingleSpec{
				CommonDefaultableParams: vmv1beta1.CommonDefaultableParams{
					DisableSelfServiceScrape: ptr.To(true),
					UseStrictSecurity:        ptr.To(true),
					UseDefaultResources:      ptr.To(false),
					Image: vmv1beta1.Image{
						Repository: "europe-docker.pkg.dev/gardener-project/releases/some-image",
						Tag:        "some-tag",
					},
					Port: "9428",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("100M"),
						},
					},
				},
				RetentionPeriod: "15d",
				Storage: &corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("30Gi"),
						},
					},
				},
				ServiceSpec: &vmv1beta1.AdditionalServiceSpec{
					EmbeddedObjectMetadata: vmv1beta1.EmbeddedObjectMetadata{
						Name: "logging-vl",
					},
				},
			},
		}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "victorialogs-vpa",
				Namespace: namespace,
				Labels:    getLabels(),
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					Kind:       "Deployment",
					Name:       "vlsingle-" + victorialogsconstants.VLSingleResourceName,
					APIVersion: appsv1.SchemeGroupVersion.String(),
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "vlsingle",
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
						},
					},
				},
			},
		}

		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: monitoringutils.ConfigObjectMeta("victorialogs", namespace, shoot.Label),
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app.kubernetes.io/name":      "vlsingle",
					"app.kubernetes.io/instance":  victorialogsconstants.VLSingleResourceName,
					"app.kubernetes.io/component": "monitoring",
					"managed-by":                  "vm-operator",
				}},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "http",
					RelabelConfigs: []monitoringv1.RelabelConfig{
						{
							Action:      "replace",
							Replacement: ptr.To("victorialogs"),
							TargetLabel: "job",
						},
						{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_service_label_(.+)`,
						},
					},
				}},
			},
		}

		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: monitoringutils.ConfigObjectMeta("victorialogs", namespace, shoot.Label),
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "victorialogs.rules",
					Rules: []monitoringv1.Rule{{
						Alert: "VictoriaLogsDown",
						Expr:  intstr.FromString(`absent(up{job="victorialogs"} == 1)`),
						For:   ptr.To(monitoringv1.Duration("30m")),
						Labels: map[string]string{
							"service":    "logging",
							"severity":   "warning",
							"type":       "seed",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"description": "There are no VictoriaLogs pods running on seed: {{ .ExternalLabels.seed }}. No logs will be collected.",
							"summary":     "VictoriaLogs is down",
						},
					}},
				}},
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
					Name:      "victorialogs",
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
				vlSingle,
				vpa,
				serviceMonitor,
				prometheusRule,
			))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(Succeed())
			Expect(customResourcesManagedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(customResourcesManagedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(customResourcesManagedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
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

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelRole:                            v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole:                           v1beta1constants.GardenRoleObservability,
		gardenerutils.NetworkPolicyLabel("logging-vl", 9428):  v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelObservabilityApplication:        "victorialogs",
	}
}
