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
	corev1 "k8s.io/api/core/v1"
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

var _ = Describe("OpenTelemetry Collector", func() {
	var (
		ctx = context.Background()

		namespace                        = "some-namespace"
		image                            = "some-image:some-tag"
		lokiEndpoint                     = "logging"
		genericTokenKubeconfigSecretName = "generic-token-kubeconfig"
		kubeRBACProxyImage               = "kube-rbac-proxy:latest"
		values                           = Values{
			Image:              image,
			KubeRBACProxyImage: kubeRBACProxyImage,
		}

		c         client.Client
		component Interface
		consistOf func(...client.Object) types.GomegaMatcher

		customResourcesManagedResourceName   = "opentelemetry-collector"
		customResourcesManagedResource       *resourcesv1alpha1.ManagedResource
		customResourcesManagedResourceSecret *corev1.Secret
		fakeSecretManager                    secretsmanager.Interface
		kubeRBACProxyContainer               corev1.Container

		volume                 corev1.Volume
		volumeMount            corev1.VolumeMount
		openTelemetryCollector *otelv1beta1.OpenTelemetryCollector
	)

	BeforeEach(func() {
		format.MaxLength = 0
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(c, namespace)
		component = New(c, namespace, values, lokiEndpoint, fakeSecretManager)
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
		customResourcesManagedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + customResourcesManagedResource.Name,
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
									Name: "shoot-access-kube-rbac-proxy",
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

		kubeRBACProxyContainer = corev1.Container{
			Name:  "kube-rbac-proxy",
			Image: kubeRBACProxyImage,
			Args: []string{
				"--insecure-listen-address=0.0.0.0:8080",
				"--upstream=http://127.0.0.1:4317/",
				"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
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
					Name:          "kube-rbac-proxy",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				volumeMount,
			},
		}

		openTelemetryCollector = &otelv1beta1.OpenTelemetryCollector{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "opentelemetry-collector",
				Namespace: namespace,
				Labels:    getLabels(),
			},
			Spec: otelv1beta1.OpenTelemetryCollectorSpec{
				Mode:            "deployment",
				UpgradeStrategy: "none",
				OpenTelemetryCommonFields: otelv1beta1.OpenTelemetryCommonFields{
					Image:   image,
					Volumes: []corev1.Volume{volume},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
					},
					Ports: []otelv1beta1.PortsSpec{
						{
							ServicePort: corev1.ServicePort{
								Name: "kube-rbac-proxy",
								Port: 8080,
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
										"endpoint": "0.0.0.0:4317",
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
										"value":  "job, unit, nodename, origin, pod_name, container_name, origin, namespace_name, nodename, gardener_cloud_role",
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
						Pipelines: map[string]*otelv1beta1.Pipeline{
							"logs": {
								Exporters:  []string{"loki"},
								Receivers:  []string{"loki"},
								Processors: []string{"attributes/labels", "batch"},
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
			))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(Succeed())
			Expect(customResourcesManagedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(customResourcesManagedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(customResourcesManagedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
		})

		It("should successfully deploy all resources with kubeRBACProxy when AuthenticationProxy is true", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(BeNotFoundError())

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
			openTelemetryCollector.Spec.AdditionalContainers = append(openTelemetryCollector.Spec.AdditionalContainers, kubeRBACProxyContainer)
			Expect(customResourcesManagedResource).To(consistOf(
				openTelemetryCollector,
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
		v1beta1constants.LabelRole:  v1beta1constants.LabelObservability,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleObservability,
		gardenerutils.NetworkPolicyLabel(valiconstants.ServiceName, valiconstants.ValiPort): v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToDNS:                                            v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                               v1beta1constants.LabelNetworkPolicyAllowed,
		v1beta1constants.LabelObservabilityApplication:                                      "opentelemetry-collector",
	}
}
