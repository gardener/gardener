// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package alertmanager_test

import (
	"context"

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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Alertmanager", func() {
	var (
		ctx context.Context

		name                = "test"
		namespace           = "some-namespace"
		managedResourceName = "alertmanager-" + name

		image                    = "some-image"
		version                  = "v1.2.3"
		priorityClassName        = "priority-class"
		replicas           int32 = 1
		clusterType              = component.ClusterTypeSeed
		storageCapacity          = resource.MustParse("1337Gi")
		alertingSMTPSecret       = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "smtp-secret"},
			Data: map[string][]byte{
				"to":            []byte("secret-data1"),
				"from":          []byte("secret-data2"),
				"smarthost":     []byte("secret-data3"),
				"auth_username": []byte("secret-data4"),
				"auth_identity": []byte("secret-data5"),
				"auth_password": []byte("secret-data6"),
				"auth_type":     []byte("smtp"),
			},
		}

		ingressAuthSecretName     = "foo"
		ingressHost               = "some-host.example.com"
		ingressWildcardSecretName = "bar"

		fakeClient client.Client
		deployer   component.DeployWaiter
		values     Values

		fakeOps   *retryfake.Ops
		consistOf func(...client.Object) types.GomegaMatcher

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		service             *corev1.Service
		alertManager        *monitoringv1.Alertmanager
		vpa                 *vpaautoscalingv1.VerticalPodAutoscaler
		config              *monitoringv1alpha1.AlertmanagerConfig
		smtpSecret          *corev1.Secret
		ingress             *networkingv1.Ingress
		podDisruptionBudget *policyv1.PodDisruptionBudget
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		values = Values{
			Name:               name,
			Image:              image,
			Version:            version,
			PriorityClassName:  priorityClassName,
			StorageCapacity:    storageCapacity,
			Replicas:           replicas,
			ClusterType:        clusterType,
			AlertingSMTPSecret: alertingSMTPSecret,
		}

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

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

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"component":    "alertmanager",
					"role":         "monitoring",
					"alertmanager": name,
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":9093}]`,
					"networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports":   `[{"protocol":"TCP","port":9093}]`,
					"networking.resources.gardener.cloud/namespace-selectors":                          `[{"matchLabels":{"gardener.cloud/role":"shoot"}}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Selector: map[string]string{
					"component":    "alertmanager",
					"role":         "monitoring",
					"alertmanager": name,
				},
				Ports: []corev1.ServicePort{{
					Name: "metrics",
					Port: 9093,
				}},
			},
		}
		alertManager = &monitoringv1.Alertmanager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					"component":    "alertmanager",
					"role":         "monitoring",
					"alertmanager": name,
				},
			},
			Spec: monitoringv1.AlertmanagerSpec{
				PodMetadata: &monitoringv1.EmbeddedObjectMetadata{
					Labels: map[string]string{
						"alertmanager":                     name,
						"component":                        "alertmanager",
						"role":                             "monitoring",
						"networking.gardener.cloud/to-dns": "allowed",
						"networking.gardener.cloud/to-public-networks":  "allowed",
						"networking.gardener.cloud/to-private-networks": "allowed",
						v1beta1constants.LabelObservabilityApplication:  "alertmanager-" + name,
					},
				},
				PriorityClassName: priorityClassName,
				Replicas:          &replicas,
				Image:             &image,
				ImagePullPolicy:   corev1.PullIfNotPresent,
				Version:           version,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("20Mi"),
					},
				},
				SecurityContext: &corev1.PodSecurityContext{RunAsUser: ptr.To[int64](0)},
				Storage: &monitoringv1.StorageSpec{
					VolumeClaimTemplate: monitoringv1.EmbeddedPersistentVolumeClaim{
						EmbeddedObjectMetadata: monitoringv1.EmbeddedObjectMetadata{Name: "alertmanager-db"},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: storageCapacity}},
						},
					},
				},
				AlertmanagerConfigSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{"alertmanager": name}},
				AlertmanagerConfigNamespaceSelector: &metav1.LabelSelector{},
				AlertmanagerConfigMatcherStrategy:   monitoringv1.AlertmanagerConfigMatcherStrategy{Type: "None"},
				LogLevel:                            "info",
				ForceEnableClusterMode:              true,
				AlertmanagerConfiguration:           &monitoringv1.AlertmanagerConfiguration{Name: "alertmanager-" + name},
			},
		}
		vpaUpdateMode, vpaControlledValuesRequestsOnly, vpaContainerScalingModeOff := vpaautoscalingv1.UpdateModeAuto, vpaautoscalingv1.ContainerControlledValuesRequestsOnly, vpaautoscalingv1.ContainerScalingModeOff
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"component":    "alertmanager",
					"role":         "monitoring",
					"alertmanager": name,
					v1beta1constants.LabelObservabilityApplication: "alertmanager-" + name,
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "monitoring.coreos.com/v1",
					Kind:       "Alertmanager",
					Name:       name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "alertmanager",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("20Mi"),
							},
							MaxAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
							ControlledValues:    &vpaControlledValuesRequestsOnly,
							ControlledResources: &[]corev1.ResourceName{corev1.ResourceMemory},
						},
						{
							ContainerName: "config-reloader",
							Mode:          &vpaContainerScalingModeOff,
						},
					},
				},
			},
		}
		config = &monitoringv1alpha1.AlertmanagerConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-" + name,
				Namespace: namespace,
			},
			Spec: monitoringv1alpha1.AlertmanagerConfigSpec{
				Route: &monitoringv1alpha1.Route{
					GroupBy:        []string{"service"},
					GroupWait:      "5m",
					GroupInterval:  "5m",
					RepeatInterval: "72h",
					Receiver:       "dev-null",
					Routes:         []apiextensionsv1.JSON{{Raw: []byte(`{"matchers":[{"matchType":"=~","name":"visibility","value":"all|operator"}],"receiver":"email-kubernetes-ops"}`)}},
				},
				InhibitRules: []monitoringv1alpha1.InhibitRule{
					{
						SourceMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "critical", MatchType: monitoringv1alpha1.MatchEqual}},
						TargetMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "warning", MatchType: monitoringv1alpha1.MatchEqual}},
						Equal:       []string{"alertname", "service", "cluster"},
					},
					{
						SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "vpn", MatchType: monitoringv1alpha1.MatchEqual}},
						TargetMatch: []monitoringv1alpha1.Matcher{{Name: "type", Value: "shoot", MatchType: monitoringv1alpha1.MatchRegexp}},
						Equal:       []string{"type", "cluster"},
					},
					{
						SourceMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "blocker", MatchType: monitoringv1alpha1.MatchEqual}},
						TargetMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "^(critical|warning)$", MatchType: monitoringv1alpha1.MatchRegexp}},
						Equal:       []string{"cluster"},
					},
					{
						SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "kube-apiserver", MatchType: monitoringv1alpha1.MatchEqual}},
						TargetMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "nodes", MatchType: monitoringv1alpha1.MatchRegexp}},
						Equal:       []string{"cluster"},
					},
					{
						SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "kube-apiserver", MatchType: monitoringv1alpha1.MatchEqual}},
						TargetMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "info", MatchType: monitoringv1alpha1.MatchRegexp}},
						Equal:       []string{"cluster"},
					},
					{
						SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "kube-state-metrics-shoot", MatchType: monitoringv1alpha1.MatchEqual}},
						TargetMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "nodes", MatchType: monitoringv1alpha1.MatchRegexp}},
						Equal:       []string{"cluster"},
					},
				},
				Receivers: []monitoringv1alpha1.Receiver{
					{Name: "dev-null"},
					{
						Name: "email-kubernetes-ops",
						EmailConfigs: []monitoringv1alpha1.EmailConfig{{
							To:           string(alertingSMTPSecret.Data["to"]),
							From:         string(alertingSMTPSecret.Data["from"]),
							Smarthost:    string(alertingSMTPSecret.Data["smarthost"]),
							AuthUsername: string(alertingSMTPSecret.Data["auth_username"]),
							AuthIdentity: string(alertingSMTPSecret.Data["auth_identity"]),
							AuthPassword: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "alertmanager-" + name + "-smtp"},
								Key:                  "auth_password",
							},
						}},
					},
				},
			},
		}
		smtpSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-" + name + "-smtp",
				Namespace: namespace,
			},
			Type: alertingSMTPSecret.Type,
			Data: map[string][]byte{"auth_password": alertingSMTPSecret.Data["auth_password"]},
		}
		ingress = &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"component":    "alertmanager",
					"role":         "monitoring",
					"alertmanager": name,
				},
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/auth-type":   "basic",
					"nginx.ingress.kubernetes.io/auth-realm":  "Authentication Required",
					"nginx.ingress.kubernetes.io/auth-secret": ingressAuthSecretName,
					"nginx.ingress.kubernetes.io/server-snippet": `location /-/reload {
  return 403;
}`,
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
										Name: "alertmanager-" + name,
										Port: networkingv1.ServiceBackendPort{Number: 9093},
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
		podDisruptionBudget = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"component":    "alertmanager",
					"role":         "monitoring",
					"alertmanager": name,
				},
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: ptr.To(intstr.FromInt32(1)),
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
					"component":    "alertmanager",
					"role":         "monitoring",
					"alertmanager": name,
				}},
				UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
			},
		}
	})

	JustBeforeEach(func() {
		deployer = New(logr.Discard(), fakeClient, namespace, values)
	})

	Describe("#Deploy", func() {
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

		When("cluster type is 'seed'", func() {
			It("should successfully deploy all resources", func() {
				Expect(managedResource).To(consistOf(
					service,
					alertManager,
					vpa,
					config,
					smtpSecret,
				))
			})

			When("ingress is configured", func() {
				BeforeEach(func() {
					values.Ingress = &IngressValues{
						AuthSecretName:         ingressAuthSecretName,
						Host:                   ingressHost,
						WildcardCertSecretName: &ingressWildcardSecretName,
					}
				})

				It("should successfully deploy all resources", func() {
					alertManager.Spec.ExternalURL = "https://" + ingressHost

					Expect(managedResource).To(consistOf(
						service,
						alertManager,
						vpa,
						config,
						smtpSecret,
						ingress,
					))
				})
			})

			When("no alerting smtp secret is configured", func() {
				BeforeEach(func() {
					values.AlertingSMTPSecret = nil
				})

				It("should successfully deploy all resources", func() {
					alertManager.Spec.AlertmanagerConfiguration = nil

					Expect(managedResource).To(consistOf(
						service,
						alertManager,
						vpa,
					))
				})
			})

			When("email receivers are configured", func() {
				BeforeEach(func() {
					values.EmailReceivers = []string{"foo@example.bar", "bar@example.foo"}

					config.Spec.Receivers[1].EmailConfigs = []monitoringv1alpha1.EmailConfig{
						{
							To:           values.EmailReceivers[0],
							From:         string(alertingSMTPSecret.Data["from"]),
							Smarthost:    string(alertingSMTPSecret.Data["smarthost"]),
							AuthUsername: string(alertingSMTPSecret.Data["auth_username"]),
							AuthIdentity: string(alertingSMTPSecret.Data["auth_identity"]),
							AuthPassword: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "alertmanager-" + name + "-smtp"},
								Key:                  "auth_password",
							},
						},
						{
							To:           values.EmailReceivers[1],
							From:         string(alertingSMTPSecret.Data["from"]),
							Smarthost:    string(alertingSMTPSecret.Data["smarthost"]),
							AuthUsername: string(alertingSMTPSecret.Data["auth_username"]),
							AuthIdentity: string(alertingSMTPSecret.Data["auth_identity"]),
							AuthPassword: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "alertmanager-" + name + "-smtp"},
								Key:                  "auth_password",
							},
						},
					}
				})

				It("should successfully deploy all resources", func() {
					Expect(managedResource).To(consistOf(
						service,
						alertManager,
						vpa,
						config,
						smtpSecret,
					))
				})
			})

			When("there are more than 1 replicas", func() {
				BeforeEach(func() {
					values.Replicas = 2
					values.RuntimeVersion = semver.MustParse("1.29.1")
				})

				It("should successfully deploy all resources", func() {
					alertManager.Spec.PodMetadata.Labels["networking.resources.gardener.cloud/to-alertmanager-operated-tcp-9094"] = "allowed"
					alertManager.Spec.PodMetadata.Labels["networking.resources.gardener.cloud/to-alertmanager-operated-udp-9094"] = "allowed"
					alertManager.Spec.Replicas = ptr.To[int32](2)

					Expect(managedResource).To(consistOf(
						service,
						alertManager,
						vpa,
						config,
						smtpSecret,
						podDisruptionBudget,
					))
				})
			})
		})

		When("cluster type is 'shoot'", func() {
			BeforeEach(func() {
				values.ClusterType = component.ClusterTypeShoot

				service.Annotations = map[string]string{"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":9093}]`}
				alertManager.Labels["gardener.cloud/role"] = "monitoring"
				alertManager.Spec.PodMetadata.Labels["gardener.cloud/role"] = "monitoring"
				config.Spec.Route.Routes[0].Raw = []byte(`{"matchers":[{"matchType":"=~","name":"visibility","value":"all|owner"}],"receiver":"email-kubernetes-ops"}`)
			})

			It("should successfully deploy all resources", func() {
				Expect(managedResource).To(consistOf(
					service,
					alertManager,
					vpa,
					config,
					smtpSecret,
				))
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
