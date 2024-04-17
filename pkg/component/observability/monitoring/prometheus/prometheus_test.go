// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus_test

import (
	"context"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
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
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Prometheus", func() {
	var (
		ctx context.Context

		name                = "test"
		namespace           = "some-namespace"
		managedResourceName = "prometheus-" + name

		image                   = "some-image"
		version                 = "v1.2.3"
		priorityClassName       = "priority-class"
		replicas          int32 = 1
		storageCapacity         = resource.MustParse("1337Gi")
		retention               = monitoringv1.Duration("1d")
		retentionSize           = monitoringv1.ByteSize("5GB")
		externalLabels          = map[string]string{"seed": "test"}
		additionalLabels        = map[string]string{"foo": "bar"}
		alertmanagerName        = "alertmgr-test"

		additionalScrapeConfig1 = `job_name: foo
honor_labels: false`
		additionalScrapeConfig2 = `job_name: bar
honor_labels: true`

		ingressAuthSecretName     = "foo"
		ingressHost               = "some-host.example.com"
		ingressWildcardSecretName = "bar"

		fakeClient client.Client
		deployer   Interface
		values     Values

		fakeOps   *retryfake.Ops
		consistOf func(...client.Object) types.GomegaMatcher
		contain   func(...client.Object) types.GomegaMatcher

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		serviceAccount                      *corev1.ServiceAccount
		service                             *corev1.Service
		clusterRoleBinding                  *rbacv1.ClusterRoleBinding
		prometheusFor                       func(string) *monitoringv1.Prometheus
		vpa                                 *vpaautoscalingv1.VerticalPodAutoscaler
		ingress                             *networkingv1.Ingress
		prometheusRule                      *monitoringv1.PrometheusRule
		serviceMonitor                      *monitoringv1.ServiceMonitor
		podMonitor                          *monitoringv1.PodMonitor
		scrapeConfig                        *monitoringv1alpha1.ScrapeConfig
		additionalConfigMap                 *corev1.ConfigMap
		secretAdditionalScrapeConfigs       *corev1.Secret
		secretAdditionalAlertRelabelConfigs *corev1.Secret
		podDisruptionBudget                 *policyv1.PodDisruptionBudget
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		values = Values{
			Name:                name,
			Image:               image,
			Version:             version,
			PriorityClassName:   priorityClassName,
			StorageCapacity:     storageCapacity,
			Replicas:            replicas,
			Retention:           &retention,
			RetentionSize:       retentionSize,
			ExternalLabels:      externalLabels,
			AdditionalPodLabels: additionalLabels,
		}

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)
		contain = NewManagedResourceContainsObjectsMatcher(fakeClient)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":9090}]`,
					"networking.resources.gardener.cloud/namespace-selectors":                        `[{"matchLabels":{"gardener.cloud/role":"shoot"}}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: map[string]string{"prometheus": name},
				Ports: []corev1.ServicePort{{
					Name:       "web",
					Port:       80,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(9090),
				}},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prometheus-" + name,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
				Annotations: map[string]string{"resources.gardener.cloud/delete-on-invalid-update": "true"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "prometheus",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "prometheus-" + name,
				Namespace: namespace,
			}},
		}
		prometheusFor = func(alertmanagerName string) *monitoringv1.Prometheus {
			obj := &monitoringv1.Prometheus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						"app":  "prometheus",
						"role": "monitoring",
						"name": name,
					},
				},
				Spec: monitoringv1.PrometheusSpec{
					Retention:          retention,
					RetentionSize:      retentionSize,
					EvaluationInterval: "1m",
					CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
						ScrapeInterval: "1m",
						ReloadStrategy: ptr.To(monitoringv1.HTTPReloadStrategyType),
						ExternalLabels: externalLabels,
						AdditionalScrapeConfigs: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "prometheus-" + name + "-additional-scrape-configs"},
							Key:                  "prometheus.yaml",
						},

						PodMetadata: &monitoringv1.EmbeddedObjectMetadata{
							Labels: map[string]string{
								"foo":                              "bar",
								"networking.gardener.cloud/to-dns": "allowed",
								"networking.gardener.cloud/to-runtime-apiserver": "allowed",
								v1beta1constants.LabelObservabilityApplication:   "prometheus-" + name,
							},
						},
						PriorityClassName: priorityClassName,
						Replicas:          ptr.To[int32](1),
						Shards:            ptr.To[int32](1),
						Image:             &image,
						ImagePullPolicy:   corev1.PullIfNotPresent,
						Version:           version,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("300m"),
								corev1.ResourceMemory: resource.MustParse("1000Mi"),
							},
						},
						ServiceAccountName: "prometheus-" + name,
						SecurityContext:    &corev1.PodSecurityContext{RunAsUser: ptr.To[int64](0)},
						Storage: &monitoringv1.StorageSpec{
							VolumeClaimTemplate: monitoringv1.EmbeddedPersistentVolumeClaim{
								EmbeddedObjectMetadata: monitoringv1.EmbeddedObjectMetadata{Name: "prometheus-db"},
								Spec: corev1.PersistentVolumeClaimSpec{
									AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
									Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: storageCapacity}},
								},
							},
						},

						ServiceMonitorSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},
						PodMonitorSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},
						ProbeSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},
						ScrapeConfigSelector:   &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},

						ServiceMonitorNamespaceSelector: &metav1.LabelSelector{},
						PodMonitorNamespaceSelector:     &metav1.LabelSelector{},
						ProbeNamespaceSelector:          &metav1.LabelSelector{},
						ScrapeConfigNamespaceSelector:   &metav1.LabelSelector{},
					},
					RuleSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},
					RuleNamespaceSelector: &metav1.LabelSelector{},
				},
			}

			if alertmanagerName != "" {
				obj.Spec.Alerting = &monitoringv1.AlertingSpec{
					Alertmanagers: []monitoringv1.AlertmanagerEndpoints{{
						Namespace: namespace,
						Name:      alertmanagerName,
						Port:      intstr.FromString("metrics"),
					}},
				}
				obj.Spec.AdditionalAlertRelabelConfigs = &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "prometheus-" + name + "-additional-alert-relabel-configs"},
					Key:                  "configs.yaml",
				}
			}

			return obj
		}
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
					v1beta1constants.LabelObservabilityApplication: "prometheus-" + name,
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "monitoring.coreos.com/v1",
					Kind:       "Prometheus",
					Name:       name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "prometheus",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1000M"),
							},
							MaxAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("28G"),
							},
							ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
						},
						{
							ContainerName: "config-reloader",
							Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
						},
					},
				},
			},
		}
		ingress = &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/auth-type":   "basic",
					"nginx.ingress.kubernetes.io/auth-realm":  "Authentication Required",
					"nginx.ingress.kubernetes.io/auth-secret": ingressAuthSecretName,
				},
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: ptr.To(v1beta1constants.SeedNginxIngressClass),
				TLS: []networkingv1.IngressTLS{{
					SecretName: ingressWildcardSecretName,
					Hosts:      []string{ingressHost},
				}},
				Rules: []networkingv1.IngressRule{{
					Host: ingressHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Backend: networkingv1.IngressBackend{
									Service: &networkingv1.IngressServiceBackend{
										Name: "prometheus-" + name,
										Port: networkingv1.ServiceBackendPort{Number: 80},
									},
								},
								Path:     "/",
								PathType: ptr.To(networkingv1.PathTypePrefix),
							}},
						},
					},
				}},
			},
		}
		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "rule",
				Labels: map[string]string{"foo": "bar"},
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{Name: "foo"}},
			},
		}
		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "monitor",
				Namespace: "default",
				Labels:    map[string]string{"foo": "bar"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				JobLabel: "foo",
			},
		}
		podMonitor = &monitoringv1.PodMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "monitor",
				Namespace: "default",
				Labels:    map[string]string{"foo": "bar"},
			},
			Spec: monitoringv1.PodMonitorSpec{
				JobLabel: "foo",
			},
		}
		scrapeConfig = &monitoringv1alpha1.ScrapeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "scrape",
				Namespace: "default",
				Labels:    map[string]string{"foo": "bar"},
			},
			Spec: monitoringv1alpha1.ScrapeConfigSpec{
				Scheme: ptr.To("baz"),
			},
		}
		additionalConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "configmap", Namespace: namespace},
		}
		secretAdditionalScrapeConfigs = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name + "-additional-scrape-configs",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"prometheus.yaml": []byte(`- job_name: foo
  honor_labels: false
- job_name: bar
  honor_labels: true
`)},
		}
		secretAdditionalAlertRelabelConfigs = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name + "-additional-alert-relabel-configs",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"configs.yaml": []byte(`
- source_labels: [ ignoreAlerts ]
  regex: true
  action: drop
`)},
		}
		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: utils.IntStrPtrFromInt32(1),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				}},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}
	})

	JustBeforeEach(func() {
		values.AdditionalResources = append(values.AdditionalResources, additionalConfigMap)
		values.CentralConfigs.AdditionalScrapeConfigs = append(values.CentralConfigs.AdditionalScrapeConfigs, additionalScrapeConfig1, additionalScrapeConfig2)
		values.CentralConfigs.PrometheusRules = append(values.CentralConfigs.PrometheusRules, prometheusRule)
		values.CentralConfigs.ScrapeConfigs = append(values.CentralConfigs.ScrapeConfigs, scrapeConfig)
		values.CentralConfigs.ServiceMonitors = append(values.CentralConfigs.ServiceMonitors, serviceMonitor)
		values.CentralConfigs.PodMonitors = append(values.CentralConfigs.PodMonitors, podMonitor)

		deployer = New(logr.Discard(), fakeClient, namespace, values)
	})

	Describe("#Deploy", func() {
		Context("resources generation", func() {
			BeforeEach(func() {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())
			})

			JustBeforeEach(func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedRuntimeMr := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels: map[string]string{
							"gardener.cloud/role":                "seed-system-component",
							"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class:       ptr.To("seed"),
						SecretRefs:  []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}},
						KeepObjects: ptr.To(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedRuntimeMr))
				Expect(managedResource).To(Equal(expectedRuntimeMr))

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			})

			It("should successfully deploy all resources", func() {
				prometheusRule.Namespace = namespace
				metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
				metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", name)
				metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
				metav1.SetMetaDataLabel(&podMonitor.ObjectMeta, "prometheus", name)

				Expect(managedResource).To(consistOf(
					serviceAccount,
					service,
					clusterRoleBinding,
					prometheusFor(""),
					vpa,
					prometheusRule,
					scrapeConfig,
					serviceMonitor,
					podMonitor,
					secretAdditionalScrapeConfigs,
					additionalConfigMap,
				))
			})

			When("ingress is configured", func() {
				test := func() {
					It("should successfully deploy all resources", func() {
						prometheusObj := prometheusFor("")
						prometheusObj.Spec.ExternalURL = "https://" + ingressHost

						prometheusRule.Namespace = namespace
						metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
						metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", name)
						metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
						metav1.SetMetaDataLabel(&podMonitor.ObjectMeta, "prometheus", name)

						Expect(managedResource).To(consistOf(
							serviceAccount,
							service,
							clusterRoleBinding,
							prometheusObj,
							vpa,
							prometheusRule,
							scrapeConfig,
							serviceMonitor,
							podMonitor,
							secretAdditionalScrapeConfigs,
							additionalConfigMap,
							ingress,
						))
					})
				}

				Context("early initialization", func() {
					BeforeEach(func() {
						values.Ingress = &IngressValues{
							AuthSecretName:         ingressAuthSecretName,
							Host:                   ingressHost,
							WildcardCertSecretName: &ingressWildcardSecretName,
						}
						deployer = New(logr.Discard(), fakeClient, namespace, values)
					})

					test()
				})

				Context("late initialization", func() {
					BeforeEach(func() {
						values.Ingress = &IngressValues{Host: ingressHost}
						deployer = New(logr.Discard(), fakeClient, namespace, values)
						deployer.SetIngressAuthSecret(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ingressAuthSecretName}})
						deployer.SetIngressWildcardCertSecret(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ingressWildcardSecretName}})
					})

					test()
				})
			})

			When("alerting is configured", func() {
				BeforeEach(func() {
					values.Alerting = &AlertingValues{AlertmanagerName: alertmanagerName}
					deployer = New(logr.Discard(), fakeClient, namespace, values)
				})

				It("should successfully deploy all resources", func() {
					prometheusRule.Namespace = namespace
					metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&podMonitor.ObjectMeta, "prometheus", name)

					Expect(managedResource).To(contain(
						serviceAccount,
						service,
						clusterRoleBinding,
						prometheusFor(alertmanagerName),
						vpa,
						prometheusRule,
						scrapeConfig,
						serviceMonitor,
						podMonitor,
						secretAdditionalScrapeConfigs,
						additionalConfigMap,
						secretAdditionalAlertRelabelConfigs,
					))
				})
			})

			When("there is more than 1 replica", func() {
				BeforeEach(func() {
					values.Replicas = 2
					values.RuntimeVersion = semver.MustParse("1.29.1")
				})

				It("should successfully deploy all resources", func() {
					prometheusObj := prometheusFor("")
					prometheusObj.Spec.Replicas = ptr.To(int32(2))

					prometheusRule.Namespace = namespace
					metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&podMonitor.ObjectMeta, "prometheus", name)

					Expect(managedResource).To(contain(
						serviceAccount,
						service,
						clusterRoleBinding,
						prometheusObj,
						vpa,
						prometheusRule,
						scrapeConfig,
						serviceMonitor,
						podMonitor,
						secretAdditionalScrapeConfigs,
						additionalConfigMap,
						podDisruptionBudget,
					))
				})
			})

			When("scrape timeout is configured", func() {
				BeforeEach(func() {
					values.ScrapeTimeout = "10s"
				})

				It("should successfully deploy all resources", func() {
					prometheusObj := prometheusFor("")
					prometheusObj.Spec.ScrapeTimeout = "10s"

					prometheusRule.Namespace = namespace
					metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&podMonitor.ObjectMeta, "prometheus", name)

					Expect(managedResource).To(contain(
						serviceAccount,
						service,
						clusterRoleBinding,
						prometheusObj,
						vpa,
						prometheusRule,
						scrapeConfig,
						serviceMonitor,
						podMonitor,
						secretAdditionalScrapeConfigs,
						additionalConfigMap,
					))
				})
			})

			When("cortex sidecar is enabled", func() {
				var (
					cortexImage   = "cortex-image"
					cacheValidity = time.Second
				)

				BeforeEach(func() {
					values.Cortex = &CortexValues{Image: cortexImage, CacheValidity: cacheValidity}
				})

				It("should successfully deploy all resources", func() {
					cortexConfigMap := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "prometheus-" + name + "-cortex",
							Namespace: namespace,
							Labels: map[string]string{
								"app":  "prometheus",
								"role": "monitoring",
								"name": name,
							},
						},
						Data: map[string]string{"config.yaml": `target: query-frontend
auth_enabled: false
http_prefix:
api:
  response_compression_enabled: true
server:
  http_listen_port: 9091
frontend:
  downstream_url: http://localhost:9090
  log_queries_longer_than: -1s
query_range:
  split_queries_by_interval: 24h
  align_queries_with_step: true
  cache_results: true
  results_cache:
    cache:
      enable_fifocache: true
      fifocache:
        max_size_bytes: ` + storageCapacity.String() + `
        validity: ` + cacheValidity.String() + `
`},
					}
					Expect(kubernetesutils.MakeUnique(cortexConfigMap)).To(Succeed())

					prometheusObj := prometheusFor("")
					prometheusObj.Spec.Containers = append(prometheusObj.Spec.Containers, corev1.Container{
						Name:            "cortex",
						Image:           cortexImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args: []string{
							"-target=query-frontend",
							"-config.file=/etc/cortex/config/config.yaml",
						},
						Ports: []corev1.ContainerPort{{
							Name:          "frontend",
							ContainerPort: 9091,
							Protocol:      corev1.ProtocolTCP,
						}},
						SecurityContext: &corev1.SecurityContext{ReadOnlyRootFilesystem: ptr.To(true)},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("300Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "cortex-config",
							MountPath: "/etc/cortex/config",
							ReadOnly:  true,
						}},
					})
					prometheusObj.Spec.Volumes = append(prometheusObj.Spec.Volumes, corev1.Volume{
						Name:         "cortex-config",
						VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: cortexConfigMap.Name}}},
					})

					service.Spec.Ports[0].TargetPort = intstr.FromInt32(9091)

					vpa.Spec.ResourcePolicy.ContainerPolicies = append(vpa.Spec.ResourcePolicy.ContainerPolicies, vpaautoscalingv1.ContainerResourcePolicy{
						ContainerName: "cortex",
						Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
					})

					prometheusRule.Namespace = namespace
					metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&podMonitor.ObjectMeta, "prometheus", name)

					Expect(managedResource).To(contain(
						serviceAccount,
						service,
						clusterRoleBinding,
						prometheusObj,
						vpa,
						prometheusRule,
						scrapeConfig,
						serviceMonitor,
						podMonitor,
						secretAdditionalScrapeConfigs,
						additionalConfigMap,
						cortexConfigMap,
					))
				})
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should succeed because the ManagedResource is healthy and progressing", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
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
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it is already removed", func() {
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
