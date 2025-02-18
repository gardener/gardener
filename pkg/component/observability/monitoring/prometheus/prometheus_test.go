// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	apitypes "k8s.io/apimachinery/pkg/types"
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
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Prometheus", func() {
	type alertmanager struct {
		name      string
		namespace *string
	}

	var (
		ctx context.Context

		name                = "test"
		namespace           = "some-namespace"
		managedResourceName = "prometheus-" + name

		image                                 = "some-image"
		version                               = "v1.2.3"
		priorityClassName                     = "priority-class"
		replicas                        int32 = 1
		storageCapacity                       = resource.MustParse("1337Gi")
		retention                             = monitoringv1.Duration("1d")
		retentionSize                         = monitoringv1.ByteSize("5GB")
		externalLabels                        = map[string]string{"seed": "test"}
		additionalLabels                      = map[string]string{"foo": "bar"}
		alertmanagerName                      = "alertmgr-test"
		alertmanagerName2                     = "alertmgr-test-2"
		alertmanagerNamespace2                = "alertmanager-namespace-2"
		serviceAccountNameTargetCluster       = "target-cluster-service-account"

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
		prometheusFor                       func([]alertmanager, bool) *monitoringv1.Prometheus
		vpa                                 *vpaautoscalingv1.VerticalPodAutoscaler
		ingress                             *networkingv1.Ingress
		prometheusRule                      *monitoringv1.PrometheusRule
		serviceMonitor                      *monitoringv1.ServiceMonitor
		podMonitor                          *monitoringv1.PodMonitor
		scrapeConfig                        *monitoringv1alpha1.ScrapeConfig
		additionalConfigMap                 *corev1.ConfigMap
		secretAdditionalScrapeConfigs       *corev1.Secret
		secretAdditionalAlertmanagerConfigs *corev1.Secret
		secretRemoteWriteBasicAuth          *corev1.Secret
		podDisruptionBudget                 *policyv1.PodDisruptionBudget

		clusterRoleTarget        *rbacv1.ClusterRole
		clusterRoleBindingTarget *rbacv1.ClusterRoleBinding
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
		prometheusFor = func(alertmanagers []alertmanager, restrictToNamespace bool) *monitoringv1.Prometheus {
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
						Web: &monitoringv1.PrometheusWebSpec{
							MaxConnections: ptr.To[int32](1024),
						},
					},
					RuleSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},
					RuleNamespaceSelector: &metav1.LabelSelector{},
				},
			}

			if restrictToNamespace {
				obj.Spec.ServiceMonitorNamespaceSelector = nil
				obj.Spec.PodMonitorNamespaceSelector = nil
				obj.Spec.ProbeNamespaceSelector = nil
				obj.Spec.ScrapeConfigNamespaceSelector = nil
				obj.Spec.RuleNamespaceSelector = nil
			}

			if len(alertmanagers) > 0 {
				obj.Spec.Alerting = &monitoringv1.AlertingSpec{}
			}

			for _, alertmanager := range alertmanagers {
				obj.Spec.Alerting.Alertmanagers = append(obj.Spec.Alerting.Alertmanagers,
					monitoringv1.AlertmanagerEndpoints{
						Namespace: alertmanager.namespace,
						Name:      alertmanager.name,
						Port:      intstr.FromString("metrics"),
						AlertRelabelConfigs: []monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"ignoreAlerts"},
							Regex:        `true`,
							Action:       "drop",
						}},
					})
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
								corev1.ResourceMemory: resource.MustParse("100M"),
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
		secretAdditionalAlertmanagerConfigs = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name + "-additional-alertmanager-configs",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			Type: corev1.SecretTypeOpaque,
		}
		secretRemoteWriteBasicAuth = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name + "-remote-write-basic-auth",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"username": []byte("uname"),
				"password": []byte("pass"),
			},
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
				MaxUnavailable: ptr.To(intstr.FromInt32(1)),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				}},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}

		clusterRoleTarget = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:monitoring:prometheus-" + name,
			},
			Rules: []rbacv1.PolicyRule{{
				NonResourceURLs: []string{"/metrics"},
				Verbs:           []string{"get"},
			}},
		}
		clusterRoleBindingTarget = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:monitoring:prometheus-" + name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "gardener.cloud:monitoring:prometheus-" + name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      serviceAccountNameTargetCluster,
				Namespace: "kube-system",
			}},
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
					prometheusFor(nil, false),
					vpa,
					prometheusRule,
					scrapeConfig,
					serviceMonitor,
					podMonitor,
					secretAdditionalScrapeConfigs,
					additionalConfigMap,
				))
			})

			When("namespace UID is set", func() {
				BeforeEach(func() {
					values.NamespaceUID = ptr.To(apitypes.UID("foo"))
				})

				It("should successfully deploy all resources", func() {
					clusterRoleBinding.Name += "-foo"
					clusterRoleBinding.OwnerReferences = []metav1.OwnerReference{{
						APIVersion:         corev1.SchemeGroupVersion.String(),
						Kind:               "Namespace",
						Name:               namespace,
						UID:                "foo",
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					}}

					prometheusRule.Namespace = namespace
					metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&podMonitor.ObjectMeta, "prometheus", name)

					Expect(managedResource).To(consistOf(
						serviceAccount,
						service,
						clusterRoleBinding,
						prometheusFor(nil, false),
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

			When("cluster type is shoot", func() {
				BeforeEach(func() {
					values.ClusterType = component.ClusterTypeShoot
					values.RestrictToNamespace = true
				})

				It("should successfully deploy all resources", func() {
					service.Annotations = map[string]string{
						"networking.resources.gardener.cloud/pod-label-selector-namespace-alias": "all-shoots",
						"networking.resources.gardener.cloud/namespace-selectors":                `[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]`,
					}

					prometheusRule.Namespace = namespace
					metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
					metav1.SetMetaDataLabel(&podMonitor.ObjectMeta, "prometheus", name)

					Expect(managedResource).To(consistOf(
						serviceAccount,
						service,
						clusterRoleBinding,
						prometheusFor(nil, true),
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

			When("ingress is configured", func() {
				test := func() {
					It("should successfully deploy all resources", func() {
						prometheusObj := prometheusFor(nil, false)
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

					When("management APIs shall be blocked", func() {
						BeforeEach(func() {
							values.Ingress.BlockManagementAndTargetAPIAccess = true
						})

						It("should successfully deploy all resources", func() {
							prometheusObj := prometheusFor(nil, false)
							prometheusObj.Spec.ExternalURL = "https://" + ingressHost
							ingress.Annotations["nginx.ingress.kubernetes.io/server-snippet"] = `location /-/reload {
  return 403;
}
location /-/quit {
  return 403;
}
location /api/v1/targets {
  return 403;
}`

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
					})
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
					values.Alerting = &AlertingValues{Alertmanagers: []*Alertmanager{{Name: alertmanagerName}}}
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
						prometheusFor([]alertmanager{{name: alertmanagerName}}, false),
						vpa,
						prometheusRule,
						scrapeConfig,
						serviceMonitor,
						podMonitor,
						secretAdditionalScrapeConfigs,
						additionalConfigMap,
					))
				})

				When("an additional alertmanager is configured via the Alertmananagers slice", func() {
					BeforeEach(func() {
						values.Alerting.Alertmanagers = append(values.Alerting.Alertmanagers, &Alertmanager{Name: alertmanagerName2, Namespace: ptr.To(alertmanagerNamespace2)})
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
							prometheusFor(
								[]alertmanager{
									{name: alertmanagerName},
									{name: alertmanagerName2, namespace: &alertmanagerNamespace2}},
								false),
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

				When("additional alertmanagers are configured via the AdditionalAlertmanager field", func() {
					When("configured w/ basic auth", func() {
						BeforeEach(func() {
							values.Alerting.AdditionalAlertmanager = map[string][]byte{
								"auth_type": []byte("basic"),
								"url":       []byte("some-url"),
								"username":  []byte("uname"),
								"password":  []byte("pass"),
							}
						})

						It("should successfully deploy all resources", func() {
							prometheusObj := prometheusFor([]alertmanager{{name: alertmanagerName}}, false)
							prometheusObj.Spec.AdditionalAlertManagerConfigs = &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretAdditionalAlertmanagerConfigs.Name},
								Key:                  "configs.yaml",
							}

							secretAdditionalAlertmanagerConfigs.Data = map[string][]byte{"configs.yaml": []byte(`
static_configs:
- targets:
  - some-url
basic_auth:
  username: uname
  password: pass`)}

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
								secretAdditionalAlertmanagerConfigs,
							))
						})
					})

					When("configured w/ certificate", func() {
						BeforeEach(func() {
							values.Alerting.AdditionalAlertmanager = map[string][]byte{
								"auth_type":            []byte("certificate"),
								"url":                  []byte("some-url"),
								"ca.crt":               []byte("ca-crt"),
								"tls.crt":              []byte("tls-crt"),
								"tls.key":              []byte("tls-key"),
								"insecure_skip_verify": []byte("false"),
							}
						})

						It("should successfully deploy all resources", func() {
							prometheusObj := prometheusFor([]alertmanager{{name: alertmanagerName}}, false)
							prometheusObj.Spec.AdditionalAlertManagerConfigs = &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretAdditionalAlertmanagerConfigs.Name},
								Key:                  "configs.yaml",
							}

							secretAdditionalAlertmanagerConfigs.Data = map[string][]byte{"configs.yaml": []byte(`
static_configs:
- targets:
  - some-url
tls_config:
  ca: ca-crt
  cert: tls-crt
  key: tls-key
  insecure_skip_verify: false`)}

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
								secretAdditionalAlertmanagerConfigs,
							))
						})
					})
				})

				When("additional alert relabel configs are provided", func() {
					BeforeEach(func() {
						values.AdditionalAlertRelabelConfigs = []monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"project", "name"},
							Regex:        "(.+);(.+)",
							Action:       "replace",
							Replacement:  ptr.To("https://dashboard.ingress.gardener.cloud/namespace/garden-$1/shoots/$2"),
							TargetLabel:  "shoot_dashboard_url",
						}}
					})

					It("should successfully append the additional alert relabel config", func() {
						prometheusRule.Namespace = namespace
						metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
						metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", name)
						metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
						metav1.SetMetaDataLabel(&podMonitor.ObjectMeta, "prometheus", name)

						prometheus := prometheusFor([]alertmanager{{name: alertmanagerName}}, false)
						prometheus.Spec.Alerting.Alertmanagers[0].AlertRelabelConfigs = append(
							prometheus.Spec.Alerting.Alertmanagers[0].AlertRelabelConfigs,
							monitoringv1.RelabelConfig{
								SourceLabels: []monitoringv1.LabelName{"project", "name"},
								Regex:        "(.+);(.+)",
								Action:       "replace",
								Replacement:  ptr.To("https://dashboard.ingress.gardener.cloud/namespace/garden-$1/shoots/$2"),
								TargetLabel:  "shoot_dashboard_url",
							},
						)

						Expect(managedResource).To(contain(
							serviceAccount,
							service,
							clusterRoleBinding,
							prometheus,
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
			})

			When("there is more than 1 replica", func() {
				BeforeEach(func() {
					values.Replicas = 2
					values.RuntimeVersion = semver.MustParse("1.29.1")
				})

				It("should successfully deploy all resources", func() {
					prometheusObj := prometheusFor(nil, false)
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
					prometheusObj := prometheusFor(nil, false)
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

					prometheusObj := prometheusFor(nil, false)
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
					Expect(references.InjectAnnotations(prometheusObj)).To(Succeed())

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

			When("remote write config is provided", func() {
				BeforeEach(func() {
					values.RemoteWrite = &RemoteWriteValues{
						URL:         "rw-url",
						KeptMetrics: []string{"1", "2"},
					}
				})

				It("should successfully deploy all resources", func() {
					prometheusObj := prometheusFor(nil, false)
					prometheusObj.Spec.RemoteWrite = []monitoringv1.RemoteWriteSpec{{
						URL: "rw-url",
						WriteRelabelConfigs: []monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(1|2)$`,
						}},
					}}

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
					))
				})

				When("global shoot remote write secret is provided", func() {
					BeforeEach(func() {
						values.RemoteWrite.GlobalShootRemoteWriteSecret = &corev1.Secret{Data: map[string][]byte{
							"username": []byte(`uname`),
							"password": []byte(`pass`),
						}}
					})

					It("should successfully deploy all resources", func() {
						prometheusObj := prometheusFor(nil, false)
						prometheusObj.Spec.RemoteWrite = []monitoringv1.RemoteWriteSpec{{
							URL: "rw-url",
							WriteRelabelConfigs: []monitoringv1.RelabelConfig{{
								SourceLabels: []monitoringv1.LabelName{"__name__"},
								Action:       "keep",
								Regex:        `^(1|2)$`,
							}},
							BasicAuth: &monitoringv1.BasicAuth{
								Username: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: secretRemoteWriteBasicAuth.Name},
									Key:                  "username",
								},
								Password: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: secretRemoteWriteBasicAuth.Name},
									Key:                  "password",
								},
							},
						}}

						Expect(references.InjectAnnotations(prometheusObj)).To(Succeed())

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
							secretRemoteWriteBasicAuth,
						))
					})
				})
			})

			When("target cluster is configured", func() {
				var (
					managedResourceTarget       *resourcesv1alpha1.ManagedResource
					managedResourceSecretTarget *corev1.Secret
				)

				BeforeEach(func() {
					values.TargetCluster = &TargetClusterValues{
						ServiceAccountName: serviceAccountNameTargetCluster,
					}

					managedResourceTarget = &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:      managedResourceName + "-target",
							Namespace: namespace,
						},
					}
					managedResourceSecretTarget = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "managedresource-" + managedResource.Name,
							Namespace: namespace,
						},
					}

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceTarget), managedResourceTarget)).To(BeNotFoundError())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretTarget), managedResourceSecretTarget)).To(BeNotFoundError())

					Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceTarget.Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())
				})

				JustBeforeEach(func() {
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceTarget), managedResourceTarget)).To(Succeed())
					expectedTargetMr := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceTarget.Name,
							Namespace:       managedResourceTarget.Namespace,
							ResourceVersion: "2",
							Generation:      1,
							Labels: map[string]string{
								"origin":                             "gardener",
								"care.gardener.cloud/condition-type": "ObservabilityComponentsHealthy",
							},
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceTarget.Spec.SecretRefs[0].Name}},
							KeepObjects:  ptr.To(false),
							InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						},
						Status: healthyManagedResourceStatus,
					}
					utilruntime.Must(references.InjectAnnotations(expectedTargetMr))
					Expect(managedResourceTarget).To(Equal(expectedTargetMr))

					managedResourceSecretTarget.Name = managedResourceTarget.Spec.SecretRefs[0].Name
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretTarget), managedResourceSecretTarget)).To(Succeed())

					Expect(managedResourceSecretTarget.Type).To(Equal(corev1.SecretTypeOpaque))
					Expect(managedResourceSecretTarget.Immutable).To(Equal(ptr.To(true)))
					Expect(managedResourceSecretTarget.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
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
						prometheusFor(nil, false),
						vpa,
						prometheusRule,
						scrapeConfig,
						serviceMonitor,
						podMonitor,
						secretAdditionalScrapeConfigs,
						additionalConfigMap,
					))

					Expect(managedResourceTarget).To(consistOf(
						clusterRoleTarget,
						clusterRoleBindingTarget,
					))
				})

				When("it is configured that metrics are scraped from components in target cluster", func() {
					BeforeEach(func() {
						values.TargetCluster.ScrapesMetrics = true
					})

					It("should successfully deploy all resources", func() {
						clusterRoleTarget.Rules = append(clusterRoleTarget.Rules,
							rbacv1.PolicyRule{
								APIGroups: []string{""},
								Resources: []string{"nodes", "services", "endpoints", "pods"},
								Verbs:     []string{"get", "list", "watch"},
							},
							rbacv1.PolicyRule{
								APIGroups: []string{""},
								Resources: []string{"nodes/metrics", "pods/log", "nodes/proxy", "services/proxy", "pods/proxy"},
								Verbs:     []string{"get"},
							},
						)

						prometheusRule.Namespace = namespace
						metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
						metav1.SetMetaDataLabel(&scrapeConfig.ObjectMeta, "prometheus", name)
						metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
						metav1.SetMetaDataLabel(&podMonitor.ObjectMeta, "prometheus", name)

						Expect(managedResource).To(consistOf(
							serviceAccount,
							service,
							clusterRoleBinding,
							prometheusFor(nil, false),
							vpa,
							prometheusRule,
							scrapeConfig,
							serviceMonitor,
							podMonitor,
							secretAdditionalScrapeConfigs,
							additionalConfigMap,
						))

						Expect(managedResourceTarget).To(consistOf(
							clusterRoleTarget,
							clusterRoleBindingTarget,
						))
					})
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
