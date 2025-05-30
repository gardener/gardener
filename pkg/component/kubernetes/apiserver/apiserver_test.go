// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver_test

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/apiserver"
	. "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVar(&secretsutils.GenerateRandomString, secretsutils.FakeGenerateRandomString))
	DeferCleanup(test.WithVar(&secretsutils.GenerateKey, secretsutils.FakeGenerateKey))
	DeferCleanup(test.WithVar(&secretsutils.Read, func(b []byte) (int, error) {
		copy(b, []byte(strings.Repeat("_", len(b))))
		return len(b), nil
	}))
	DeferCleanup(test.WithVar(&secretsutils.GenerateVPNKey, secretsutils.FakeGenerateVPNKey))
	DeferCleanup(test.WithVar(&secretsutils.Clock, testclock.NewFakeClock(time.Time{})))
})

var _ = Describe("KubeAPIServer", func() {
	var (
		ctx = context.Background()

		namespace         = "some-namespace"
		priorityClassName = "some-priority-class"

		kubernetesInterface kubernetes.Interface
		c                   client.Client
		sm                  secretsmanager.Interface
		kapi                Interface
		version             *semver.Version
		runtimeVersion      *semver.Version
		autoscalingConfig   AutoscalingConfig
		namePrefix          string
		consistOf           func(...client.Object) types.GomegaMatcher

		secretNameStaticToken             = "kube-apiserver-static-token-53d619b2"
		secretNameCA                      = "ca"
		secretNameCAClient                = "ca-client"
		secretNameCAEtcd                  = "ca-etcd"
		secretNameCAFrontProxy            = "ca-front-proxy"
		secretNameCAKubelet               = "ca-kubelet"
		secretNameCAVPN                   = "ca-vpn"
		secretNameEtcd                    = "etcd-client"
		secretNameHTTPProxy               = "kube-apiserver-http-proxy"
		secretNameKubeAggregator          = "kube-aggregator"
		secretNameKubeAPIServerToKubelet  = "kube-apiserver-kubelet"
		secretNameServer                  = "kube-apiserver"
		secretNameServiceAccountKey       = "service-account-key-c37a87f6"
		secretNameServiceAccountKeyBundle = "service-account-key-bundle"
		secretNameVPNSeedClient           = "vpn-seed-client"
		secretNameVPNSeedServerTLSAuth    = "vpn-seed-server-tlsauth-a1d0aa00"

		configMapNameAdmissionConfigs  = "kube-apiserver-admission-config-e38ff146"
		secretNameAdmissionKubeconfigs = "kube-apiserver-admission-kubeconfigs-e3b0c442"
		secretNameETCDEncryptionConfig = "kube-apiserver-etcd-encryption-configuration-b2b49c90"
		configMapNameAuditPolicy       = "audit-policy-config-f5b578b4"
		configMapNameEgressPolicy      = "kube-apiserver-egress-selector-config-53d92abc"

		deployment                     *appsv1.Deployment
		horizontalPodAutoscaler        *autoscalingv2.HorizontalPodAutoscaler
		verticalPodAutoscaler          *vpaautoscalingv1.VerticalPodAutoscaler
		podDisruptionBudget            *policyv1.PodDisruptionBudget
		serviceMonitor                 *monitoringv1.ServiceMonitor
		prometheusRule                 *monitoringv1.PrometheusRule
		configMapAdmission             *corev1.ConfigMap
		secretAdmissionKubeconfigs     *corev1.Secret
		configMapAuditPolicy           *corev1.ConfigMap
		configMapAuthentication        *corev1.ConfigMap
		configMapAuthorization         *corev1.ConfigMap
		secretAuthorizationKubeconfigs *corev1.Secret
		configMapEgressSelector        *corev1.ConfigMap
		managedResource                *resourcesv1alpha1.ManagedResource

		values Values
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		version = semver.MustParse("1.28.1")
		runtimeVersion = semver.MustParse("1.28.1")
		namePrefix = ""
	})

	JustBeforeEach(func() {
		values = Values{
			Values: apiserver.Values{
				ETCDEncryption: apiserver.ETCDEncryptionConfig{ResourcesToEncrypt: []string{"secrets"}},
				RuntimeVersion: runtimeVersion,
			},
			Autoscaling:       autoscalingConfig,
			PriorityClassName: priorityClassName,
			NamePrefix:        namePrefix,
			Version:           version,
			VPN:               VPNConfig{Enabled: true},
		}
		kubernetesInterface = fakekubernetes.NewClientSetBuilder().WithAPIReader(c).WithClient(c).Build()
		kapi = New(kubernetesInterface, namespace, sm, values)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-client", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-front-proxy", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-kubelet", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-vpn", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "etcd-client", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "service-account-key-bundle", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: vpnseedserver.SecretNameTLSAuth, Namespace: namespace}})).To(Succeed())

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
			},
		}
		horizontalPodAutoscaler = &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
			},
		}
		verticalPodAutoscaler = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver-vpa",
				Namespace: namespace,
			},
		}
		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
			},
		}
		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
		}
		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-kube-apiserver",
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		Describe("HorizontalPodAutoscaler", func() {
			DescribeTable("should delete the HPA resource",
				func(autoscalingConfig AutoscalingConfig) {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{

							RuntimeVersion: runtimeVersion,
						},
						Autoscaling: autoscalingConfig,
						Version:     version},
					)

					Expect(c.Create(ctx, horizontalPodAutoscaler)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(Succeed())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(BeNotFoundError())
				},

				Entry("replicas is nil", AutoscalingConfig{Replicas: nil}),
				Entry("replicas is 0", AutoscalingConfig{Replicas: ptr.To[int32](0)}),
			)

			It("should successfully deploy the HPA resource", func() {
				autoscalingConfig := AutoscalingConfig{
					Replicas:    ptr.To[int32](2),
					MinReplicas: 4,
					MaxReplicas: 6,
				}
				metrics := []autoscalingv2.MetricSpec{
					{
						Type: "Resource",
						Resource: &autoscalingv2.ResourceMetricSource{
							Name: "cpu",
							Target: autoscalingv2.MetricTarget{
								Type:         autoscalingv2.AverageValueMetricType,
								AverageValue: ptr.To(resource.MustParse("6")),
							},
						},
					},
					{
						Type: "Resource",
						Resource: &autoscalingv2.ResourceMetricSource{
							Name: "memory",
							Target: autoscalingv2.MetricTarget{
								Type:         autoscalingv2.AverageValueMetricType,
								AverageValue: ptr.To(resource.MustParse("24G")),
							},
						},
					},
				}
				hpaBehaviour := &autoscalingv2.HorizontalPodAutoscalerBehavior{
					ScaleUp: &autoscalingv2.HPAScalingRules{
						StabilizationWindowSeconds: ptr.To[int32](60),
						Policies: []autoscalingv2.HPAScalingPolicy{
							{
								Type:          autoscalingv2.PercentScalingPolicy,
								Value:         100,
								PeriodSeconds: 60,
							},
						},
					},
					ScaleDown: &autoscalingv2.HPAScalingRules{
						StabilizationWindowSeconds: ptr.To[int32](1800),
						Policies: []autoscalingv2.HPAScalingPolicy{
							{
								Type:          autoscalingv2.PodsScalingPolicy,
								Value:         1,
								PeriodSeconds: 300,
							},
						},
					},
				}

				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
					},
					Autoscaling: autoscalingConfig,
					Version:     version},
				)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(BeNotFoundError())
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(Succeed())
				Expect(horizontalPodAutoscaler).To(DeepEqual(&autoscalingv2.HorizontalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      horizontalPodAutoscaler.Name,
						Namespace: horizontalPodAutoscaler.Namespace,
						Labels: map[string]string{
							"high-availability-config.resources.gardener.cloud/type": "server",
						},
						ResourceVersion: "1",
					},
					Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
						MinReplicas: &autoscalingConfig.MinReplicas,
						MaxReplicas: autoscalingConfig.MaxReplicas,
						ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "kube-apiserver",
						},
						Metrics:  metrics,
						Behavior: hpaBehaviour,
					},
				}))
			})
		})

		Describe("VerticalPodAutoscaler", func() {
			DescribeTable("should successfully deploy the VPA resource",
				func(autoscalingConfig AutoscalingConfig, haVPN bool, annotations, labels map[string]string, containerPolicies []vpaautoscalingv1.ContainerResourcePolicy) {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Autoscaling: autoscalingConfig,
						Version:     version,
						VPN: VPNConfig{
							HighAvailabilityEnabled:             haVPN,
							HighAvailabilityNumberOfSeedServers: 2,
						},
					})

					Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(Succeed())
					Expect(verticalPodAutoscaler).To(DeepEqual(&vpaautoscalingv1.VerticalPodAutoscaler{
						ObjectMeta: metav1.ObjectMeta{
							Name:            verticalPodAutoscaler.Name,
							Namespace:       verticalPodAutoscaler.Namespace,
							Annotations:     annotations,
							Labels:          labels,
							ResourceVersion: "1",
						},
						Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
							TargetRef: &autoscalingv1.CrossVersionObjectReference{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "kube-apiserver",
							},
							UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
								UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
							},
							ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
								ContainerPolicies: containerPolicies,
							},
						},
					}))
				},
				Entry("default behaviour",
					AutoscalingConfig{},
					false,
					nil,
					nil,
					[]vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "kube-apiserver",
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("20m"),
								corev1.ResourceMemory: resource.MustParse("200M"),
							},
						},
					},
				),
				Entry("HA VPN is enabled",
					AutoscalingConfig{},
					true,
					nil,
					nil,
					[]vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "kube-apiserver",
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("20m"),
								corev1.ResourceMemory: resource.MustParse("200M"),
							},
						},
						{
							ContainerName: "vpn-client-0",
							Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
						},
						{
							ContainerName: "vpn-client-1",
							Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
						},
						{
							ContainerName: "vpn-path-controller",
							Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
						},
					},
				),
				Entry("scale-down is disabled",
					AutoscalingConfig{ScaleDownDisabled: true},
					false,
					map[string]string{"eviction-requirements.autoscaling.gardener.cloud/downscale-restriction": "never"},
					map[string]string{"autoscaling.gardener.cloud/eviction-requirements": "managed-by-controller"},
					[]vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "kube-apiserver",
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("20m"),
								corev1.ResourceMemory: resource.MustParse("200M"),
							},
						},
					},
				),
				Entry("minAllowed configured",
					AutoscalingConfig{MinAllowed: corev1.ResourceList{"memory": resource.MustParse("2Gi")}},
					false,
					nil,
					nil,
					[]vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName:    "kube-apiserver",
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("20m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
					},
				),
			)
		})

		Describe("PodDisruptionBudget", func() {
			It("should successfully deploy the PDB resource", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(BeNotFoundError())
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(Succeed())

				Expect(podDisruptionBudget).To(DeepEqual(&policyv1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name:            podDisruptionBudget.Name,
						Namespace:       podDisruptionBudget.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"app":  "kubernetes",
							"role": "apiserver",
						},
					},
					Spec: policyv1.PodDisruptionBudgetSpec{
						MaxUnavailable: ptr.To(intstr.FromInt32(1)),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app":  "kubernetes",
								"role": "apiserver",
							},
						},
						UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
					},
				}))
			})
		})

		Describe("ServiceMonitor", func() {
			var (
				prometheusName         string
				expectedServiceMonitor *monitoringv1.ServiceMonitor
			)

			BeforeEach(func() {
				prometheusName = ""
			})

			JustBeforeEach(func() {
				name := "garden-virtual-garden-kube-apiserver"
				if prometheusName == "shoot" {
					name = "shoot-kube-apiserver"
				}

				expectedServiceMonitor = &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:            name,
						Namespace:       serviceMonitor.Namespace,
						ResourceVersion: "1",
						Labels:          map[string]string{"prometheus": prometheusName},
					},
					Spec: monitoringv1.ServiceMonitorSpec{
						Selector: metav1.LabelSelector{MatchLabels: map[string]string{
							"app":  "kubernetes",
							"role": "apiserver",
						}},
						Endpoints: []monitoringv1.Endpoint{{
							TargetPort: ptr.To(intstr.FromInt32(443)),
							Scheme:     "https",
							TLSConfig:  &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)}},
							Authorization: &monitoringv1.SafeAuthorization{Credentials: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "shoot-access-prometheus-" + prometheusName},
								Key:                  "token",
							}},
							RelabelConfigs: []monitoringv1.RelabelConfig{{
								Action: "labelmap",
								Regex:  `__meta_kubernetes_service_label_(.+)`,
							}},
							MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
								SourceLabels: []monitoringv1.LabelName{"__name__"},
								Action:       "keep",
								Regex:        `^(authentication_attempts|authenticated_user_requests|apiserver_admission_controller_admission_duration_seconds_.+|apiserver_admission_webhook_admission_duration_seconds_.+|apiserver_admission_step_admission_duration_seconds_.+|apiserver_admission_webhook_request_total|apiserver_admission_webhook_rejection_count|apiserver_audit_event_total|apiserver_audit_error_total|apiserver_audit_requests_rejected_total|apiserver_cache_list_.+|apiserver_crd_webhook_conversion_duration_seconds_.+|apiserver_current_inflight_requests|apiserver_current_inqueue_requests|apiserver_init_events_total|apiserver_latency|apiserver_latency_seconds|apiserver_longrunning_requests|apiserver_request_duration_seconds_.+|apiserver_request_duration_seconds_bucket|apiserver_request_duration_seconds_count|apiserver_request_terminations_total|apiserver_response_sizes_.+|apiserver_storage_db_total_size_in_bytes|apiserver_storage_list_.+|apiserver_storage_objects|apiserver_storage_transformation_duration_seconds_.+|apiserver_storage_transformation_operations_total|apiserver_storage_size_bytes|apiserver_registered_watchers|apiserver_request_count|apiserver_request_total|apiserver_watch_duration|apiserver_watch_events_sizes_.+|apiserver_watch_events_total|etcd_request_duration_seconds_.+|go_.+|process_max_fds|process_open_fds|watch_cache_capacity_increase_total|watch_cache_capacity_decrease_total|watch_cache_capacity)$`,
							}},
						}},
					},
				}
			})

			When("name prefix is provided", func() {
				BeforeEach(func() {
					namePrefix = "virtual-garden-"
					prometheusName = "garden"
				})

				It("should successfully deploy the resource", func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedServiceMonitor), serviceMonitor)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedServiceMonitor), serviceMonitor)).To(Succeed())
					Expect(serviceMonitor).To(DeepEqual(expectedServiceMonitor))
				})
			})

			When("name prefix is not provided", func() {
				BeforeEach(func() {
					prometheusName = "shoot"
				})

				It("should successfully deploy the resource", func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedServiceMonitor), serviceMonitor)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedServiceMonitor), serviceMonitor)).To(Succeed())
					Expect(serviceMonitor).To(DeepEqual(expectedServiceMonitor))
				})
			})
		})

		Describe("PrometheusRule", func() {
			var (
				prometheusName         string
				expectedPrometheusRule *monitoringv1.PrometheusRule
			)

			BeforeEach(func() {
				prometheusName = ""
			})

			JustBeforeEach(func() {
				name := "garden-virtual-garden-kube-apiserver"
				if prometheusName == "shoot" {
					name = "shoot-kube-apiserver"
				}

				expectedPrometheusRule = &monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:            name,
						Namespace:       serviceMonitor.Namespace,
						ResourceVersion: "1",
						Labels:          map[string]string{"prometheus": prometheusName},
					},
					Spec: monitoringv1.PrometheusRuleSpec{
						Groups: []monitoringv1.RuleGroup{{
							Name: "kube-apiserver.rules",
							Rules: []monitoringv1.Rule{
								{
									Alert: "ApiServerNotReachable",
									Expr:  intstr.FromString(`probe_success{job="blackbox-apiserver"} == 0`),
									For:   ptr.To(monitoringv1.Duration("5m")),
									Labels: map[string]string{
										"service":    "kube-apiserver",
										"severity":   "blocker",
										"type":       "seed",
										"visibility": "all",
									},
									Annotations: map[string]string{
										"summary":     "API server not reachable (externally).",
										"description": "API server not reachable via external endpoint: {{ $labels.instance }}.",
									},
								},
								{
									Alert: "KubeApiserverDown",
									Expr:  intstr.FromString(`absent(up{job="kube-apiserver"} == 1)`),
									For:   ptr.To(monitoringv1.Duration("5m")),
									Labels: map[string]string{
										"service":    "kube-apiserver",
										"severity":   "blocker",
										"type":       "seed",
										"visibility": "operator",
									},
									Annotations: map[string]string{
										"summary":     "API server unreachable.",
										"description": "All API server replicas are down/unreachable, or all API server could not be found.",
									},
								},
								{
									Alert: "KubeApiServerTooManyOpenFileDescriptors",
									Expr:  intstr.FromString(`100 * process_open_fds{job="kube-apiserver"} / process_max_fds > 50`),
									For:   ptr.To(monitoringv1.Duration("30m")),
									Labels: map[string]string{
										"service":    "kube-apiserver",
										"severity":   "warning",
										"type":       "seed",
										"visibility": "owner",
									},
									Annotations: map[string]string{
										"summary":     "The API server has too many open file descriptors",
										"description": "The API server ({{ $labels.instance }}) is using {{ $value }}% of the available file/socket descriptors.",
									},
								},
								{
									Alert: "KubeApiServerTooManyOpenFileDescriptors",
									Expr:  intstr.FromString(`100 * process_open_fds{job="kube-apiserver"} / process_max_fds{job="kube-apiserver"} > 80`),
									For:   ptr.To(monitoringv1.Duration("30m")),
									Labels: map[string]string{
										"service":    "kube-apiserver",
										"severity":   "critical",
										"type":       "seed",
										"visibility": "owner",
									},
									Annotations: map[string]string{
										"summary":     "The API server has too many open file descriptors",
										"description": "The API server ({{ $labels.instance }}) is using {{ $value }}% of the available file/socket descriptors.",
									},
								},
								{
									Alert: "KubeApiServerLatency",
									// Some verbs excluded because they are expected to be long-lasting:
									// - WATCHLIST is long-poll
									// - CONNECT is "kubectl exec"
									Expr: intstr.FromString(`histogram_quantile(0.99, sum without (instance,resource,subresource) (rate(apiserver_request_duration_seconds_bucket{subresource!~"log|portforward|exec|proxy|attach",verb!~"CONNECT|WATCHLIST|WATCH|PROXY proxy"}[5m]))) > 3`),
									For:  ptr.To(monitoringv1.Duration("30m")),
									Labels: map[string]string{
										"service":    "kube-apiserver",
										"severity":   "warning",
										"type":       "seed",
										"visibility": "owner",
									},
									Annotations: map[string]string{
										"summary":     "Kubernetes API server latency is high",
										"description": "Kube API server latency for verb {{ $labels.verb }} is high. This could be because the shoot workers and the control plane are in different regions. 99th percentile of request latency is greater than 3 seconds.",
									},
								},
								{
									Record: "shoot:apiserver_watch_duration:quantile",
									Expr:   intstr.FromString(`histogram_quantile(0.2, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))`),
									Labels: map[string]string{"quantile": "0.2"},
								},
								{
									Record: "shoot:apiserver_watch_duration:quantile",
									Expr:   intstr.FromString(`histogram_quantile(0.5, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))`),
									Labels: map[string]string{"quantile": "0.5"},
								},
								{
									Record: "shoot:apiserver_watch_duration:quantile",
									Expr:   intstr.FromString(`histogram_quantile(0.9, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))`),
									Labels: map[string]string{"quantile": "0.9"},
								},
								{
									Record: "shoot:apiserver_watch_duration:quantile",
									Expr:   intstr.FromString(`histogram_quantile(0.2, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))`),
									Labels: map[string]string{"quantile": "0.2"},
								},
								{
									Record: "shoot:apiserver_watch_duration:quantile",
									Expr:   intstr.FromString(`histogram_quantile(0.5, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))`),
									Labels: map[string]string{"quantile": "0.5"},
								},
								{
									Record: "shoot:apiserver_watch_duration:quantile",
									Expr:   intstr.FromString(`histogram_quantile(0.9, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))`),
									Labels: map[string]string{"quantile": "0.9"},
								},

								// API Auditlog
								{
									Alert: "KubeApiServerTooManyAuditlogFailures",
									Expr:  intstr.FromString(`sum(rate (apiserver_audit_error_total{plugin!="log",job="kube-apiserver"}[5m])) / sum(rate(apiserver_audit_event_total{job="kube-apiserver"}[5m])) > bool 0.02 == 1`),
									For:   ptr.To(monitoringv1.Duration("15m")),
									Labels: map[string]string{
										"service":    "auditlog",
										"severity":   "warning",
										"type":       "seed",
										"visibility": "operator",
									},
									Annotations: map[string]string{
										"summary":     "The kubernetes API server has too many failed attempts to log audit events",
										"description": "The API servers cumulative failure rate in logging audit events is greater than 2%.",
									},
								},
								{
									Record: "shoot:apiserver_audit_event_total:sum",
									Expr:   intstr.FromString(`sum(rate(apiserver_audit_event_total{job="kube-apiserver"}[5m]))`),
								},
								{
									Record: "shoot:apiserver_audit_error_total:sum",
									Expr:   intstr.FromString(`sum(rate(apiserver_audit_error_total{plugin!="log",job="kube-apiserver"}[5m]))`),
								},

								// API latency
								{
									Record: "shoot:apiserver_latency_seconds:quantile",
									Expr:   intstr.FromString(`histogram_quantile(0.99, sum without (instance, pod) (rate(apiserver_request_duration_seconds_bucket[5m])))`),
									Labels: map[string]string{"quantile": "0.99"},
								},
								{
									Record: "shoot:apiserver_latency_seconds:quantile",
									Expr:   intstr.FromString(`histogram_quantile(0.9, sum without (instance, pod) (rate(apiserver_request_duration_seconds_bucket[5m])))`),
									Labels: map[string]string{"quantile": "0.9"},
								},
								{
									Record: "shoot:apiserver_latency_seconds:quantile",
									Expr:   intstr.FromString(`histogram_quantile(0.5, sum without (instance, pod) (rate(apiserver_request_duration_seconds_bucket[5m])))`),
									Labels: map[string]string{"quantile": "0.5"},
								},

								// API server request duration greater than 1s percentage
								{
									Record: "shoot:apiserver_latency:percentage",
									Expr:   intstr.FromString(`1 - sum(rate(apiserver_request_duration_seconds_bucket{le="1.0",subresource!~"log|portforward|exec|proxy|attach",verb!~"CONNECT|LIST|WATCH"}[1h])) / sum(rate(apiserver_request_duration_seconds_count{subresource!~"log|portforward|exec|proxy|attach",verb!~"CONNECT|LIST|WATCH"}[1h]))`),
								},

								{
									Record: "shoot:kube_apiserver:sum_by_pod",
									Expr:   intstr.FromString(`sum(up{job="kube-apiserver"}) by (pod)`),
								},

								// API failure rate
								{
									Alert: "ApiserverRequestsFailureRate",
									Expr:  intstr.FromString(`max(sum by(instance,resource,verb) (rate(apiserver_request_total{code=~"5.."}[10m])) / sum by(instance,resource,verb) (rate(apiserver_request_total[10m]))) * 100 > 10`),
									For:   ptr.To(monitoringv1.Duration("30m")),
									Labels: map[string]string{
										"service":    "kube-apiserver",
										"severity":   "warning",
										"type":       "seed",
										"visibility": "operator",
									},
									Annotations: map[string]string{
										"summary":     "Kubernetes API server failure rate is high",
										"description": "The API Server requests failure rate exceeds 10%.",
									},
								},
							},
						}},
					},
				}
			})

			When("name prefix is provided", func() {
				BeforeEach(func() {
					namePrefix = "virtual-garden-"
					prometheusName = "garden"
				})

				It("should not deploy the resource at all", func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedPrometheusRule), prometheusRule)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedPrometheusRule), prometheusRule)).To(BeNotFoundError())
				})
			})

			When("name prefix is not provided", func() {
				BeforeEach(func() {
					prometheusName = "shoot"
				})

				It("should successfully deploy the resource", func() {
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedPrometheusRule), prometheusRule)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedPrometheusRule), prometheusRule)).To(Succeed())
					Expect(prometheusRule).To(DeepEqual(expectedPrometheusRule))

					componenttest.PrometheusRule(prometheusRule, "testdata/shoot-kube-apiserver.prometheusrule.test.yaml")
				})
			})
		})

		Describe("Shoot Resources", func() {
			It("should successfully deploy the managed resource and its secret", func() {
				var (
					clusterRole = &rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{
							Name: "system:apiserver:kubelet",
						},
						Rules: []rbacv1.PolicyRule{
							{
								APIGroups: []string{""},
								Resources: []string{
									"nodes/proxy",
									"nodes/stats",
									"nodes/log",
									"nodes/spec",
									"nodes/metrics",
								},
								Verbs: []string{"create", "get", "update", "patch", "delete"},
							},
							{
								NonResourceURLs: []string{"*"},
								Verbs:           []string{"create", "get", "update", "patch", "delete"},
							},
						},
					}

					clusterRoleBinding = &rbacv1.ClusterRoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: "system:apiserver:kubelet",
							Annotations: map[string]string{
								"resources.gardener.cloud/delete-on-invalid-update": "true",
							},
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: "rbac.authorization.k8s.io",
							Kind:     "ClusterRole",
							Name:     "system:apiserver:kubelet",
						},
						Subjects: []rbacv1.Subject{
							{
								Kind: "User",
								Name: "system:kube-apiserver:kubelet",
							},
						},
					}
				)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"origin": "gardener",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						KeepObjects:  ptr.To(false),
						SecretRefs:   []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}},
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(DeepEqual(expectedMr))
				Expect(managedResource).To(consistOf(clusterRole, clusterRoleBinding))
			})
		})

		Describe("Secrets", func() {
			Context("admission kubeconfigs", func() {
				It("should successfully deploy the secret resource w/o admission plugin kubeconfigs", func() {
					secretAdmissionKubeconfigs = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-admission-kubeconfigs", Namespace: namespace},
						Data:       map[string][]byte{},
					}
					Expect(kubernetesutils.MakeUnique(secretAdmissionKubeconfigs)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(Succeed())
					Expect(secretAdmissionKubeconfigs).To(DeepEqual(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:            secretAdmissionKubeconfigs.Name,
							Namespace:       secretAdmissionKubeconfigs.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      secretAdmissionKubeconfigs.Data,
					}))
				})

				It("should successfully deploy the secret resource w/ admission plugins", func() {
					admissionPlugins := []apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz"}, Kubeconfig: []byte("foo")},
					}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							EnabledAdmissionPlugins: admissionPlugins,
							RuntimeVersion:          runtimeVersion,
						},
						Version: version,
					})

					secretAdmissionKubeconfigs = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-admission-kubeconfigs", Namespace: namespace},
						Data: map[string][]byte{
							"baz-kubeconfig.yaml": []byte("foo"),
						},
					}
					Expect(kubernetesutils.MakeUnique(secretAdmissionKubeconfigs)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretAdmissionKubeconfigs), secretAdmissionKubeconfigs)).To(Succeed())
					Expect(secretAdmissionKubeconfigs).To(DeepEqual(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:            secretAdmissionKubeconfigs.Name,
							Namespace:       secretAdmissionKubeconfigs.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      secretAdmissionKubeconfigs.Data,
					}))
				})
			})

			It("should successfully deploy the OIDCCABundle secret resource", func() {
				var (
					caBundle   = "some-ca-bundle"
					oidcConfig = &gardencorev1beta1.OIDCConfig{CABundle: &caBundle}
				)

				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
					},
					OIDC:    oidcConfig,
					Version: version,
				})

				expectedSecretOIDCCABundle := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-oidc-cabundle", Namespace: namespace},
					Data:       map[string][]byte{"ca.crt": []byte(caBundle)},
				}
				Expect(kubernetesutils.MakeUnique(expectedSecretOIDCCABundle)).To(Succeed())

				actualSecretOIDCCABundle := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretOIDCCABundle), actualSecretOIDCCABundle)).To(BeNotFoundError())

				Expect(kapi.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretOIDCCABundle), actualSecretOIDCCABundle)).To(Succeed())
				Expect(actualSecretOIDCCABundle).To(DeepEqual(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:            expectedSecretOIDCCABundle.Name,
						Namespace:       expectedSecretOIDCCABundle.Namespace,
						Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
						ResourceVersion: "1",
					},
					Immutable: ptr.To(true),
					Data:      expectedSecretOIDCCABundle.Data,
				}))
			})

			It("should not deploy the OIDCCABundle secret resource when version is >= v1.30 and feature gate is not set", func() {
				var (
					caBundle   = "some-ca-bundle"
					clientID   = "some-client-id"
					issuerURL  = "https://issuer.url.com"
					version    = semver.MustParse("1.30.0")
					oidcConfig = &gardencorev1beta1.OIDCConfig{
						IssuerURL: &issuerURL,
						ClientID:  &clientID,
						CABundle:  &caBundle,
					}
				)

				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
					},
					OIDC:    oidcConfig,
					Version: version,
				})

				expectedSecretOIDCCABundle := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-oidc-cabundle", Namespace: namespace},
					Data:       map[string][]byte{"ca.crt": []byte(caBundle)},
				}
				Expect(kubernetesutils.MakeUnique(expectedSecretOIDCCABundle)).To(Succeed())

				actualSecretOIDCCABundle := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretOIDCCABundle), actualSecretOIDCCABundle)).To(BeNotFoundError())
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretOIDCCABundle), actualSecretOIDCCABundle)).To(BeNotFoundError())
			})

			It("should not deploy the OIDCCABundle secret resource when version is >= v1.30 and feature gate is set to true", func() {
				var (
					caBundle   = "some-ca-bundle"
					clientID   = "some-client-id"
					issuerURL  = "https://issuer.url.com"
					version    = semver.MustParse("1.30.0")
					oidcConfig = &gardencorev1beta1.OIDCConfig{
						IssuerURL: &issuerURL,
						ClientID:  &clientID,
						CABundle:  &caBundle,
					}
				)

				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
						FeatureGates: map[string]bool{
							"StructuredAuthenticationConfiguration": true,
						},
					},
					OIDC:    oidcConfig,
					Version: version,
				})

				expectedSecretOIDCCABundle := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-oidc-cabundle", Namespace: namespace},
					Data:       map[string][]byte{"ca.crt": []byte(caBundle)},
				}
				Expect(kubernetesutils.MakeUnique(expectedSecretOIDCCABundle)).To(Succeed())

				actualSecretOIDCCABundle := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretOIDCCABundle), actualSecretOIDCCABundle)).To(BeNotFoundError())
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretOIDCCABundle), actualSecretOIDCCABundle)).To(BeNotFoundError())
			})

			It("should successfully deploy the OIDCCABundle secret resource when version is >= v1.30 and feature gate is set to false", func() {
				var (
					caBundle   = "some-ca-bundle"
					version    = semver.MustParse("1.30.0")
					oidcConfig = &gardencorev1beta1.OIDCConfig{CABundle: &caBundle}
				)

				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
						FeatureGates: map[string]bool{
							"StructuredAuthenticationConfiguration": false,
						},
					},
					OIDC:    oidcConfig,
					Version: version,
				})

				expectedSecretOIDCCABundle := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-oidc-cabundle", Namespace: namespace},
					Data:       map[string][]byte{"ca.crt": []byte(caBundle)},
				}
				Expect(kubernetesutils.MakeUnique(expectedSecretOIDCCABundle)).To(Succeed())

				actualSecretOIDCCABundle := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretOIDCCABundle), actualSecretOIDCCABundle)).To(BeNotFoundError())

				Expect(kapi.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretOIDCCABundle), actualSecretOIDCCABundle)).To(Succeed())
				Expect(actualSecretOIDCCABundle).To(DeepEqual(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:            expectedSecretOIDCCABundle.Name,
						Namespace:       expectedSecretOIDCCABundle.Namespace,
						Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
						ResourceVersion: "1",
					},
					Immutable: ptr.To(true),
					Data:      expectedSecretOIDCCABundle.Data,
				}))
			})

			It("should successfully deploy the ETCD encryption configuration secret resource", func() {
				etcdEncryptionConfiguration := `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:
      - name: key-62135596800
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=
  - identity: {}
  resources:
  - secrets
`

				By("Verify encryption config secret")
				expectedSecretETCDEncryptionConfiguration := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-etcd-encryption-configuration", Namespace: namespace},
					Data:       map[string][]byte{"encryption-configuration.yaml": []byte(etcdEncryptionConfiguration)},
				}
				Expect(kubernetesutils.MakeUnique(expectedSecretETCDEncryptionConfiguration)).To(Succeed())

				actualSecretETCDEncryptionConfiguration := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(BeNotFoundError())

				Expect(kapi.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
				Expect(actualSecretETCDEncryptionConfiguration).To(Equal(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      expectedSecretETCDEncryptionConfiguration.Name,
						Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
						Labels: map[string]string{
							"resources.gardener.cloud/garbage-collectable-reference": "true",
							"role": "kube-apiserver-etcd-encryption-configuration",
						},
						ResourceVersion: "1",
					},
					Immutable: ptr.To(true),
					Data:      expectedSecretETCDEncryptionConfiguration.Data,
				}))

				By("Deploy again and ensure that labels are still present")
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
				Expect(actualSecretETCDEncryptionConfiguration.Labels).To(Equal(map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
					"role": "kube-apiserver-etcd-encryption-configuration",
				}))

				By("Verify encryption key secret")
				secretList := &corev1.SecretList{}
				Expect(c.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
					"name":       "kube-apiserver-etcd-encryption-key",
					"managed-by": "secrets-manager",
				})).To(Succeed())
				Expect(secretList.Items).To(HaveLen(1))
				Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
			})

			DescribeTable("successfully deploy the ETCD encryption configuration secret resource w/ old key",
				func(encryptWithCurrentKey bool) {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							ETCDEncryption: apiserver.ETCDEncryptionConfig{EncryptWithCurrentKey: encryptWithCurrentKey, ResourcesToEncrypt: []string{"secrets"}},
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
					})

					oldKeyName, oldKeySecret := "key-old", "old-secret"
					Expect(c.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver-etcd-encryption-key-old",
							Namespace: namespace,
						},
						Data: map[string][]byte{
							"key":    []byte(oldKeyName),
							"secret": []byte(oldKeySecret),
						},
					})).To(Succeed())

					etcdEncryptionConfiguration := `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:`

					if encryptWithCurrentKey {
						etcdEncryptionConfiguration += `
      - name: key-62135596800
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=
      - name: ` + oldKeyName + `
        secret: ` + oldKeySecret
					} else {
						etcdEncryptionConfiguration += `
      - name: ` + oldKeyName + `
        secret: ` + oldKeySecret + `
      - name: key-62135596800
        secret: X19fX19fX19fX19fX19fX19fX19fX19fX19fX19fX18=`
					}

					etcdEncryptionConfiguration += `
  - identity: {}
  resources:
  - secrets
`

					expectedSecretETCDEncryptionConfiguration := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-etcd-encryption-configuration", Namespace: namespace},
						Data:       map[string][]byte{"encryption-configuration.yaml": []byte(etcdEncryptionConfiguration)},
					}
					Expect(kubernetesutils.MakeUnique(expectedSecretETCDEncryptionConfiguration)).To(Succeed())

					actualSecretETCDEncryptionConfiguration := &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(BeNotFoundError())

					Expect(kapi.Deploy(ctx)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecretETCDEncryptionConfiguration), actualSecretETCDEncryptionConfiguration)).To(Succeed())
					Expect(actualSecretETCDEncryptionConfiguration).To(DeepEqual(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      expectedSecretETCDEncryptionConfiguration.Name,
							Namespace: expectedSecretETCDEncryptionConfiguration.Namespace,
							Labels: map[string]string{
								"resources.gardener.cloud/garbage-collectable-reference": "true",
								"role": "kube-apiserver-etcd-encryption-configuration",
							},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      expectedSecretETCDEncryptionConfiguration.Data,
					}))

					secretList := &corev1.SecretList{}
					Expect(c.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
						"name":       "kube-apiserver-etcd-encryption-key",
						"managed-by": "secrets-manager",
					})).To(Succeed())
					Expect(secretList.Items).To(HaveLen(1))
					Expect(secretList.Items[0].Labels).To(HaveKeyWithValue("persist", "true"))
				},

				Entry("encrypting with current", true),
				Entry("encrypting with old", false),
			)

			Context("TLS SNI", func() {
				It("should successfully deploy the needed secret resources", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
						SNI: SNIConfig{TLS: []TLSSNIConfig{
							{SecretName: ptr.To("foo")},
							{Certificate: []byte("foo"), PrivateKey: []byte("bar")},
							{SecretName: ptr.To("baz"), Certificate: []byte("foo"), PrivateKey: []byte("bar")},
						}},
					})

					expectedSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-tls-sni-1", Namespace: namespace},
						Data:       map[string][]byte{"tls.crt": []byte("foo"), "tls.key": []byte("bar")},
					}
					Expect(kubernetesutils.MakeUnique(expectedSecret)).To(Succeed())

					actualSecret := &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(BeNotFoundError())

					Expect(kapi.Deploy(ctx)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(Succeed())
					Expect(actualSecret).To(DeepEqual(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:            expectedSecret.Name,
							Namespace:       expectedSecret.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      expectedSecret.Data,
					}))
				})

				It("should return an error for invalid configuration", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
						SNI:     SNIConfig{TLS: []TLSSNIConfig{{}}},
					})

					Expect(kapi.Deploy(ctx)).To(MatchError(ContainSubstring("either the name of an existing secret or both certificate and private key must be provided for TLS SNI config")))
				})
			})

			It("should successfully deploy the audit webhook kubeconfig secret resource", func() {
				var (
					kubeconfig  = []byte("some-kubeconfig")
					auditConfig = &apiserver.AuditConfig{Webhook: &apiserver.AuditWebhook{Kubeconfig: kubeconfig}}
				)

				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						Audit:          auditConfig,
						RuntimeVersion: runtimeVersion,
					},
					Version: version,
				})

				expectedSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-audit-webhook-kubeconfig", Namespace: namespace},
					Data:       map[string][]byte{"kubeconfig.yaml": kubeconfig},
				}
				Expect(kubernetesutils.MakeUnique(expectedSecret)).To(Succeed())

				actualSecret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(BeNotFoundError())

				Expect(kapi.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(Succeed())
				Expect(actualSecret).To(DeepEqual(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:            expectedSecret.Name,
						Namespace:       expectedSecret.Namespace,
						Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
						ResourceVersion: "1",
					},
					Immutable: ptr.To(true),
					Data:      expectedSecret.Data,
				}))
			})

			It("should successfully deploy the authentication webhook kubeconfig secret resource", func() {
				var (
					kubeconfig        = []byte("some-kubeconfig")
					authWebhookConfig = &AuthenticationWebhook{Kubeconfig: kubeconfig}
				)

				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
					},
					AuthenticationWebhook: authWebhookConfig,
					Version:               version,
				})

				expectedSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-webhook-kubeconfig", Namespace: namespace},
					Data:       map[string][]byte{"kubeconfig.yaml": kubeconfig},
				}
				Expect(kubernetesutils.MakeUnique(expectedSecret)).To(Succeed())

				actualSecret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(BeNotFoundError())

				Expect(kapi.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(expectedSecret), actualSecret)).To(Succeed())
				Expect(actualSecret).To(DeepEqual(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:            expectedSecret.Name,
						Namespace:       expectedSecret.Namespace,
						Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
						ResourceVersion: "1",
					},
					Immutable: ptr.To(true),
					Data:      expectedSecret.Data,
				}))
			})

			Context("authorization webhook kubeconfigs", func() {
				It("should not deploy the secret resource when there are no webhook configurations", func() {
					secretAuthorizationKubeconfigs = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authorization-webhooks-kubeconfigs", Namespace: namespace}}
					Expect(kubernetesutils.MakeUnique(secretAuthorizationKubeconfigs)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretAuthorizationKubeconfigs), secretAuthorizationKubeconfigs)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretAuthorizationKubeconfigs), secretAuthorizationKubeconfigs)).To(BeNotFoundError())
				})

				It("should successfully deploy the secret resource w/ authorization webhooks", func() {
					authorizationWebhooks := []AuthorizationWebhook{
						{Name: "foo", Kubeconfig: []byte("bar")},
						{Name: "baz", Kubeconfig: []byte("foo")},
					}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values:                apiserver.Values{RuntimeVersion: runtimeVersion},
						AuthorizationWebhooks: authorizationWebhooks,
						Version:               version,
					})

					secretAuthorizationKubeconfigs = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authorization-webhooks-kubeconfigs", Namespace: namespace},
						Data: map[string][]byte{
							"foo-kubeconfig.yaml": []byte("bar"),
							"baz-kubeconfig.yaml": []byte("foo"),
						},
					}
					Expect(kubernetesutils.MakeUnique(secretAuthorizationKubeconfigs)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretAuthorizationKubeconfigs), secretAuthorizationKubeconfigs)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(secretAuthorizationKubeconfigs), secretAuthorizationKubeconfigs)).To(Succeed())
					Expect(secretAuthorizationKubeconfigs).To(DeepEqual(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:            secretAuthorizationKubeconfigs.Name,
							Namespace:       secretAuthorizationKubeconfigs.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      secretAuthorizationKubeconfigs.Data,
					}))
				})
			})
		})

		Describe("ConfigMaps", func() {
			Context("admission", func() {
				It("should successfully deploy the configmap resource w/o admission plugins", func() {
					configMapAdmission = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-admission-config", Namespace: namespace},
						Data: map[string]string{"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins: null
`},
					}
					Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
					Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAdmission.Name,
							Namespace:       configMapAdmission.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAdmission.Data,
					}))
				})

				It("should successfully deploy the configmap resource w/ admission plugins", func() {
					admissionPlugins := []apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz", Config: &runtime.RawExtension{Raw: []byte("some-config-for-baz")}}},
						{
							AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
								Name: "MutatingAdmissionWebhook",
								Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
							},
							Kubeconfig: []byte("foo"),
						},
						{
							AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
								Name: "ValidatingAdmissionWebhook",
								Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
							},
							Kubeconfig: []byte("foo"),
						},
						{
							AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
								Name: "ImagePolicyWebhook",
								Config: &runtime.RawExtension{Raw: []byte(`imagePolicy:
  foo: bar
  kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
							},
							Kubeconfig: []byte("foo"),
						},
					}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							EnabledAdmissionPlugins: admissionPlugins,
							RuntimeVersion:          runtimeVersion,
						},
						Version: version,
					})

					configMapAdmission = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-admission-config", Namespace: namespace},
						Data: map[string]string{
							"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: Baz
  path: /etc/kubernetes/admission/baz.yaml
- configuration: null
  name: MutatingAdmissionWebhook
  path: /etc/kubernetes/admission/mutatingadmissionwebhook.yaml
- configuration: null
  name: ValidatingAdmissionWebhook
  path: /etc/kubernetes/admission/validatingadmissionwebhook.yaml
- configuration: null
  name: ImagePolicyWebhook
  path: /etc/kubernetes/admission/imagepolicywebhook.yaml
`,
							"baz.yaml": "some-config-for-baz",
							"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/mutatingadmissionwebhook-kubeconfig.yaml
`,
							"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/validatingadmissionwebhook-kubeconfig.yaml
`,
							"imagepolicywebhook.yaml": `imagePolicy:
  foo: bar
  kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/imagepolicywebhook-kubeconfig.yaml
`,
						},
					}
					Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
					Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAdmission.Name,
							Namespace:       configMapAdmission.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAdmission.Data,
					}))
				})

				It("should successfully deploy the configmap resource w/ admission plugins w/ config but w/o kubeconfigs", func() {
					admissionPlugins := []apiserver.AdmissionPluginConfig{
						{
							AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
								Name: "MutatingAdmissionWebhook",
								Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
							},
						},
						{
							AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
								Name: "ValidatingAdmissionWebhook",
								Config: &runtime.RawExtension{Raw: []byte(`apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
							},
						},
						{
							AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
								Name: "ImagePolicyWebhook",
								Config: &runtime.RawExtension{Raw: []byte(`imagePolicy:
  foo: bar
  kubeConfigFile: /etc/kubernetes/foobar.yaml
`)},
							},
						},
					}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							EnabledAdmissionPlugins: admissionPlugins,
							RuntimeVersion:          runtimeVersion,
						},
						Version: version,
					})

					configMapAdmission = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-admission-config", Namespace: namespace},
						Data: map[string]string{
							"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: MutatingAdmissionWebhook
  path: /etc/kubernetes/admission/mutatingadmissionwebhook.yaml
- configuration: null
  name: ValidatingAdmissionWebhook
  path: /etc/kubernetes/admission/validatingadmissionwebhook.yaml
- configuration: null
  name: ImagePolicyWebhook
  path: /etc/kubernetes/admission/imagepolicywebhook.yaml
`,
							"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: ""
`,
							"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1alpha1
kind: WebhookAdmission
kubeConfigFile: ""
`,
							"imagepolicywebhook.yaml": `imagePolicy:
  foo: bar
  kubeConfigFile: ""
`,
						},
					}
					Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
					Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAdmission.Name,
							Namespace:       configMapAdmission.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAdmission.Data,
					}))
				})

				It("should successfully deploy the configmap resource w/ admission plugins w/o configs but w/ kubeconfig", func() {
					admissionPlugins := []apiserver.AdmissionPluginConfig{
						{
							AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
								Name: "MutatingAdmissionWebhook",
							},
							Kubeconfig: []byte("foo"),
						},
						{
							AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
								Name: "ValidatingAdmissionWebhook",
							},
							Kubeconfig: []byte("foo"),
						},
						{
							AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{
								Name: "ImagePolicyWebhook",
							},
							Kubeconfig: []byte("foo"),
						},
					}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							EnabledAdmissionPlugins: admissionPlugins,
							RuntimeVersion:          runtimeVersion,
						},
						Version: version,
					})

					configMapAdmission = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-admission-config", Namespace: namespace},
						Data: map[string]string{
							"admission-configuration.yaml": `apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- configuration: null
  name: MutatingAdmissionWebhook
  path: /etc/kubernetes/admission/mutatingadmissionwebhook.yaml
- configuration: null
  name: ValidatingAdmissionWebhook
  path: /etc/kubernetes/admission/validatingadmissionwebhook.yaml
- configuration: null
  name: ImagePolicyWebhook
  path: /etc/kubernetes/admission/imagepolicywebhook.yaml
`,
							"mutatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/mutatingadmissionwebhook-kubeconfig.yaml
`,
							"validatingadmissionwebhook.yaml": `apiVersion: apiserver.config.k8s.io/v1
kind: WebhookAdmissionConfiguration
kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/validatingadmissionwebhook-kubeconfig.yaml
`,
							"imagepolicywebhook.yaml": `imagePolicy:
  kubeConfigFile: /etc/kubernetes/admission-kubeconfigs/imagepolicywebhook-kubeconfig.yaml
`,
						},
					}
					Expect(kubernetesutils.MakeUnique(configMapAdmission)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAdmission), configMapAdmission)).To(Succeed())
					Expect(configMapAdmission).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAdmission.Name,
							Namespace:       configMapAdmission.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAdmission.Data,
					}))
				})
			})

			Context("audit policy", func() {
				It("should successfully deploy the configmap resource w/ default policy", func() {
					configMapAuditPolicy = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "audit-policy-config", Namespace: namespace},
						Data: map[string]string{"audit-policy.yaml": `apiVersion: audit.k8s.io/v1
kind: Policy
metadata:
  creationTimestamp: null
rules:
- level: None
`},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuditPolicy)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(Succeed())
					Expect(configMapAuditPolicy).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuditPolicy.Name,
							Namespace:       configMapAuditPolicy.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuditPolicy.Data,
					}))
				})

				It("should successfully deploy the configmap resource w/o default policy", func() {
					var (
						policy      = "some-audit-policy"
						auditConfig = &apiserver.AuditConfig{Policy: &policy}
					)

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							Audit:          auditConfig,
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
					})

					configMapAuditPolicy = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "audit-policy-config", Namespace: namespace},
						Data:       map[string]string{"audit-policy.yaml": policy},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuditPolicy)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuditPolicy), configMapAuditPolicy)).To(Succeed())
					Expect(configMapAuditPolicy).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuditPolicy.Name,
							Namespace:       configMapAuditPolicy.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuditPolicy.Data,
					}))
				})
			})

			Context("authentication configuration", func() {
				It("should error when authentication config is set but version is < v1.30", func() {
					var (
						authenticationConfig = "some-auth-config"
						version              = semver.MustParse("1.29.0")
					)

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						AuthenticationConfiguration: ptr.To(authenticationConfig),
						Version:                     version,
					})

					Expect(kapi.Deploy(ctx)).To(MatchError("structured authentication is not available for versions < v1.30"))
				})

				It("should error when authentication config and oidc settings are configured", func() {
					var (
						authenticationConfig = "some-auth-config"
						version              = semver.MustParse("1.30.0")
					)

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						AuthenticationConfiguration: ptr.To(authenticationConfig),
						OIDC:                        &gardencorev1beta1.OIDCConfig{},
						Version:                     version,
					})

					configMapAuthentication = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
						Data:       map[string]string{"config.yaml": authenticationConfig},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(MatchError("oidc configuration is incompatible with structured authentication"))
				})

				It("should successfully deploy the configmap resource disabling anonymous authentication", func() {
					var (
						authenticationConfigInput = &apiserverv1beta1.AuthenticationConfiguration{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "apiserver.config.k8s.io/v1beta1",
								Kind:       "AuthenticationConfiguration",
							},
							JWT: []apiserverv1beta1.JWTAuthenticator{
								{
									Issuer: apiserverv1beta1.Issuer{
										URL:       "https://foo.com",
										Audiences: []string{"example-client-id"},
									},
									ClaimMappings: apiserverv1beta1.ClaimMappings{
										Username: apiserverv1beta1.PrefixedClaimOrExpression{
											Claim:  "username",
											Prefix: ptr.To("foo:"),
										},
									},
								},
							},
						}
						version = semver.MustParse("1.32.0")
					)

					authenticationConfig, err := runtime.Encode(ConfigCodec, authenticationConfigInput)
					Expect(err).ToNot(HaveOccurred())

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						AuthenticationConfiguration: ptr.To(string(authenticationConfig)),
						Version:                     version,
					})

					authenticationConfigInput.Anonymous = &apiserverv1beta1.AnonymousAuthConfig{
						Enabled: false,
					}
					expectedAuthenticationConfig, err := runtime.Encode(ConfigCodec, authenticationConfigInput)
					Expect(err).ToNot(HaveOccurred())

					configMapAuthentication = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
						Data:       map[string]string{"config.yaml": string(expectedAuthenticationConfig)},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(Succeed())
					Expect(configMapAuthentication).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuthentication.Name,
							Namespace:       configMapAuthentication.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuthentication.Data,
					}))
				})

				It("should successfully deploy the configmap resource enabling anonymous authentication when config is passed directly to deployer", func() {
					var (
						authenticationConfigInput = &apiserverv1beta1.AuthenticationConfiguration{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "apiserver.config.k8s.io/v1beta1",
								Kind:       "AuthenticationConfiguration",
							},
							JWT: []apiserverv1beta1.JWTAuthenticator{
								{
									Issuer: apiserverv1beta1.Issuer{
										URL:       "https://foo.com",
										Audiences: []string{"example-client-id"},
									},
									ClaimMappings: apiserverv1beta1.ClaimMappings{
										Username: apiserverv1beta1.PrefixedClaimOrExpression{
											Claim:  "username",
											Prefix: ptr.To("foo:"),
										},
									},
								},
							},
						}
						version = semver.MustParse("1.32.0")
					)

					authenticationConfig, err := runtime.Encode(ConfigCodec, authenticationConfigInput)
					Expect(err).ToNot(HaveOccurred())

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						AnonymousAuthenticationEnabled: ptr.To(true),
						AuthenticationConfiguration:    ptr.To(string(authenticationConfig)),
						Version:                        version,
					})

					authenticationConfigInput.Anonymous = &apiserverv1beta1.AnonymousAuthConfig{
						Enabled: true,
					}
					expectedAuthenticationConfig, err := runtime.Encode(ConfigCodec, authenticationConfigInput)
					Expect(err).ToNot(HaveOccurred())

					configMapAuthentication = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
						Data:       map[string]string{"config.yaml": string(expectedAuthenticationConfig)},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(Succeed())
					Expect(configMapAuthentication).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuthentication.Name,
							Namespace:       configMapAuthentication.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuthentication.Data,
					}))
				})

				It("should successfully deploy the configmap resource and passed anonymous authentication should take precedence", func() {
					var (
						authenticationConfigInput = &apiserverv1beta1.AuthenticationConfiguration{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "apiserver.config.k8s.io/v1beta1",
								Kind:       "AuthenticationConfiguration",
							},
							JWT: []apiserverv1beta1.JWTAuthenticator{
								{
									Issuer: apiserverv1beta1.Issuer{
										URL:       "https://foo.com",
										Audiences: []string{"example-client-id"},
									},
									ClaimMappings: apiserverv1beta1.ClaimMappings{
										Username: apiserverv1beta1.PrefixedClaimOrExpression{
											Claim:  "username",
											Prefix: ptr.To("foo:"),
										},
									},
								},
							},
							Anonymous: &apiserverv1beta1.AnonymousAuthConfig{
								Enabled: true,
							},
						}
						version = semver.MustParse("1.32.0")
					)

					authenticationConfig, err := runtime.Encode(ConfigCodec, authenticationConfigInput)
					Expect(err).ToNot(HaveOccurred())

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						AnonymousAuthenticationEnabled: ptr.To(false),
						AuthenticationConfiguration:    ptr.To(string(authenticationConfig)),
						Version:                        version,
					})

					configMapAuthentication = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
						Data:       map[string]string{"config.yaml": string(authenticationConfig)},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(Succeed())
					Expect(configMapAuthentication).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuthentication.Name,
							Namespace:       configMapAuthentication.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuthentication.Data,
					}))
				})

				It("should successfully deploy the configmap resource but not configure anonymous authentication if AnonymousAuthConfigurableEndpoints is disabled", func() {
					var (
						authenticationConfigInput = &apiserverv1beta1.AuthenticationConfiguration{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "apiserver.config.k8s.io/v1beta1",
								Kind:       "AuthenticationConfiguration",
							},
							JWT: []apiserverv1beta1.JWTAuthenticator{
								{
									Issuer: apiserverv1beta1.Issuer{
										URL:       "https://foo.com",
										Audiences: []string{"example-client-id"},
									},
									ClaimMappings: apiserverv1beta1.ClaimMappings{
										Username: apiserverv1beta1.PrefixedClaimOrExpression{
											Claim:  "username",
											Prefix: ptr.To("foo:"),
										},
									},
								},
							},
						}
						version = semver.MustParse("1.32.0")
					)

					authenticationConfig, err := runtime.Encode(ConfigCodec, authenticationConfigInput)
					Expect(err).ToNot(HaveOccurred())

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
							FeatureGates: map[string]bool{
								"AnonymousAuthConfigurableEndpoints": false,
							},
						},
						AnonymousAuthenticationEnabled: ptr.To(true),
						AuthenticationConfiguration:    ptr.To(string(authenticationConfig)),
						Version:                        version,
					})

					configMapAuthentication = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
						Data:       map[string]string{"config.yaml": string(authenticationConfig)},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(Succeed())
					Expect(configMapAuthentication).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuthentication.Name,
							Namespace:       configMapAuthentication.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuthentication.Data,
					}))
				})

				It("should not deploy the configmap resource when feature gate is disabled", func() {
					var (
						authenticationConfig = "some-auth-config"
						version              = semver.MustParse("1.30.0")
					)

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
							FeatureGates: map[string]bool{
								"StructuredAuthenticationConfiguration": false,
							},
						},
						AuthenticationConfiguration: ptr.To(authenticationConfig),
						Version:                     version,
					})

					configMapAuthentication = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
						Data:       map[string]string{"config.yaml": authenticationConfig},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(BeNotFoundError())
				})

				It("should successfully deploy the configmap resource from oidc settings", func() {
					var (
						oidc = &gardencorev1beta1.OIDCConfig{
							CABundle:     ptr.To("some-ca-bundle"),
							ClientID:     ptr.To("some-client-id"),
							GroupsClaim:  ptr.To("some-groups-claim"),
							GroupsPrefix: ptr.To("some-groups-prefix"),
							IssuerURL:    ptr.To("https://issuer.url.com"),
							RequiredClaims: map[string]string{
								"claim": "value",
							},
							SigningAlgs:    []string{"signing", "algs"},
							UsernameClaim:  ptr.To("some-username-claim"),
							UsernamePrefix: ptr.To("some-username-prefix"),
						}
						version              = semver.MustParse("1.30.0")
						authenticationConfig = `apiVersion: apiserver.config.k8s.io/v1beta1
jwt:
- claimMappings:
    groups:
      claim: some-groups-claim
      prefix: some-groups-prefix
    uid: {}
    username:
      claim: some-username-claim
      prefix: some-username-prefix
  claimValidationRules:
  - claim: claim
    requiredValue: value
  issuer:
    audiences:
    - some-client-id
    certificateAuthority: some-ca-bundle
    url: https://issuer.url.com
kind: AuthenticationConfiguration
`
					)

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						OIDC:    oidc,
						Version: version,
					})

					configMapAuthentication = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
						Data:       map[string]string{"config.yaml": authenticationConfig},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(Succeed())
					Expect(configMapAuthentication).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuthentication.Name,
							Namespace:       configMapAuthentication.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuthentication.Data,
					}))
				})
			})

			It("should successfully deploy the configmap resource from oidc settings with defaults", func() {
				var (
					oidc = &gardencorev1beta1.OIDCConfig{
						ClientID:    ptr.To("some-client-id"),
						GroupsClaim: ptr.To("some-groups-claim"),
						IssuerURL:   ptr.To("https://issuer.url.com"),
					}
					version              = semver.MustParse("1.30.0")
					authenticationConfig = `apiVersion: apiserver.config.k8s.io/v1beta1
jwt:
- claimMappings:
    groups:
      claim: some-groups-claim
      prefix: ""
    uid: {}
    username:
      claim: sub
      prefix: https://issuer.url.com#
  issuer:
    audiences:
    - some-client-id
    url: https://issuer.url.com
kind: AuthenticationConfiguration
`
				)

				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
					},
					OIDC:    oidc,
					Version: version,
				})

				configMapAuthentication = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
					Data:       map[string]string{"config.yaml": authenticationConfig},
				}
				Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(BeNotFoundError())
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(Succeed())
				Expect(configMapAuthentication).To(DeepEqual(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:            configMapAuthentication.Name,
						Namespace:       configMapAuthentication.Namespace,
						Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
						ResourceVersion: "1",
					},
					Immutable: ptr.To(true),
					Data:      configMapAuthentication.Data,
				}))
			})

			It("should successfully deploy the configmap resource from oidc settings with empty user prefix", func() {
				var (
					oidc = &gardencorev1beta1.OIDCConfig{
						ClientID:       ptr.To("some-client-id"),
						IssuerURL:      ptr.To("https://issuer.url.com"),
						UsernamePrefix: ptr.To("-"),
					}
					version              = semver.MustParse("1.30.0")
					authenticationConfig = `apiVersion: apiserver.config.k8s.io/v1beta1
jwt:
- claimMappings:
    groups: {}
    uid: {}
    username:
      claim: sub
      prefix: ""
  issuer:
    audiences:
    - some-client-id
    url: https://issuer.url.com
kind: AuthenticationConfiguration
`
				)

				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
					},
					OIDC:    oidc,
					Version: version,
				})

				configMapAuthentication = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
					Data:       map[string]string{"config.yaml": authenticationConfig},
				}
				Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(BeNotFoundError())
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthentication), configMapAuthentication)).To(Succeed())
				Expect(configMapAuthentication).To(DeepEqual(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:            configMapAuthentication.Name,
						Namespace:       configMapAuthentication.Namespace,
						Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
						ResourceVersion: "1",
					},
					Immutable: ptr.To(true),
					Data:      configMapAuthentication.Data,
				}))
			})

			Context("authorization configuration", func() {
				It("should do nothing when Kubernetes version is < v1.30", func() {
					version := semver.MustParse("1.29.0")

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
					})

					configMapAuthorization = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authorization-config", Namespace: namespace}}
					Expect(kubernetesutils.MakeUnique(configMapAuthorization)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(BeNotFoundError())
				})

				It("should do nothing when Kubernetes version is >= v1.30 but the feature gate is disabled", func() {
					version := semver.MustParse("1.30.0")

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
							FeatureGates: map[string]bool{
								"StructuredAuthorizationConfiguration": false,
							},
						},
						Version: version,
					})

					configMapAuthorization = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authorization-config", Namespace: namespace}}
					Expect(kubernetesutils.MakeUnique(configMapAuthorization)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(BeNotFoundError())
				})

				It("should successfully deploy the configmap resource w/o webhooks", func() {
					version := semver.MustParse("1.30.0")

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
					})

					configMapAuthorization = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authorization-config", Namespace: namespace},
						Data: map[string]string{"config.yaml": `apiVersion: apiserver.config.k8s.io/v1beta1
authorizers:
- name: node
  type: Node
- name: rbac
  type: RBAC
kind: AuthorizationConfiguration
`},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthorization)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(Succeed())
					Expect(configMapAuthorization).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuthorization.Name,
							Namespace:       configMapAuthorization.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuthorization.Data,
					}))
				})

				It("should successfully deploy the configmap resource w/ webhooks", func() {
					version := semver.MustParse("1.30.0")

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
						AuthorizationWebhooks: []AuthorizationWebhook{
							{Name: "foo", WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{}},
							{Name: "bar", WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{}},
						},
					})

					configMapAuthorization = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authorization-config", Namespace: namespace},
						Data: map[string]string{"config.yaml": `apiVersion: apiserver.config.k8s.io/v1beta1
authorizers:
- name: node
  type: Node
- name: rbac
  type: RBAC
- name: foo
  type: Webhook
  webhook:
    authorizedTTL: 0s
    connectionInfo:
      kubeConfigFile: /etc/kubernetes/structured/authorization-kubeconfigs/foo-kubeconfig.yaml
      type: KubeConfigFile
    failurePolicy: ""
    matchConditionSubjectAccessReviewVersion: ""
    matchConditions: null
    subjectAccessReviewVersion: ""
    timeout: 0s
    unauthorizedTTL: 0s
- name: bar
  type: Webhook
  webhook:
    authorizedTTL: 0s
    connectionInfo:
      kubeConfigFile: /etc/kubernetes/structured/authorization-kubeconfigs/bar-kubeconfig.yaml
      type: KubeConfigFile
    failurePolicy: ""
    matchConditionSubjectAccessReviewVersion: ""
    matchConditions: null
    subjectAccessReviewVersion: ""
    timeout: 0s
    unauthorizedTTL: 0s
kind: AuthorizationConfiguration
`},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthorization)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(Succeed())
					Expect(configMapAuthorization).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuthorization.Name,
							Namespace:       configMapAuthorization.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuthorization.Data,
					}))
				})

				It("should successfully deploy the configmap resource for workerless clusters w/o webhooks", func() {
					version := semver.MustParse("1.30.0")

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Version:      version,
						IsWorkerless: true,
					})

					configMapAuthorization = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authorization-config", Namespace: namespace},
						Data: map[string]string{"config.yaml": `apiVersion: apiserver.config.k8s.io/v1beta1
authorizers:
- name: rbac
  type: RBAC
kind: AuthorizationConfiguration
`},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthorization)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(Succeed())
					Expect(configMapAuthorization).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuthorization.Name,
							Namespace:       configMapAuthorization.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuthorization.Data,
					}))
				})

				It("should successfully deploy the configmap resource for workerless clusters w/ webhooks", func() {
					version := semver.MustParse("1.30.0")

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
						AuthorizationWebhooks: []AuthorizationWebhook{
							{Name: "foo", WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{}},
							{Name: "bar", WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{}},
						},
						IsWorkerless: true,
					})

					configMapAuthorization = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authorization-config", Namespace: namespace},
						Data: map[string]string{"config.yaml": `apiVersion: apiserver.config.k8s.io/v1beta1
authorizers:
- name: rbac
  type: RBAC
- name: foo
  type: Webhook
  webhook:
    authorizedTTL: 0s
    connectionInfo:
      kubeConfigFile: /etc/kubernetes/structured/authorization-kubeconfigs/foo-kubeconfig.yaml
      type: KubeConfigFile
    failurePolicy: ""
    matchConditionSubjectAccessReviewVersion: ""
    matchConditions: null
    subjectAccessReviewVersion: ""
    timeout: 0s
    unauthorizedTTL: 0s
- name: bar
  type: Webhook
  webhook:
    authorizedTTL: 0s
    connectionInfo:
      kubeConfigFile: /etc/kubernetes/structured/authorization-kubeconfigs/bar-kubeconfig.yaml
      type: KubeConfigFile
    failurePolicy: ""
    matchConditionSubjectAccessReviewVersion: ""
    matchConditions: null
    subjectAccessReviewVersion: ""
    timeout: 0s
    unauthorizedTTL: 0s
kind: AuthorizationConfiguration
`},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthorization)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapAuthorization), configMapAuthorization)).To(Succeed())
					Expect(configMapAuthorization).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapAuthorization.Name,
							Namespace:       configMapAuthorization.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapAuthorization.Data,
					}))
				})
			})

			Context("egress selector", func() {
				It("should successfully deploy the configmap resource", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
						VPN:     VPNConfig{Enabled: true},
					})

					configMapEgressSelector = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-egress-selector-config", Namespace: namespace},
						Data:       map[string]string{"egress-selector-configuration.yaml": egressSelectorConfigFor("controlplane")},
					}
					Expect(kubernetesutils.MakeUnique(configMapEgressSelector)).To(Succeed())

					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapEgressSelector), configMapEgressSelector)).To(BeNotFoundError())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapEgressSelector), configMapEgressSelector)).To(Succeed())
					Expect(configMapEgressSelector).To(DeepEqual(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:            configMapEgressSelector.Name,
							Namespace:       configMapEgressSelector.Namespace,
							Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
							ResourceVersion: "1",
						},
						Immutable: ptr.To(true),
						Data:      configMapEgressSelector.Data,
					}))
				})

				DescribeTable("do nothing",
					func(vpnConfig VPNConfig) {
						kapi = New(kubernetesInterface, namespace, sm, Values{
							Version: version,
							VPN:     vpnConfig,
						})

						var found bool

						configMapList := &corev1.ConfigMapList{}
						Expect(c.List(ctx, configMapList, client.InNamespace(namespace))).To(Succeed())
						for _, configMap := range configMapList.Items {
							if strings.HasPrefix(configMap.Name, "kube-apiserver-egress-selector-config") {
								found = true
								break
							}
						}

						Expect(found).To(BeFalse())
					},

					Entry("VPN is disabled", VPNConfig{Enabled: false}),
					Entry("VPN is enabled but HA is disabled", VPNConfig{Enabled: true, HighAvailabilityEnabled: false}),
				)
			})
		})

		Describe("Deployment", func() {
			deployAndRead := func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(BeNotFoundError())
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			}

			It("should have the expected labels w/o SNI", func() {
				deployAndRead()

				Expect(deployment.Labels).To(Equal(map[string]string{
					"gardener.cloud/role": "controlplane",
					"app":                 "kubernetes",
					"role":                "apiserver",
					"high-availability-config.resources.gardener.cloud/type":             "server",
					"provider.extensions.gardener.cloud/mutated-by-controlplane-webhook": "true",
				}))
			})

			It("should have the expected labels w/ SNI", func() {
				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
					},
					SNI:     SNIConfig{Enabled: true},
					Version: version,
				})
				deployAndRead()

				Expect(deployment.Labels).To(Equal(map[string]string{
					"gardener.cloud/role": "controlplane",
					"app":                 "kubernetes",
					"role":                "apiserver",
					"high-availability-config.resources.gardener.cloud/type":             "server",
					"provider.extensions.gardener.cloud/mutated-by-controlplane-webhook": "true",
				}))
			})

			Context("expected annotations", func() {
				var defaultAnnotations map[string]string

				BeforeEach(func() {
					defaultAnnotations = map[string]string{
						"reference.resources.gardener.cloud/secret-a92da147":    secretNameCAFrontProxy,
						"reference.resources.gardener.cloud/secret-a709ce3a":    secretNameServiceAccountKey,
						"reference.resources.gardener.cloud/secret-ad29e1cc":    secretNameServiceAccountKeyBundle,
						"reference.resources.gardener.cloud/secret-69590970":    secretNameCA,
						"reference.resources.gardener.cloud/secret-17c26aa4":    secretNameCAClient,
						"reference.resources.gardener.cloud/secret-e01f5645":    secretNameCAEtcd,
						"reference.resources.gardener.cloud/secret-389fbba5":    secretNameEtcd,
						"reference.resources.gardener.cloud/secret-998b2966":    secretNameKubeAggregator,
						"reference.resources.gardener.cloud/secret-3ddd1800":    secretNameServer,
						"reference.resources.gardener.cloud/secret-af50ac19":    secretNameStaticToken,
						"reference.resources.gardener.cloud/secret-c4700ce9":    secretNameETCDEncryptionConfig,
						"reference.resources.gardener.cloud/configmap-130aa219": configMapNameAdmissionConfigs,
						"reference.resources.gardener.cloud/secret-5613e39f":    secretNameAdmissionKubeconfigs,
						"reference.resources.gardener.cloud/configmap-d4419cd4": configMapNameAuditPolicy,
					}
				})

				It("should have the expected annotations when there are no nodes", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless: true,
						Version:      version,
					})
					deployAndRead()

					Expect(deployment.Annotations).To(Equal(defaultAnnotations))
				})

				It("should have the expected annotations when there are nodes", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless: false,
						Version:      version,
					})
					deployAndRead()

					Expect(deployment.Annotations).To(Equal(utils.MergeStringMaps(defaultAnnotations, map[string]string{
						"reference.resources.gardener.cloud/secret-77bc5458": secretNameCAKubelet,
						"reference.resources.gardener.cloud/secret-c1267cc2": secretNameKubeAPIServerToKubelet,
					})))
				})

				It("should have the expected annotations when VPN is disabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless: true,
						Version:      version,
						VPN:          VPNConfig{Enabled: false},
					})
					deployAndRead()

					Expect(deployment.Annotations).To(Equal(defaultAnnotations))
				})

				It("should have the expected annotations when VPN is enabled but HA is disabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless: true,
						Version:      version,
						VPN:          VPNConfig{Enabled: true, HighAvailabilityEnabled: false},
					})
					deployAndRead()

					Expect(deployment.Annotations).To(Equal(utils.MergeStringMaps(defaultAnnotations, map[string]string{
						"reference.resources.gardener.cloud/secret-0acc967c":    secretNameHTTPProxy,
						"reference.resources.gardener.cloud/secret-8ddd8e24":    secretNameCAVPN,
						"reference.resources.gardener.cloud/configmap-f79954be": configMapNameEgressPolicy,
					})))
				})

				It("should have the expected annotations when VPN and HA is enabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless:        true,
						Version:             version,
						ServiceNetworkCIDRs: []net.IPNet{{IP: net.ParseIP("4.5.6.0"), Mask: net.CIDRMask(24, 32)}},
						VPN:                 VPNConfig{Enabled: true, HighAvailabilityEnabled: true, PodNetworkCIDRs: []net.IPNet{{IP: net.ParseIP("1.2.3.0"), Mask: net.CIDRMask(24, 32)}}},
					})
					deployAndRead()

					Expect(deployment.Annotations).To(Equal(utils.MergeStringMaps(defaultAnnotations, map[string]string{
						"reference.resources.gardener.cloud/secret-8ddd8e24":    secretNameCAVPN,
						"reference.resources.gardener.cloud/secret-a41fe9a3":    secretNameVPNSeedClient,
						"reference.resources.gardener.cloud/secret-facfe649":    secretNameVPNSeedServerTLSAuth,
						"reference.resources.gardener.cloud/configmap-a9a818ab": "kube-root-ca.crt",
					})))
				})
			})

			It("should have the expected deployment settings", func() {
				var (
					intStr100Percent = intstr.FromString("100%")
					intStrZero       = intstr.FromInt32(0)
				)

				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
					},
					Version: version,
				})
				deployAndRead()

				Expect(deployment.Spec.Strategy).To(Equal(appsv1.DeploymentStrategy{
					Type: appsv1.RollingUpdateDeploymentStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDeployment{
						MaxSurge:       &intStr100Percent,
						MaxUnavailable: &intStrZero,
					},
				}))
			})

			Context("expected pod template labels", func() {
				var defaultLabels map[string]string

				BeforeEach(func() {
					defaultLabels = map[string]string{
						"gardener.cloud/role":              "controlplane",
						"app":                              "kubernetes",
						"role":                             "apiserver",
						"networking.gardener.cloud/to-dns": "allowed",
						"networking.gardener.cloud/to-private-networks":                         "allowed",
						"networking.gardener.cloud/to-public-networks":                          "allowed",
						"networking.resources.gardener.cloud/to-all-webhook-targets":            "allowed",
						"networking.resources.gardener.cloud/to-extensions-all-webhook-targets": "allowed",
						"networking.resources.gardener.cloud/to-etcd-main-client-tcp-2379":      "allowed",
						"networking.resources.gardener.cloud/to-etcd-events-client-tcp-2379":    "allowed",
					}
				})

				It("should have the expected pod template labels", func() {
					deployAndRead()

					Expect(deployment.Spec.Template.Labels).To(Equal(utils.MergeStringMaps(defaultLabels, map[string]string{
						"networking.resources.gardener.cloud/to-vpn-seed-server-tcp-9443": "allowed",
					})))
				})

				It("should have the expected pod template labels with vpn enabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless: true,
						Version:      version,
						VPN:          VPNConfig{Enabled: true},
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Labels).To(Equal(utils.MergeStringMaps(defaultLabels, map[string]string{
						"networking.resources.gardener.cloud/to-vpn-seed-server-tcp-9443": "allowed",
					})))
				})

				It("should have the expected pod template labels with ha vpn enabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless:        true,
						Version:             version,
						ServiceNetworkCIDRs: []net.IPNet{{IP: net.ParseIP("4.5.6.0"), Mask: net.CIDRMask(24, 32)}},
						VPN:                 VPNConfig{Enabled: true, HighAvailabilityEnabled: true, HighAvailabilityNumberOfSeedServers: 2, PodNetworkCIDRs: []net.IPNet{{IP: net.ParseIP("1.2.3.0"), Mask: net.CIDRMask(24, 32)}}},
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Labels).To(Equal(utils.MergeStringMaps(defaultLabels, map[string]string{
						"networking.gardener.cloud/to-runtime-apiserver":                    "allowed",
						"networking.resources.gardener.cloud/to-vpn-seed-server-0-tcp-1194": "allowed",
						"networking.resources.gardener.cloud/to-vpn-seed-server-1-tcp-1194": "allowed",
					})))
				})
			})

			Context("expected pod template annotations", func() {
				var defaultAnnotations map[string]string

				BeforeEach(func() {
					defaultAnnotations = map[string]string{
						"reference.resources.gardener.cloud/secret-a709ce3a":    secretNameServiceAccountKey,
						"reference.resources.gardener.cloud/secret-ad29e1cc":    secretNameServiceAccountKeyBundle,
						"reference.resources.gardener.cloud/secret-69590970":    secretNameCA,
						"reference.resources.gardener.cloud/secret-17c26aa4":    secretNameCAClient,
						"reference.resources.gardener.cloud/secret-e01f5645":    secretNameCAEtcd,
						"reference.resources.gardener.cloud/secret-a92da147":    secretNameCAFrontProxy,
						"reference.resources.gardener.cloud/secret-389fbba5":    secretNameEtcd,
						"reference.resources.gardener.cloud/secret-998b2966":    secretNameKubeAggregator,
						"reference.resources.gardener.cloud/secret-3ddd1800":    secretNameServer,
						"reference.resources.gardener.cloud/secret-af50ac19":    secretNameStaticToken,
						"reference.resources.gardener.cloud/secret-c4700ce9":    secretNameETCDEncryptionConfig,
						"reference.resources.gardener.cloud/configmap-130aa219": configMapNameAdmissionConfigs,
						"reference.resources.gardener.cloud/secret-5613e39f":    secretNameAdmissionKubeconfigs,
						"reference.resources.gardener.cloud/configmap-d4419cd4": configMapNameAuditPolicy,
					}
				})

				It("should have the expected annotations when there are no nodes", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless: true,
						Version:      version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Annotations).To(Equal(defaultAnnotations))
				})

				It("should have the expected annotations when there are nodes", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless: false,
						Version:      version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Annotations).To(Equal(utils.MergeStringMaps(defaultAnnotations, map[string]string{
						"reference.resources.gardener.cloud/secret-77bc5458": secretNameCAKubelet,
						"reference.resources.gardener.cloud/secret-c1267cc2": secretNameKubeAPIServerToKubelet,
					})))
				})

				It("should have the expected annotations when VPN is disabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless: true,
						Version:      version,
						VPN:          VPNConfig{Enabled: false},
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Annotations).To(Equal(defaultAnnotations))
				})

				It("should have the expected annotations when VPN is enabled but HA is disabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless: true,
						Version:      version,
						VPN:          VPNConfig{Enabled: true, HighAvailabilityEnabled: false},
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Annotations).To(Equal(utils.MergeStringMaps(defaultAnnotations, map[string]string{
						"reference.resources.gardener.cloud/secret-0acc967c":    secretNameHTTPProxy,
						"reference.resources.gardener.cloud/secret-8ddd8e24":    secretNameCAVPN,
						"reference.resources.gardener.cloud/configmap-f79954be": configMapNameEgressPolicy,
					})))
				})

				It("should have the expected annotations when VPN and HA is enabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						IsWorkerless:        true,
						Version:             version,
						ServiceNetworkCIDRs: []net.IPNet{{IP: net.ParseIP("4.5.6.0"), Mask: net.CIDRMask(24, 32)}},
						VPN:                 VPNConfig{Enabled: true, HighAvailabilityEnabled: true, PodNetworkCIDRs: []net.IPNet{{IP: net.ParseIP("1.2.3.0"), Mask: net.CIDRMask(24, 32)}}},
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Annotations).To(Equal(utils.MergeStringMaps(defaultAnnotations, map[string]string{
						"reference.resources.gardener.cloud/secret-8ddd8e24":    secretNameCAVPN,
						"reference.resources.gardener.cloud/secret-a41fe9a3":    secretNameVPNSeedClient,
						"reference.resources.gardener.cloud/secret-facfe649":    secretNameVPNSeedServerTLSAuth,
						"reference.resources.gardener.cloud/configmap-a9a818ab": "kube-root-ca.crt",
					})))
				})
			})

			It("should have the expected pod settings", func() {
				deployAndRead()

				Expect(deployment.Spec.Template.Spec.PriorityClassName).To(Equal(priorityClassName))
				Expect(deployment.Spec.Template.Spec.AutomountServiceAccountToken).To(PointTo(BeFalse()))
				Expect(deployment.Spec.Template.Spec.DNSPolicy).To(Equal(corev1.DNSClusterFirst))
				Expect(deployment.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
				Expect(deployment.Spec.Template.Spec.SchedulerName).To(Equal("default-scheduler"))
				Expect(deployment.Spec.Template.Spec.TerminationGracePeriodSeconds).To(PointTo(Equal(int64(30))))
				Expect(deployment.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(Equal(ptr.To(true)))
				Expect(deployment.Spec.Template.Spec.SecurityContext.RunAsUser).To(PointTo(Equal(int64(65532))))
				Expect(deployment.Spec.Template.Spec.SecurityContext.RunAsGroup).To(PointTo(Equal(int64(65532))))
				Expect(deployment.Spec.Template.Spec.SecurityContext.FSGroup).To(PointTo(Equal(int64(65532))))
			})

			It("should have no init containers", func() {
				kapi = New(kubernetesInterface, namespace, sm, Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
					},
					Version: version,
				})

				deployAndRead()

				Expect(deployment.Spec.Template.Spec.InitContainers).To(BeEmpty())
			})

			haVPNClientContainerFor := func(index int) corev1.Container {

				var serviceCIDRs, podCIDRs, nodeCIDRs []string

				for _, v := range values.ServiceNetworkCIDRs {
					serviceCIDRs = append(serviceCIDRs, v.String())
				}

				for _, v := range values.VPN.PodNetworkCIDRs {
					podCIDRs = append(podCIDRs, v.String())
				}

				for _, v := range values.VPN.NodeNetworkCIDRs {
					nodeCIDRs = append(nodeCIDRs, v.String())
				}

				container := corev1.Container{
					Name:            fmt.Sprintf("vpn-client-%d", index),
					Image:           "vpn-client-image:really-latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env: []corev1.EnvVar{
						{
							Name:  "ENDPOINT",
							Value: fmt.Sprintf("vpn-seed-server-%d", index),
						},
						{
							Name:  "SERVICE_NETWORK",
							Value: values.ServiceNetworkCIDRs[0].String(),
						},
						{
							Name:  "POD_NETWORK",
							Value: values.VPN.PodNetworkCIDRs[0].String(),
						},
						{
							Name:  "NODE_NETWORK",
							Value: values.VPN.NodeNetworkCIDRs[0].String(),
						},
						{
							Name:  "SERVICE_NETWORKS",
							Value: strings.Join(serviceCIDRs, ","),
						},
						{
							Name:  "POD_NETWORKS",
							Value: strings.Join(podCIDRs, ","),
						},
						{
							Name:  "NODE_NETWORKS",
							Value: strings.Join(nodeCIDRs, ","),
						},
						{
							Name:  "VPN_SERVER_INDEX",
							Value: strconv.Itoa(index),
						},
						{
							Name:  "IS_HA",
							Value: "true",
						},
						{
							Name:  "HA_VPN_SERVERS",
							Value: "2",
						},
						{
							Name:  "HA_VPN_CLIENTS",
							Value: "3",
						},
						{
							Name:  "OPENVPN_PORT",
							Value: "1194",
						},
						{
							Name:  "IP_FAMILIES",
							Value: "IPv4",
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("5M"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot: ptr.To(false),
						RunAsUser:    ptr.To[int64](0),
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN"},
						},
					},
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "vpn-seed-client",
							MountPath: "/srv/secrets/vpn-client",
						},
						{
							Name:      "vpn-seed-tlsauth",
							MountPath: "/srv/secrets/tlsauth",
						},
						{
							Name:      "dev-net-tun",
							MountPath: "/dev/net/tun",
						},
					},
				}

				return container
			}

			haVPNInitClientContainer := func() corev1.Container {
				initContainer := haVPNClientContainerFor(0)
				initContainer.Name = "vpn-client-init"
				initContainer.LivenessProbe = nil
				initContainer.Command = []string{"/bin/vpn-client", "setup"}
				initContainer.Env = append(initContainer.Env, []corev1.EnvVar{
					{
						Name: "POD_NAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.name",
							},
						},
					},
					{
						Name: "NAMESPACE",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.namespace",
							},
						},
					},
				}...)
				initContainer.VolumeMounts = append(initContainer.VolumeMounts, corev1.VolumeMount{
					Name:      "kube-api-access-gardener",
					MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
					ReadOnly:  true,
				})
				initContainer.SecurityContext.Privileged = ptr.To(true)
				return initContainer
			}

			testHAVPN := func() {
				values = Values{
					Values: apiserver.Values{
						RuntimeVersion: runtimeVersion,
					},
					Images:              Images{VPNClient: "vpn-client-image:really-latest"},
					ServiceNetworkCIDRs: []net.IPNet{{IP: net.ParseIP("4.5.6.0"), Mask: net.CIDRMask(24, 32)}},
					VPN: VPNConfig{
						Enabled:                              true,
						HighAvailabilityEnabled:              true,
						HighAvailabilityNumberOfSeedServers:  2,
						HighAvailabilityNumberOfShootClients: 3,
						PodNetworkCIDRs:                      []net.IPNet{{IP: net.ParseIP("1.2.3.0"), Mask: net.CIDRMask(24, 32)}},
						NodeNetworkCIDRs:                     []net.IPNet{{IP: net.ParseIP("7.8.9.0"), Mask: net.CIDRMask(24, 32)}},
						IPFamilies:                           []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4},
					},
					Version: version,
				}
				kapi = New(kubernetesInterface, namespace, sm, values)
				deployAndRead()

				initContainer := haVPNInitClientContainer()
				Expect(deployment.Spec.Template.Spec.InitContainers).To(DeepEqual([]corev1.Container{initContainer}))
				Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(values.VPN.HighAvailabilityNumberOfSeedServers + 2))
				for i := 0; i < values.VPN.HighAvailabilityNumberOfSeedServers; i++ {
					labelKey := fmt.Sprintf("networking.resources.gardener.cloud/to-vpn-seed-server-%d-tcp-1194", i)
					Expect(deployment.Spec.Template.Labels).To(HaveKeyWithValue(labelKey, "allowed"))
					Expect(deployment.Spec.Template.Spec.Containers[i+1]).To(DeepEqual(haVPNClientContainerFor(i)))
				}

				var serviceCIDRs, podCIDRs, nodeCIDRs []string
				for _, v := range values.ServiceNetworkCIDRs {
					serviceCIDRs = append(serviceCIDRs, v.String())
				}
				for _, v := range values.VPN.PodNetworkCIDRs {
					podCIDRs = append(podCIDRs, v.String())
				}
				for _, v := range values.VPN.NodeNetworkCIDRs {
					nodeCIDRs = append(nodeCIDRs, v.String())
				}
				pathControllerContainer := corev1.Container{
					Name:            "vpn-path-controller",
					Image:           "vpn-client-image:really-latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/bin/vpn-client", "path-controller"},
					Env: []corev1.EnvVar{
						{
							Name:  "SERVICE_NETWORK",
							Value: values.ServiceNetworkCIDRs[0].String(),
						},
						{
							Name:  "POD_NETWORK",
							Value: values.VPN.PodNetworkCIDRs[0].String(),
						},
						{
							Name:  "NODE_NETWORK",
							Value: values.VPN.NodeNetworkCIDRs[0].String(),
						},
						{
							Name:  "SERVICE_NETWORKS",
							Value: strings.Join(serviceCIDRs, ","),
						},
						{
							Name:  "POD_NETWORKS",
							Value: strings.Join(podCIDRs, ","),
						},
						{
							Name:  "NODE_NETWORKS",
							Value: strings.Join(nodeCIDRs, ","),
						},
						{
							Name:  "IS_HA",
							Value: "true",
						},
						{
							Name:  "HA_VPN_CLIENTS",
							Value: "3",
						},
						{
							Name:      "POD_IP",
							ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"}},
						},
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("5M"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot: ptr.To(false),
						RunAsGroup:   ptr.To[int64](0),
						RunAsUser:    ptr.To[int64](0),
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN"},
						},
					},
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: corev1.TerminationMessageReadFile,
				}
				Expect(deployment.Spec.Template.Spec.Containers[values.VPN.HighAvailabilityNumberOfSeedServers+1]).To(DeepEqual(pathControllerContainer))

				Expect(deployment.Spec.Template.Spec.Containers[0].Args).NotTo(ContainElement(ContainSubstring("--egress-selector-config-file=")))
				Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("http-proxy")})))
				Expect(deployment.Spec.Template.Spec.Volumes).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("http-proxy")})))

				hostPathCharDev := corev1.HostPathCharDev
				Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
					corev1.Volume{
						Name: "vpn-seed-client",
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								DefaultMode: ptr.To[int32](0640),
								Sources: []corev1.VolumeProjection{
									{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretNameCAVPN,
											},
											Items: []corev1.KeyToPath{{
												Key:  "bundle.crt",
												Path: "ca.crt",
											}},
										},
									},
									{
										Secret: &corev1.SecretProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretNameVPNSeedClient,
											},
											Items: []corev1.KeyToPath{
												{
													Key:  "tls.crt",
													Path: "tls.crt",
												},
												{
													Key:  "tls.key",
													Path: "tls.key",
												},
											},
										},
									},
								},
							},
						},
					},
					corev1.Volume{
						Name: "vpn-seed-tlsauth",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  secretNameVPNSeedServerTLSAuth,
								DefaultMode: ptr.To[int32](0640),
							},
						},
					},
					corev1.Volume{
						Name: "dev-net-tun",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/dev/net/tun",
								Type: &hostPathCharDev,
							},
						},
					},
				))
			}

			It("should have one init container and three vpn-seed-client sidecar containers when VPN high availability are enabled", func() {
				testHAVPN()
			})

			Context("kube-apiserver container", func() {
				var (
					acceptedIssuers  = []string{"issuer1", "issuer2"}
					admissionPlugin1 = "foo"
					admissionPlugin2 = "foo"
					admissionPlugins = []apiserver.AdmissionPluginConfig{
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: admissionPlugin1}},
						{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: admissionPlugin2}},
					}
					apiServerResources = corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("4Gi"),
						},
					}
					eventTTL                            = 2 * time.Hour
					externalHostname                    = "api.foo.bar.com"
					images                              = Images{KubeAPIServer: "some-kapi-image:latest"}
					serviceAccountIssuer                = "issuer"
					serviceAccountMaxTokenExpiration    = time.Hour
					serviceAccountExtendTokenExpiration = false
					serviceNetworkCIDRs                 = []net.IPNet{{IP: net.ParseIP("1.2.3.4"), Mask: net.CIDRMask(5, 32)}, {IP: net.ParseIP("2001:db8::"), Mask: net.CIDRMask(64, 128)}}
				)

				JustBeforeEach(func() {
					values = Values{
						Values: apiserver.Values{
							EnabledAdmissionPlugins: admissionPlugins,
							Logging: &gardencorev1beta1.APIServerLogging{
								Verbosity:           ptr.To[int32](3),
								HTTPAccessVerbosity: ptr.To[int32](3),
							},
							RuntimeVersion: runtimeVersion,
						},
						Autoscaling:      AutoscalingConfig{APIServerResources: apiServerResources},
						EventTTL:         &metav1.Duration{Duration: eventTTL},
						ExternalHostname: externalHostname,
						Images:           images,
						IsWorkerless:     true,
						ServiceAccount: ServiceAccountConfig{
							Issuer:                serviceAccountIssuer,
							AcceptedIssuers:       acceptedIssuers,
							MaxTokenExpiration:    &metav1.Duration{Duration: serviceAccountMaxTokenExpiration},
							ExtendTokenExpiration: &serviceAccountExtendTokenExpiration,
							JWKSURI:               ptr.To("https://foo.bar/jwks"),
						},
						ServiceNetworkCIDRs: serviceNetworkCIDRs,
						Version:             version,
						VPN:                 VPNConfig{},
					}
					kapi = New(kubernetesInterface, namespace, sm, values)
				})

				It("should have the kube-apiserver container with the expected spec when VPN is disabled and when there are no nodes", func() {
					values.VPN = VPNConfig{Enabled: false}
					kapi = New(kubernetesInterface, namespace, sm, values)
					deployAndRead()

					issuerIdx := indexOfElement(deployment.Spec.Template.Spec.Containers[0].Args, "--service-account-issuer="+serviceAccountIssuer)
					issuerIdx1 := indexOfElement(deployment.Spec.Template.Spec.Containers[0].Args, "--service-account-issuer="+acceptedIssuers[0])
					issuerIdx2 := indexOfElement(deployment.Spec.Template.Spec.Containers[0].Args, "--service-account-issuer="+acceptedIssuers[1])
					tlscipherSuites := kubernetesutils.TLSCipherSuites

					Expect(deployment.Spec.Template.Spec.Containers[0].Name).To(Equal("kube-apiserver"))
					Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal(images.KubeAPIServer))
					Expect(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
					Expect(deployment.Spec.Template.Spec.Containers[0].Command).To(ConsistOf("/usr/local/bin/kube-apiserver"))
					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ConsistOf(
						"--enable-admission-plugins="+admissionPlugin1+","+admissionPlugin2,
						"--admission-control-config-file=/etc/kubernetes/admission/admission-configuration.yaml",
						"--anonymous-auth=false",
						"--audit-policy-file=/etc/kubernetes/audit/audit-policy.yaml",
						"--authorization-mode=RBAC",
						"--client-ca-file=/srv/kubernetes/ca-client/bundle.crt",
						"--enable-aggregator-routing=true",
						"--enable-bootstrap-token-auth=true",
						"--http2-max-streams-per-connection=1000",
						"--etcd-cafile=/srv/kubernetes/etcd/ca/bundle.crt",
						"--etcd-certfile=/srv/kubernetes/etcd/client/tls.crt",
						"--etcd-keyfile=/srv/kubernetes/etcd/client/tls.key",
						"--etcd-servers=https://etcd-main-client:2379",
						"--etcd-servers-overrides=/events#https://etcd-events-client:2379",
						"--encryption-provider-config=/etc/kubernetes/etcd-encryption-secret/encryption-configuration.yaml",
						"--event-ttl="+eventTTL.String(),
						"--external-hostname="+externalHostname,
						"--livez-grace-period=1m",
						"--shutdown-delay-duration=15s",
						"--profiling=false",
						"--proxy-client-cert-file=/srv/kubernetes/aggregator/tls.crt",
						"--proxy-client-key-file=/srv/kubernetes/aggregator/tls.key",
						"--requestheader-client-ca-file=/srv/kubernetes/ca-front-proxy/bundle.crt",
						"--requestheader-extra-headers-prefix=X-Remote-Extra-",
						"--requestheader-group-headers=X-Remote-Group",
						"--requestheader-username-headers=X-Remote-User",
						"--runtime-config=apps/v1=false,autoscaling/v2=false,batch/v1=false,discovery.k8s.io/v1=false,policy/v1=false,storage.k8s.io/v1/csinodes=false",
						"--secure-port=443",
						"--service-cluster-ip-range="+serviceNetworkCIDRs[0].String()+","+serviceNetworkCIDRs[1].String(),
						"--service-account-issuer="+serviceAccountIssuer,
						"--service-account-issuer="+acceptedIssuers[0],
						"--service-account-issuer="+acceptedIssuers[1],
						"--service-account-jwks-uri=https://foo.bar/jwks",
						"--service-account-max-token-expiration="+serviceAccountMaxTokenExpiration.String(),
						"--service-account-extend-token-expiration="+strconv.FormatBool(serviceAccountExtendTokenExpiration),
						"--service-account-key-file=/srv/kubernetes/service-account-key-bundle/bundle.key",
						"--service-account-signing-key-file=/srv/kubernetes/service-account-key/id_rsa",
						"--token-auth-file=/srv/kubernetes/token/static_tokens.csv",
						"--tls-cert-file=/srv/kubernetes/apiserver/tls.crt",
						"--tls-private-key-file=/srv/kubernetes/apiserver/tls.key",
						"--tls-cipher-suites="+strings.Join(tlscipherSuites, ","),
						"--vmodule=httplog=3",
						"--v=3",
					))
					Expect(issuerIdx).To(BeNumerically(">=", 0))
					Expect(issuerIdx).To(BeNumerically("<", issuerIdx1))
					Expect(issuerIdx).To(BeNumerically("<", issuerIdx2))
					Expect(deployment.Spec.Template.Spec.Containers[0].TerminationMessagePath).To(Equal("/dev/termination-log"))
					Expect(deployment.Spec.Template.Spec.Containers[0].TerminationMessagePolicy).To(Equal(corev1.TerminationMessageReadFile))
					Expect(deployment.Spec.Template.Spec.Containers[0].Ports).To(ConsistOf(corev1.ContainerPort{
						Name:          "https",
						ContainerPort: int32(443),
						Protocol:      corev1.ProtocolTCP,
					}))
					Expect(deployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(apiServerResources))
					Expect(deployment.Spec.Template.Spec.Containers[0].SecurityContext).To(Equal(&corev1.SecurityContext{AllowPrivilegeEscalation: ptr.To(false)}))
					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ConsistOf(
						corev1.VolumeMount{
							Name:      "audit-policy-config",
							MountPath: "/etc/kubernetes/audit",
						},
						corev1.VolumeMount{
							Name:      "admission-config",
							MountPath: "/etc/kubernetes/admission",
						},
						corev1.VolumeMount{
							Name:      "admission-kubeconfigs",
							MountPath: "/etc/kubernetes/admission-kubeconfigs",
						},
						corev1.VolumeMount{
							Name:      "ca",
							MountPath: "/srv/kubernetes/ca",
						},
						corev1.VolumeMount{
							Name:      "ca-etcd",
							MountPath: "/srv/kubernetes/etcd/ca",
						},
						corev1.VolumeMount{
							Name:      "ca-client",
							MountPath: "/srv/kubernetes/ca-client",
						},
						corev1.VolumeMount{
							Name:      "ca-front-proxy",
							MountPath: "/srv/kubernetes/ca-front-proxy",
						},
						corev1.VolumeMount{
							Name:      "etcd-client",
							MountPath: "/srv/kubernetes/etcd/client",
						},
						corev1.VolumeMount{
							Name:      "server",
							MountPath: "/srv/kubernetes/apiserver",
						},
						corev1.VolumeMount{
							Name:      "service-account-key",
							MountPath: "/srv/kubernetes/service-account-key",
						},
						corev1.VolumeMount{
							Name:      "service-account-key-bundle",
							MountPath: "/srv/kubernetes/service-account-key-bundle",
						},
						corev1.VolumeMount{
							Name:      "static-token",
							MountPath: "/srv/kubernetes/token",
						},
						corev1.VolumeMount{
							Name:      "kube-aggregator",
							MountPath: "/srv/kubernetes/aggregator",
						},
						corev1.VolumeMount{
							Name:      "etcd-encryption-secret",
							MountPath: "/etc/kubernetes/etcd-encryption-secret",
							ReadOnly:  true,
						},
					))
					Expect(deployment.Spec.Template.Spec.Volumes).To(ConsistOf(
						corev1.Volume{
							Name: "audit-policy-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapNameAuditPolicy,
									},
								},
							},
						},
						corev1.Volume{
							Name: "admission-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapNameAdmissionConfigs,
									},
								},
							},
						},
						corev1.Volume{
							Name: "admission-kubeconfigs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameAdmissionKubeconfigs,
								},
							},
						},
						corev1.Volume{
							Name: "ca",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameCA,
								},
							},
						},
						corev1.Volume{
							Name: "ca-etcd",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameCAEtcd,
								},
							},
						},
						corev1.Volume{
							Name: "ca-client",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameCAClient,
								},
							},
						},
						corev1.Volume{
							Name: "ca-front-proxy",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameCAFrontProxy,
								},
							},
						},
						corev1.Volume{
							Name: "etcd-client",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameEtcd,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
						corev1.Volume{
							Name: "service-account-key",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameServiceAccountKey,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
						corev1.Volume{
							Name: "service-account-key-bundle",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameServiceAccountKeyBundle,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
						corev1.Volume{
							Name: "static-token",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameStaticToken,
								},
							},
						},
						corev1.Volume{
							Name: "kube-aggregator",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameKubeAggregator,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
						corev1.Volume{
							Name: "etcd-encryption-secret",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameETCDEncryptionConfig,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
						corev1.Volume{
							Name: "server",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameServer,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
					))

					secret := &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretNameStaticToken}, secret)).To(Succeed())
					Expect(secret.Data).To(HaveKey("static_tokens.csv"))
				})

				It("should have the kube-apiserver container with the expected spec when there are nodes", func() {
					values.IsWorkerless = false
					kapi = New(kubernetesInterface, namespace, sm, values)
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--allow-privileged=true",
						"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
						"--kubelet-certificate-authority=/srv/kubernetes/ca-kubelet/bundle.crt",
						"--kubelet-client-certificate=/srv/kubernetes/apiserver-kubelet/tls.crt",
						"--kubelet-client-key=/srv/kubernetes/apiserver-kubelet/tls.key",
					))
					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
						corev1.VolumeMount{
							Name:      "ca-kubelet",
							MountPath: "/srv/kubernetes/ca-kubelet",
						},
						corev1.VolumeMount{
							Name:      "kubelet-client",
							MountPath: "/srv/kubernetes/apiserver-kubelet",
						},
					))
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						corev1.Volume{
							Name: "ca-kubelet",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameCAKubelet,
								},
							},
						},
						corev1.Volume{
							Name: "kubelet-client",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameKubeAPIServerToKubelet,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
					))
				})

				It("should have the kube-apiserver container with the expected spec when VPN is enabled but HA is disabled", func() {
					values.VPN = VPNConfig{Enabled: true, HighAvailabilityEnabled: false}
					kapi = New(kubernetesInterface, namespace, sm, values)
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
						"--egress-selector-config-file=/etc/kubernetes/egress/egress-selector-configuration.yaml",
					))
					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
						corev1.VolumeMount{
							Name:      "ca-vpn",
							MountPath: "/srv/kubernetes/ca-vpn",
							ReadOnly:  false,
						},
						corev1.VolumeMount{
							Name:      "http-proxy",
							MountPath: "/etc/srv/kubernetes/envoy",
							ReadOnly:  false,
						},
						corev1.VolumeMount{
							Name:      "egress-selection-config",
							MountPath: "/etc/kubernetes/egress",
							ReadOnly:  false,
						},
					))
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						// VPN-related secrets (will be asserted in detail later)
						MatchFields(IgnoreExtras, Fields{"Name": Equal("ca-vpn")}),
						MatchFields(IgnoreExtras, Fields{"Name": Equal("http-proxy")}),
						MatchFields(IgnoreExtras, Fields{"Name": Equal("egress-selection-config")}),
					))
				})

				It("should have the kube-apiserver container with the expected spec when VPN and HA is enabled", func() {
					values.VPN = VPNConfig{Enabled: true, HighAvailabilityEnabled: true, PodNetworkCIDRs: []net.IPNet{{IP: net.ParseIP("9.8.7.6"), Mask: net.CIDRMask(24, 32)}}}
					kapi = New(kubernetesInterface, namespace, sm, values)
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						// VPN-related secrets (will be asserted in detail later)
						MatchFields(IgnoreExtras, Fields{"Name": Equal("vpn-seed-client")}),
						MatchFields(IgnoreExtras, Fields{"Name": Equal("vpn-seed-tlsauth")}),
						MatchFields(IgnoreExtras, Fields{"Name": Equal("dev-net-tun")}),
						MatchFields(IgnoreExtras, Fields{"Name": Equal("kube-api-access-gardener")}),
					))
				})

				It("should generate a kubeconfig secret for the user when StaticTokenKubeconfigEnabled is set to true", func() {
					kapi.EnableStaticTokenKubeconfig()

					deployAndRead()

					secretList := &corev1.SecretList{}
					Expect(c.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
						"name": "user-kubeconfig",
					})).To(Succeed())

					Expect(secretList.Items).To(HaveLen(1))
					Expect(secretList.Items[0].Data).To(HaveKey("kubeconfig"))

					kubeconfig := &clientcmdv1.Config{}
					Expect(yaml.Unmarshal(secretList.Items[0].Data["kubeconfig"], kubeconfig)).To(Succeed())
					Expect(kubeconfig.CurrentContext).To(Equal(namespace))
					Expect(kubeconfig.Clusters).To(HaveLen(1))
					Expect(kubeconfig.Clusters[0].Cluster.Server).To(Equal("https://" + externalHostname))
					Expect(kubeconfig.AuthInfos).To(HaveLen(1))
					Expect(kubeconfig.AuthInfos[0].AuthInfo.Token).NotTo(BeEmpty())
				})

				It("should generate kube-apiserver-static-token without system:cluster-admin token", func() {
					deployAndRead()

					secret := &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretNameStaticToken}, secret)).To(Succeed())
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						corev1.Volume{
							Name: "static-token",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameStaticToken,
								},
							},
						},
					))

					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						corev1.Volume{
							Name: "static-token",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameStaticToken,
								},
							},
						},
					))

					secret = &corev1.Secret{}
					Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretNameStaticToken}, secret)).To(Succeed())
					Expect(secret.Data).To(HaveKey("static_tokens.csv"))
				})

				It("should properly set the anonymous auth flag if enabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						AnonymousAuthenticationEnabled: ptr.To(true),
						Images:                         images,
						Version:                        version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(ContainSubstring(
						"--anonymous-auth=true",
					)))
				})

				It("should not set the anonymous auth flag if structured authentication configures anonymous auth", func() {
					authenticationConfig := `
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous:
  enabled: true
`

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Images:                      images,
						Version:                     semver.MustParse("1.31.0"),
						AuthenticationConfiguration: ptr.To(authenticationConfig),
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).ToNot(ContainElement(ContainSubstring(
						"--anonymous-auth",
					)))
				})

				It("should set the anonymous auth flag if structured authentication configures anonymous auth", func() {
					authenticationConfig := `
apiVersion: apiserver.config.k8s.io/v1beta1
kind: AuthenticationConfiguration
anonymous: null
`

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Images:                      images,
						Version:                     semver.MustParse("1.31.0"),
						AuthenticationConfiguration: ptr.To(authenticationConfig),
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(ContainSubstring(
						"--anonymous-auth=false",
					)))
				})

				It("should not set the anonymous auth flag if cluster is >= 1.32.0", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Images:  images,
						Version: semver.MustParse("1.32.0"),
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).ToNot(ContainElement(ContainSubstring(
						"--anonymous-auth",
					)))
				})

				It("should configure the advertise address if SNI is enabled", func() {
					advertiseAddress := "1.2.3.4"

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						SNI:     SNIConfig{Enabled: true, AdvertiseAddress: advertiseAddress},
						Images:  images,
						Version: version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
						"--advertise-address=" + advertiseAddress,
					))
				})

				It("should not configure the advertise address if SNI is enabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						SNI:     SNIConfig{Enabled: false, AdvertiseAddress: "foo"},
						Images:  images,
						Version: version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).NotTo(ContainElement(ContainSubstring("--advertise-address=")))
				})

				It("should configure the correct etcd overrides for etcd-events", func() {
					var (
						resourcesToStoreInETCDEvents = []schema.GroupResource{
							{Group: "networking.k8s.io", Resource: "networkpolicies"},
							{Group: "", Resource: "events"},
							{Group: "apps", Resource: "daemonsets"},
						}
					)

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						ResourcesToStoreInETCDEvents: resourcesToStoreInETCDEvents,
						Images:                       images,
						Version:                      version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
						"--etcd-servers-overrides=networking.k8s.io/networkpolicies#https://etcd-events-client:2379,/events#https://etcd-events-client:2379,apps/daemonsets#https://etcd-events-client:2379",
					))
				})

				It("should configure correctly when run as static pod", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion:  runtimeVersion,
							RunsAsStaticPod: true,
						},
						Images:  images,
						Version: version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--etcd-servers=https://localhost:2379",
						"--etcd-servers-overrides=/events#https://localhost:2382",
					))
				})

				It("should configure the api audiences if provided", func() {
					var (
						apiAudience1 = "foo"
						apiAudience2 = "bar"
						apiAudiences = []string{apiAudience1, apiAudience2}
					)

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						APIAudiences: apiAudiences,
						Images:       images,
						Version:      version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
						"--api-audiences=" + apiAudience1 + "," + apiAudience2,
					))
				})

				It("should not configure the api audiences if not provided", func() {
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).NotTo(ContainElement(ContainSubstring("--api-audiences=")))
				})

				It("should configure the feature gates if provided", func() {
					featureGates := map[string]bool{"Foo": true, "Bar": false}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							FeatureGates:   featureGates,
							RuntimeVersion: runtimeVersion,
						},
						Images:  images,
						Version: version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
						"--feature-gates=Bar=false,Foo=true",
					))
				})

				It("should not configure the feature gates if not provided", func() {
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).NotTo(ContainElement(ContainSubstring("--feature-gates=")))
				})

				It("should configure the request settings if provided", func() {
					requests := &gardencorev1beta1.APIServerRequests{
						MaxNonMutatingInflight: ptr.To[int32](123),
						MaxMutatingInflight:    ptr.To[int32](456),
					}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							Requests:       requests,
							RuntimeVersion: runtimeVersion,
						},
						Images:  images,
						Version: version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--max-requests-inflight=123",
						"--max-mutating-requests-inflight=456",
					))
				})

				It("should configure authentication config when authentication configuration is set", func() {
					var (
						authenticationConfigInput = &apiserverv1beta1.AuthenticationConfiguration{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "apiserver.config.k8s.io/v1beta1",
								Kind:       "AuthenticationConfiguration",
							},
							JWT: []apiserverv1beta1.JWTAuthenticator{
								{
									Issuer: apiserverv1beta1.Issuer{
										URL:       "https://foo.com",
										Audiences: []string{"example-client-id"},
									},
									ClaimMappings: apiserverv1beta1.ClaimMappings{
										Username: apiserverv1beta1.PrefixedClaimOrExpression{
											Claim:  "username",
											Prefix: ptr.To("foo:"),
										},
									},
								},
							},
						}

						version = semver.MustParse("1.30.0")
					)

					authenticationConfig, err := runtime.Encode(ConfigCodec, authenticationConfigInput)
					Expect(err).ToNot(HaveOccurred())

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						AuthenticationConfiguration: ptr.To(string(authenticationConfig)),
						Version:                     version,
					})

					configMapAuthentication = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
						Data:       map[string]string{"config.yaml": string(authenticationConfig)},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						AuthenticationConfiguration: ptr.To(string(authenticationConfig)),
						Version:                     version,
					})

					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						corev1.Volume{
							Name: "authentication-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapAuthentication.Name,
									},
								},
							},
						},
					))
					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
						corev1.VolumeMount{
							Name:      "authentication-config",
							MountPath: "/etc/kubernetes/structured/authentication",
						},
					))
					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--authentication-config=/etc/kubernetes/structured/authentication/config.yaml",
					))
				})

				It("should configure authentication config when oidc configuration is set", func() {
					var (
						oidc = &gardencorev1beta1.OIDCConfig{
							ClientID:  ptr.To("some-client-id"),
							IssuerURL: ptr.To("https://issuer.url.com"),
						}
						version              = semver.MustParse("1.30.0")
						authenticationConfig = `apiVersion: apiserver.config.k8s.io/v1beta1
jwt:
- claimMappings:
    groups: {}
    uid: {}
    username:
      claim: sub
      prefix: https://issuer.url.com#
  issuer:
    audiences:
    - some-client-id
    url: https://issuer.url.com
kind: AuthenticationConfiguration
`
					)

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						OIDC:    oidc,
						Version: version,
					})

					configMapAuthentication = &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-authentication-config", Namespace: namespace},
						Data:       map[string]string{"config.yaml": authenticationConfig},
					}
					Expect(kubernetesutils.MakeUnique(configMapAuthentication)).To(Succeed())

					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						corev1.Volume{
							Name: "authentication-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapAuthentication.Name,
									},
								},
							},
						},
					))
					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
						corev1.VolumeMount{
							Name:      "authentication-config",
							MountPath: "/etc/kubernetes/structured/authentication",
						},
					))
					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--authentication-config=/etc/kubernetes/structured/authentication/config.yaml",
					))
				})

				It("should not configure the request settings if not provided", func() {
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).NotTo(ContainElements(
						ContainSubstring("--max-requests-inflight="),
						ContainSubstring("--max-mutating-requests-inflight="),
					))
				})

				It("should configure the runtime config if provided", func() {
					runtimeConfig := map[string]bool{"foo": true, "bar": false}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						RuntimeConfig: runtimeConfig,
						Images:        images,
						Version:       version,
						IsWorkerless:  false,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
						"--runtime-config=bar=false,foo=true",
					))
				})

				It("should not configure the runtime config if not provided when shoot has workers", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Images:       images,
						Version:      version,
						IsWorkerless: false,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).NotTo(ContainElement(ContainSubstring("--runtime-config=")))
				})

				It("should allow to enable apis via 'RuntimeConfig' in case of workerless shoot", func() {
					runtimeConfig := map[string]bool{"apps/v1": true, "bar": false}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						RuntimeConfig: runtimeConfig,
						Images:        images,
						Version:       version,
						IsWorkerless:  true,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
						"--runtime-config=apps/v1=true,autoscaling/v2=false,bar=false,batch/v1=false,discovery.k8s.io/v1=false,policy/v1=false,storage.k8s.io/v1/csinodes=false",
					))
				})

				It("should configure the watch cache settings if provided", func() {
					watchCacheSizes := &gardencorev1beta1.WatchCacheSizes{
						Default: ptr.To[int32](123),
						Resources: []gardencorev1beta1.ResourceWatchCacheSize{
							{Resource: "foo", CacheSize: 456},
							{Resource: "bar", CacheSize: 789, APIGroup: ptr.To("baz")},
						},
					}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion:  runtimeVersion,
							WatchCacheSizes: watchCacheSizes,
						},
						Images:  images,
						Version: version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--default-watch-cache-size=123",
						"--watch-cache-sizes=foo#456,bar.baz#789",
					))
				})

				It("should not configure the watch cache settings if not provided", func() {
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).NotTo(ContainElements(
						ContainSubstring("--default-watch-cache-size="),
						ContainSubstring("--watch-cache-sizes="),
					))
				})

				It("should configure the defaultNotReadyTolerationSeconds and defaultUnreachableTolerationSeconds settings if provided", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						DefaultNotReadyTolerationSeconds:    ptr.To[int64](120),
						DefaultUnreachableTolerationSeconds: ptr.To[int64](130),
						Images:                              images,
						Version:                             version,
					})

					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--default-not-ready-toleration-seconds=120",
						"--default-unreachable-toleration-seconds=130",
					))
				})

				It("should configure the KubeAPISeverLogging settings if provided", func() {
					logging := &gardencorev1beta1.APIServerLogging{
						Verbosity:           ptr.To[int32](3),
						HTTPAccessVerbosity: ptr.To[int32](3),
					}

					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							Logging:        logging,
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--vmodule=httplog=3",
						"--v=3",
					))
				})

				It("should not configure the KubeAPISeverLogging settings if not provided", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Version: version,
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).NotTo(ContainElements(
						ContainSubstring("--vmodule=httplog"),
						ContainSubstring("--v="),
					))
				})

				It("should properly configure the settings related to reversed vpn if enabled", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Images:  images,
						Version: version,
						VPN:     VPNConfig{Enabled: true},
					})
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
						"--egress-selector-config-file=/etc/kubernetes/egress/egress-selector-configuration.yaml",
					))

					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
						corev1.VolumeMount{
							Name:      "ca-vpn",
							MountPath: "/srv/kubernetes/ca-vpn",
						},
						corev1.VolumeMount{
							Name:      "http-proxy",
							MountPath: "/etc/srv/kubernetes/envoy",
						},
						corev1.VolumeMount{
							Name:      "egress-selection-config",
							MountPath: "/etc/kubernetes/egress",
						},
					))

					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						corev1.Volume{
							Name: "ca-vpn",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretNameCAVPN,
								},
							},
						},
						corev1.Volume{
							Name: "http-proxy",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secretNameHTTPProxy,
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
						corev1.Volume{
							Name: "egress-selection-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kube-apiserver-egress-selector-config-53d92abc",
									},
								},
							},
						},
					))
				})

				It("should not configure the settings related to oidc if disabled", func() {
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("oidc-cabundle")})))
					Expect(deployment.Spec.Template.Spec.Volumes).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("oidc-cabundle")})))
				})

				It("should not configure the settings related to the service account signing key if not provided", func() {
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("oidc-cabundle")})))
					Expect(deployment.Spec.Template.Spec.Volumes).NotTo(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("oidc-cabundle")})))
				})

				It("should have the proper probes", func() {
					kapi = New(kubernetesInterface, namespace, sm, Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Images:  images,
						Version: semver.MustParse("1.31.1"),
					})
					deployAndRead()

					validateProbe := func(probe *corev1.Probe, path string, initialDelaySeconds int32) {
						Expect(probe.ProbeHandler.HTTPGet.Path).To(Equal(path))
						Expect(probe.ProbeHandler.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
						Expect(probe.ProbeHandler.HTTPGet.Port).To(Equal(intstr.FromInt32(443)))
						Expect(probe.ProbeHandler.HTTPGet.HTTPHeaders).To(HaveLen(1))
						Expect(probe.ProbeHandler.HTTPGet.HTTPHeaders[0].Name).To(Equal("Authorization"))
						Expect(probe.ProbeHandler.HTTPGet.HTTPHeaders[0].Value).To(ContainSubstring("Bearer "))
						Expect(len(probe.ProbeHandler.HTTPGet.HTTPHeaders[0].Value)).To(BeNumerically(">", 128))
						Expect(probe.SuccessThreshold).To(Equal(int32(1)))
						Expect(probe.FailureThreshold).To(Equal(int32(3)))
						Expect(probe.InitialDelaySeconds).To(Equal(initialDelaySeconds))
						Expect(probe.PeriodSeconds).To(Equal(int32(10)))
						Expect(probe.TimeoutSeconds).To(Equal(int32(15)))
					}

					validateProbe(deployment.Spec.Template.Spec.Containers[0].LivenessProbe, "/livez", 15)
					validateProbe(deployment.Spec.Template.Spec.Containers[0].ReadinessProbe, "/readyz", 10)
				})

				It("should have no lifecycle settings", func() {
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Lifecycle).To(BeNil())
				})

				It("should properly set the TLS SNI flag if necessary", func() {
					values.SNI.TLS = []TLSSNIConfig{
						{SecretName: ptr.To("existing-secret")},
						{Certificate: []byte("foo"), PrivateKey: []byte("bar"), DomainPatterns: []string{"foo1.com", "*.foo2.com"}},
					}
					kapi = New(kubernetesInterface, namespace, sm, values)
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--tls-sni-cert-key=/srv/kubernetes/tls-sni/0/tls.crt,/srv/kubernetes/tls-sni/0/tls.key",
						"--tls-sni-cert-key=/srv/kubernetes/tls-sni/1/tls.crt,/srv/kubernetes/tls-sni/1/tls.key:foo1.com,*.foo2.com",
					))
					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
						corev1.VolumeMount{
							Name:      "tls-sni-0",
							MountPath: "/srv/kubernetes/tls-sni/0",
							ReadOnly:  true,
						},
						corev1.VolumeMount{
							Name:      "tls-sni-1",
							MountPath: "/srv/kubernetes/tls-sni/1",
							ReadOnly:  true,
						},
					))
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						corev1.Volume{
							Name: "tls-sni-0",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "existing-secret",
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
						corev1.Volume{
							Name: "tls-sni-1",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "kube-apiserver-tls-sni-1-ec321de5",
									DefaultMode: ptr.To[int32](0640),
								},
							},
						},
					))
				})

				It("should properly configure the audit settings with webhook", func() {
					values.Audit = &apiserver.AuditConfig{
						Webhook: &apiserver.AuditWebhook{
							Kubeconfig:   []byte("foo"),
							BatchMaxSize: ptr.To[int32](30),
							Version:      ptr.To("audit.k8s.io/v1beta1"),
						},
					}
					kapi = New(kubernetesInterface, namespace, sm, values)
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--audit-webhook-config-file=/etc/kubernetes/webhook/audit/kubeconfig.yaml",
						"--audit-webhook-batch-max-size=30",
						"--audit-webhook-version=audit.k8s.io/v1beta1",
					))
					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
						corev1.VolumeMount{
							Name:      "audit-webhook-kubeconfig",
							MountPath: "/etc/kubernetes/webhook/audit",
							ReadOnly:  true,
						},
					))
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						corev1.Volume{
							Name: "audit-webhook-kubeconfig",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "kube-apiserver-audit-webhook-kubeconfig-50522102",
								},
							},
						},
					))
				})

				It("should properly configure the authentication settings with webhook", func() {
					values.AuthenticationWebhook = &AuthenticationWebhook{
						Kubeconfig: []byte("foo"),
						CacheTTL:   ptr.To(30 * time.Second),
						Version:    ptr.To("v1beta1"),
					}
					kapi = New(kubernetesInterface, namespace, sm, values)
					deployAndRead()

					Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
						"--authentication-token-webhook-config-file=/etc/kubernetes/webhook/authentication/kubeconfig.yaml",
						"--authentication-token-webhook-cache-ttl=30s",
						"--authentication-token-webhook-version=v1beta1",
					))
					Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
						corev1.VolumeMount{
							Name:      "authentication-webhook-kubeconfig",
							MountPath: "/etc/kubernetes/webhook/authentication",
							ReadOnly:  true,
						},
					))
					Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
						corev1.Volume{
							Name: "authentication-webhook-kubeconfig",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "kube-apiserver-authentication-webhook-kubeconfig-50522102",
								},
							},
						},
					))
				})

				Context("authorization settings", func() {
					It("should properly configure the authorization settings with webhook for Kubernetes < 1.30", func() {
						values.AuthorizationWebhooks = []AuthorizationWebhook{{
							Name: "foo",
							WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{
								AuthorizedTTL:              metav1.Duration{Duration: 13 * time.Second},
								UnauthorizedTTL:            metav1.Duration{Duration: 37 * time.Second},
								SubjectAccessReviewVersion: "v1alpha1",
							},
						}}
						kapi = New(kubernetesInterface, namespace, sm, values)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElements(
							"--authorization-webhook-config-file=/etc/kubernetes/structured/authorization-kubeconfigs/foo-kubeconfig.yaml",
							"--authorization-webhook-cache-authorized-ttl=13s",
							"--authorization-webhook-cache-unauthorized-ttl=37s",
							"--authorization-webhook-version=v1alpha1",
							"--authorization-mode=RBAC,Webhook",
						))
						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
							corev1.VolumeMount{
								Name:      "authorization-kubeconfigs",
								MountPath: "/etc/kubernetes/structured/authorization-kubeconfigs",
								ReadOnly:  true,
							},
						))
						Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
							corev1.Volume{
								Name: "authorization-kubeconfigs",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "kube-apiserver-authorization-webhooks-kubeconfigs-e3b0c442",
									},
								},
							},
						))
					})

					It("should properly configure the authorization settings with webhook for Kubernetes >= 1.30", func() {
						values.AuthorizationWebhooks = []AuthorizationWebhook{{
							Name: "foo",
							WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{
								AuthorizedTTL:              metav1.Duration{Duration: 13 * time.Second},
								UnauthorizedTTL:            metav1.Duration{Duration: 37 * time.Second},
								SubjectAccessReviewVersion: "v1alpha1",
							},
						}}
						values.Version = semver.MustParse("1.30.3")
						kapi = New(kubernetesInterface, namespace, sm, values)
						deployAndRead()

						Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(
							"--authorization-config=/etc/kubernetes/structured/authorization/config.yaml",
						))
						Expect(deployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElements(
							corev1.VolumeMount{
								Name:      "authorization-config",
								MountPath: "/etc/kubernetes/structured/authorization",
								ReadOnly:  true,
							},
							corev1.VolumeMount{
								Name:      "authorization-kubeconfigs",
								MountPath: "/etc/kubernetes/structured/authorization-kubeconfigs",
								ReadOnly:  true,
							},
						))
						Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElements(
							corev1.Volume{
								Name: "authorization-config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: "kube-apiserver-authorization-config-6cc78111"},
									},
								},
							},
							corev1.Volume{
								Name: "authorization-kubeconfigs",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "kube-apiserver-authorization-webhooks-kubeconfigs-e3b0c442",
									},
								},
							},
						))
					})
				})
			})
		})

		Describe("Role", func() {
			var (
				roleHAVPN = &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-apiserver-vpn-client-init",
						Namespace: namespace,
					},
				}
				roleBindingHAVPN = &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-apiserver-vpn-client-init",
						Namespace: namespace,
					},
				}
				serviceAccountHAVPN = &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-apiserver",
						Namespace: namespace,
					},
				}
			)

			objectsNotExisting := func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(roleHAVPN), roleHAVPN)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingHAVPN), roleBindingHAVPN)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(serviceAccountHAVPN), serviceAccountHAVPN)).To(BeNotFoundError())
			}

			deployAndRead := func() {
				objectsNotExisting()
				Expect(kapi.Deploy(ctx)).To(Succeed())
			}

			Context("HA VPN role", func() {
				It("should not deploy role, rolebinding and service account w/o HA VPN", func() {
					values := Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Images:              Images{VPNClient: "vpn-client-image:really-latest"},
						ServiceNetworkCIDRs: []net.IPNet{{IP: net.ParseIP("4.5.6.0"), Mask: net.CIDRMask(24, 32)}},
						VPN: VPNConfig{
							Enabled:                              true,
							HighAvailabilityEnabled:              false,
							HighAvailabilityNumberOfSeedServers:  2,
							HighAvailabilityNumberOfShootClients: 3,
							PodNetworkCIDRs:                      []net.IPNet{{IP: net.ParseIP("1.2.3.0"), Mask: net.CIDRMask(24, 32)}},
							NodeNetworkCIDRs:                     []net.IPNet{{IP: net.ParseIP("7.8.9.0"), Mask: net.CIDRMask(24, 32)}},
						},
						Version: version,
					}
					kapi = New(kubernetesInterface, namespace, sm, values)
					deployAndRead()
					objectsNotExisting()

					By("Destroy")
					Expect(kapi.Destroy(ctx)).To(Succeed())
					objectsNotExisting()
				})

				It("should successfully deploy and destroy the role, rolebinding and service account w/ HA VPN", func() {
					values := Values{
						Values: apiserver.Values{
							RuntimeVersion: runtimeVersion,
						},
						Images:              Images{VPNClient: "vpn-client-image:really-latest"},
						ServiceNetworkCIDRs: []net.IPNet{{IP: net.ParseIP("4.5.6.0"), Mask: net.CIDRMask(24, 32)}},
						VPN: VPNConfig{
							Enabled:                              true,
							HighAvailabilityEnabled:              true,
							HighAvailabilityNumberOfSeedServers:  2,
							HighAvailabilityNumberOfShootClients: 3,
							PodNetworkCIDRs:                      []net.IPNet{{IP: net.ParseIP("1.2.3.0"), Mask: net.CIDRMask(24, 32)}},
							NodeNetworkCIDRs:                     []net.IPNet{{IP: net.ParseIP("7.8.9.0"), Mask: net.CIDRMask(24, 32)}},
						},
						Version: version,
					}
					kapi = New(kubernetesInterface, namespace, sm, values)
					deployAndRead()

					Expect(c.Get(ctx, client.ObjectKeyFromObject(roleHAVPN), roleHAVPN)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(roleBindingHAVPN), roleBindingHAVPN)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(serviceAccountHAVPN), serviceAccountHAVPN)).To(Succeed())
					Expect(roleHAVPN.Rules).To(DeepEqual([]rbacv1.PolicyRule{
						{
							APIGroups: []string{""},
							Resources: []string{"pods"},
							Verbs:     []string{"get", "list", "watch", "patch", "update"},
						},
					}))
					Expect(roleBindingHAVPN.RoleRef).To(DeepEqual(rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "Role",
						Name:     roleHAVPN.Name,
					}))
					Expect(roleBindingHAVPN.Subjects).To(DeepEqual([]rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      serviceAccountHAVPN.Name,
							Namespace: namespace,
						},
					}))

					By("Destroy")
					Expect(kapi.Destroy(ctx)).To(Succeed())
					objectsNotExisting()
				})
			})
		})
	})

	Describe("#Destroy", func() {
		JustBeforeEach(func() {
			Expect(c.Create(ctx, deployment)).To(Succeed())
			Expect(c.Create(ctx, verticalPodAutoscaler)).To(Succeed())
			Expect(c.Create(ctx, managedResource)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
		})

		AfterEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
		})

		It("should delete all the resources successfully", func() {
			Expect(c.Create(ctx, horizontalPodAutoscaler)).To(Succeed())
			Expect(c.Create(ctx, podDisruptionBudget)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(Succeed())

			Expect(kapi.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(BeNotFoundError())
		})
	})

	Describe("#Wait", func() {
		JustBeforeEach(func() {
			deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: GetLabels()}
		})

		It("should successfully wait for the deployment to be updated", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeKubernetesInterface := fakekubernetes.NewClientSetBuilder().WithAPIReader(fakeClient).WithClient(fakeClient).Build()
			kapi = New(fakeKubernetesInterface, namespace, nil, Values{
				Values: apiserver.Values{
					RuntimeVersion: runtimeVersion,
				},
				Version: version,
			})
			deploy := deployment.DeepCopy()

			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 100*time.Millisecond)()

			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())

			Expect(fakeClient.Create(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod",
					Namespace: deployment.Namespace,
					Labels:    GetLabels(),
				},
			})).To(Succeed())

			timer := time.AfterFunc(10*time.Millisecond, func() {
				deploy.Generation = 24
				deploy.Spec.Replicas = ptr.To[int32](0)
				deploy.Status.Conditions = []appsv1.DeploymentCondition{
					{Type: appsv1.DeploymentProgressing, Status: "True", Reason: "NewReplicaSetAvailable"},
					{Type: appsv1.DeploymentAvailable, Status: "True"},
				}
				deploy.Status.ObservedGeneration = deploy.Generation
				deploy.Status.Replicas = *deploy.Spec.Replicas
				deploy.Status.UpdatedReplicas = *deploy.Spec.Replicas
				deploy.Status.AvailableReplicas = *deploy.Spec.Replicas
				Expect(fakeClient.Status().Update(ctx, deploy)).To(Succeed())
			})
			defer timer.Stop()

			Expect(kapi.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should successfully wait for the deployment to be deleted", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeKubernetesInterface := fakekubernetes.NewClientSetBuilder().WithAPIReader(fakeClient).WithClient(fakeClient).Build()
			kapi = New(fakeKubernetesInterface, namespace, nil, Values{})
			deploy := deployment.DeepCopy()

			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 100*time.Millisecond)()

			Expect(fakeClient.Create(ctx, deploy)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())

			timer := time.AfterFunc(10*time.Millisecond, func() {
				Expect(fakeClient.Delete(ctx, deploy)).To(Succeed())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(BeNotFoundError())
			})
			defer timer.Stop()

			Expect(kapi.WaitCleanup(ctx)).To(Succeed())
		})

		It("should time out while waiting for the deployment to be deleted", func() {
			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()
			defer test.WithVars(&TimeoutWaitForDeployment, 100*time.Millisecond)()

			Expect(c.Create(ctx, deployment)).To(Succeed())

			Expect(kapi.WaitCleanup(ctx)).To(MatchError(ContainSubstring("context deadline exceeded")))
		})

		It("should abort due to a severe error while waiting for the deployment to be deleted", func() {
			defer test.WithVars(&IntervalWaitForDeployment, time.Millisecond)()

			Expect(c.Create(ctx, deployment)).To(Succeed())

			scheme := runtime.NewScheme()
			clientWithoutScheme := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			kubernetesInterface2 := fakekubernetes.NewClientSetBuilder().WithClient(clientWithoutScheme).Build()
			kapi = New(kubernetesInterface2, namespace, nil, Values{})

			Expect(runtime.IsNotRegisteredError(kapi.WaitCleanup(ctx))).To(BeTrue())
		})
	})

	Describe("#SetAutoscalingAPIServerResources", func() {
		It("should properly set the field", func() {
			v := corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10Mi")}}
			kapi.SetAutoscalingAPIServerResources(v)
			Expect(kapi.GetValues().Autoscaling.APIServerResources).To(Equal(v))
		})
	})

	Describe("#GetAutoscalingReplicas", func() {
		It("should properly get the field", func() {
			v := ptr.To[int32](2)
			kapi.SetAutoscalingReplicas(v)
			Expect(kapi.GetAutoscalingReplicas()).To(Equal(v))
		})
	})

	Describe("#SetExternalServer", func() {
		It("should properly set the field", func() {
			v := "bar"
			kapi.SetExternalServer(v)
			Expect(kapi.GetValues().ExternalServer).To(Equal(v))
		})
	})

	Describe("#SetAutoscalingReplicas", func() {
		It("should properly set the field", func() {
			v := ptr.To[int32](2)
			kapi.SetAutoscalingReplicas(v)
			Expect(kapi.GetValues().Autoscaling.Replicas).To(Equal(v))
		})
	})

	Describe("#SetServiceAccountConfig", func() {
		It("should properly set the field", func() {
			v := ServiceAccountConfig{Issuer: "foo"}
			kapi.SetServiceAccountConfig(v)
			Expect(kapi.GetValues().ServiceAccount).To(Equal(v))
		})
	})

	Describe("#SetSNIConfig", func() {
		It("should properly set the field", func() {
			v := SNIConfig{AdvertiseAddress: "foo"}
			kapi.SetSNIConfig(v)
			Expect(kapi.GetValues().SNI).To(Equal(v))
		})
	})

	Describe("#SetExternalHostname", func() {
		It("should properly set the field", func() {
			v := "bar"
			kapi.SetExternalHostname(v)
			Expect(kapi.GetValues().ExternalHostname).To(Equal(v))
		})
	})

	Describe("#ComputeKubeAPIServerServiceAccountConfig", func() {
		externalHostname := "foo.bar"
		DescribeTable("should have the expected ServiceAccountConfig config", func(
			config *gardencorev1beta1.ServiceAccountConfig,
			rotationPhase gardencorev1beta1.CredentialsRotationPhase,
			expected ServiceAccountConfig,
		) {
			actual := ComputeKubeAPIServerServiceAccountConfig(config, externalHostname, rotationPhase)
			Expect(actual).To(Equal(expected))
		},
			Entry("ServiceAccountConfig is nil",
				nil,
				gardencorev1beta1.CredentialsRotationPhase(""),
				ServiceAccountConfig{
					Issuer: "https://" + externalHostname,
				},
			),
			Entry("ServiceAccountConfig is default",
				&gardencorev1beta1.ServiceAccountConfig{},
				gardencorev1beta1.CredentialsRotationPhase(""),
				ServiceAccountConfig{
					Issuer: "https://" + externalHostname,
				},
			),
			Entry("Service account key rotation phase is set",
				&gardencorev1beta1.ServiceAccountConfig{},
				gardencorev1beta1.RotationCompleting,
				ServiceAccountConfig{
					Issuer:        "https://" + externalHostname,
					RotationPhase: gardencorev1beta1.RotationCompleting,
				},
			),
			Entry("ExtendTokenExpiration and MaxTokenExpiration are set",
				&gardencorev1beta1.ServiceAccountConfig{
					ExtendTokenExpiration: ptr.To(true),
					MaxTokenExpiration:    &metav1.Duration{Duration: time.Second},
				},
				gardencorev1beta1.CredentialsRotationPhase(""),
				ServiceAccountConfig{
					Issuer:                "https://" + externalHostname,
					ExtendTokenExpiration: ptr.To(true),
					MaxTokenExpiration:    &metav1.Duration{Duration: time.Second},
				},
			),
			Entry("Issuer is set",
				&gardencorev1beta1.ServiceAccountConfig{
					Issuer: ptr.To("issuer"),
				},
				gardencorev1beta1.CredentialsRotationPhase(""),
				ServiceAccountConfig{
					Issuer:          "issuer",
					AcceptedIssuers: []string{"https://" + externalHostname},
				},
			),
			Entry("AcceptedIssuers is set and Issuer is not",
				&gardencorev1beta1.ServiceAccountConfig{
					AcceptedIssuers: []string{"issuer1", "issuer2"},
				},
				gardencorev1beta1.CredentialsRotationPhase(""),
				ServiceAccountConfig{
					Issuer:          "https://" + externalHostname,
					AcceptedIssuers: []string{"issuer1", "issuer2"},
				},
			),
			Entry("AcceptedIssuers and Issuer are both set",
				&gardencorev1beta1.ServiceAccountConfig{
					Issuer:          ptr.To("issuer"),
					AcceptedIssuers: []string{"issuer1", "issuer2"},
				},
				gardencorev1beta1.CredentialsRotationPhase(""),
				ServiceAccountConfig{
					Issuer:          "issuer",
					AcceptedIssuers: []string{"issuer1", "issuer2", "https://" + externalHostname},
				},
			),
			Entry("Default Issuer is already part of AcceptedIssuers",
				&gardencorev1beta1.ServiceAccountConfig{
					Issuer:          ptr.To("issuer"),
					AcceptedIssuers: []string{"https://" + externalHostname},
				},
				gardencorev1beta1.CredentialsRotationPhase(""),
				ServiceAccountConfig{
					Issuer:          "issuer",
					AcceptedIssuers: []string{"https://" + externalHostname},
				},
			),
			Entry("Default Issuer is already part of AcceptedIssuers but Issuer is not set",
				&gardencorev1beta1.ServiceAccountConfig{
					AcceptedIssuers: []string{"https://" + externalHostname},
				},
				gardencorev1beta1.CredentialsRotationPhase(""),
				ServiceAccountConfig{
					Issuer:          "https://" + externalHostname,
					AcceptedIssuers: []string{},
				},
			),
		)
	})
})

func egressSelectorConfigFor(controlPlaneName string) string {
	return `apiVersion: apiserver.k8s.io/v1alpha1
egressSelections:
- connection:
    proxyProtocol: HTTPConnect
    transport:
      tcp:
        tlsConfig:
          caBundle: /srv/kubernetes/ca-vpn/bundle.crt
          clientCert: /etc/srv/kubernetes/envoy/tls.crt
          clientKey: /etc/srv/kubernetes/envoy/tls.key
        url: https://vpn-seed-server:9443
  name: cluster
- connection:
    proxyProtocol: Direct
  name: ` + controlPlaneName + `
- connection:
    proxyProtocol: Direct
  name: etcd
kind: EgressSelectorConfiguration
`
}

func indexOfElement(elements []string, element string) int {
	for i, e := range elements {
		if e == element {
			return i
		}
	}
	return -1
}
