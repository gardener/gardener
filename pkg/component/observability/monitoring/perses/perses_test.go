// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package perses_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	persesv1alpha2 "github.com/perses/perses-operator/api/v1alpha2"
	persesconfig "github.com/perses/perses/pkg/model/api/config"
	persescommon "github.com/perses/spec/go/common"
	"github.com/perses/spec/go/datasource"
	persesplugin "github.com/perses/spec/go/plugin"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istionetworkingv1alpha3 "istio.io/api/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/monitoring/perses"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	comptest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Perses", func() {
	var (
		ctx context.Context

		managedResourceName = "perses"
		namespace           = "some-namespace"

		image = "perses-image:latest"

		fakeClient client.Client
		deployer   Interface
		values     Values

		fakeOps   *retryfake.Ops
		consistOf func(...client.Object) gomegatypes.GomegaMatcher

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		persesCR           *persesv1alpha2.Perses
		dsAggregate        *persesv1alpha2.PersesDatasource
		dsSeed             *persesv1alpha2.PersesDatasource
		dsGarden           *persesv1alpha2.PersesDatasource
		dsLongterm         *persesv1alpha2.PersesDatasource
		seedServiceMonitor *monitoringv1.ServiceMonitor

		newExpectedDatasource func(string, string, string, bool, string) *persesv1alpha2.PersesDatasource
	)

	BeforeEach(func() {
		ctx = context.Background()

		managedResourceName = "perses"

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		values = Values{
			Image:       image,
			ClusterType: component.ClusterTypeSeed,
			Replicas:    1,
		}

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient, comptest.CmpOptsForIstio()...)

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

		persesCR = &persesv1alpha2.Perses{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "perses-seed",
				Namespace: namespace,
				Labels: map[string]string{
					"app":                        "perses",
					"role":                       "monitoring",
					"app.kubernetes.io/instance": "perses-seed",
				},
			},
			Spec: persesv1alpha2.PersesSpec{
				Metadata: &persesv1alpha2.Metadata{
					Labels: map[string]string{
						v1beta1constants.LabelObservabilityApplication:                 "perses",
						v1beta1constants.LabelNetworkPolicyToDNS:                       v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:          v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelRole:                                     v1beta1constants.LabelMonitoring,
						gardenerutils.NetworkPolicyLabel("prometheus-aggregate", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("prometheus-seed", 9090):      v1beta1constants.LabelNetworkPolicyAllowed,
					},
				},
				Config: persesv1alpha2.PersesConfig{
					Config: persesconfig.Config{
						Security: persesconfig.Security{
							Readonly:   false,
							EnableAuth: false,
						},
						Database: persesconfig.Database{
							File: &persesconfig.File{
								Folder: "/perses",
							},
						},
						Frontend: persesconfig.Frontend{
							Explorer: persesconfig.Explorer{
								Enable: true,
							},
						},
					},
				},
				Replicas:      new(int32(1)),
				Image:         new(image),
				ContainerPort: new(int32(8080)),
				Resources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
				Service: &persesv1alpha2.PersesService{
					Annotations: map[string]string{
						"networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8080}]`,
					},
				},
				Storage: &persesv1alpha2.StorageConfiguration{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}

		newExpectedDatasource = func(dsName, pluginKind, url string, isDefault bool, instanceName string) *persesv1alpha2.PersesDatasource {
			var allowedEndpoints []any
			switch pluginKind {
			case "PrometheusDatasource":
				for _, pattern := range []string{
					"/api/v1/query",
					"/api/v1/query_range",
					"/api/v1/labels",
					"/api/v1/label/[a-zA-Z0-9_]+/values",
					"/api/v1/series",
					"/api/v1/metadata",
					"/api/v1/parse_query",
				} {
					allowedEndpoints = append(allowedEndpoints,
						map[string]any{"endpointPattern": pattern, "method": "GET"},
						map[string]any{"endpointPattern": pattern, "method": "POST"},
					)
				}
			case "VictoriaLogsDatasource":
				for _, pattern := range []string{
					"/select/logsql/query",
					"/select/logsql/stats_query_range",
					"/select/logsql/field_names",
					"/select/logsql/field_values",
				} {
					allowedEndpoints = append(allowedEndpoints,
						map[string]any{"endpointPattern": pattern, "method": "POST"},
					)
				}
			}

			return &persesv1alpha2.PersesDatasource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dsName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":  "perses",
						"role": "monitoring",
					},
				},
				Spec: persesv1alpha2.DatasourceSpec{
					Config: persesv1alpha2.Datasource{
						Spec: datasource.Spec{
							Display: &persescommon.Display{
								Name: dsName,
							},
							Default: isDefault,
							Plugin: persesplugin.Plugin{
								Kind: pluginKind,
								Spec: map[string]any{
									"proxy": map[string]any{
										"kind": "HTTPProxy",
										"spec": map[string]any{
											"url":              url,
											"allowedEndpoints": allowedEndpoints,
										},
									},
								},
							},
						},
					},
					InstanceSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      "app.kubernetes.io/instance",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{instanceName},
						}},
					},
				},
			}
		}

		dsAggregate = newExpectedDatasource("prometheus-aggregate", "PrometheusDatasource", "http://prometheus-aggregate:80", true, "perses-seed")
		dsSeed = newExpectedDatasource("prometheus-seed", "PrometheusDatasource", "http://prometheus-seed:80", false, "perses-seed")
		dsGarden = newExpectedDatasource("prometheus-garden", "PrometheusDatasource", "http://prometheus-garden:80", true, "perses-garden")
		dsLongterm = newExpectedDatasource("prometheus-longterm", "PrometheusDatasource", "http://prometheus-longterm:81", false, "perses-garden")

		seedServiceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: monitoringutils.ConfigObjectMeta("perses", namespace, "seed"),
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{
					"app.kubernetes.io/instance": "perses-seed",
				}},
				Endpoints: []monitoringv1.Endpoint{{
					TargetPort: new(intstr.FromInt32(8080)),
				}},
			},
		}
	})

	JustBeforeEach(func() {
		deployer = New(fakeClient, namespace, values)
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
				expectedMr := &resourcesv1alpha1.ManagedResource{
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
						Class:       new("seed"),
						SecretRefs:  []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}},
						KeepObjects: new(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(Equal(expectedMr))

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(new(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			})

			Context("seed cluster", func() {
				It("should successfully deploy all resources", func() {
					Expect(managedResource).To(consistOf(
						persesCR,
						dsAggregate,
						dsSeed,
						seedServiceMonitor,
					))
				})
			})

			Context("seed cluster with external exposure", func() {
				var (
					tlsSecret       *corev1.Secret
					gateway         *istionetworkingv1beta1.Gateway
					virtualService  *istionetworkingv1beta1.VirtualService
					destinationRule *istionetworkingv1beta1.DestinationRule
				)

				BeforeEach(func() {
					Expect(fakeClient.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "wildcard-tls", Namespace: namespace},
					})).To(Succeed())

					values.ExternalExposure = &ExposureValues{
						AuthSecretName:               "auth-secret",
						AuthSecretManaged:            true,
						Host:                         "perses-seed.example.com",
						IstioIngressGatewayLabels:    map[string]string{"istio": "ingressgateway"},
						IstioIngressGatewayNamespace: "istio-ingress",
						WildcardCertSecretName:       new("wildcard-tls"),
					}

					persesCR.Spec.Service.Annotations["networking.istio.io/exportTo"] = "istio-ingress"

					tlsSecret = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      namespace + "-perses-seed-wildcard-tls",
							Namespace: "istio-ingress",
							Labels: map[string]string{
								"app":  "perses",
								"role": "monitoring",
							},
						},
					}
					gateway = &istionetworkingv1beta1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "perses-seed",
							Namespace: namespace,
							Labels: map[string]string{
								"app":  "perses",
								"role": "monitoring",
							},
						},
						Spec: istionetworkingv1alpha3.Gateway{
							Selector: map[string]string{"istio": "ingressgateway"},
							Servers: []*istionetworkingv1alpha3.Server{{
								Port: &istionetworkingv1alpha3.Port{
									Name: "tls", Number: 443, Protocol: "HTTPS",
								},
								Hosts: []string{"perses-seed.example.com"},
								Tls: &istionetworkingv1alpha3.ServerTLSSettings{
									CredentialName: namespace + "-perses-seed-wildcard-tls",
									Mode:           istionetworkingv1alpha3.ServerTLSSettings_SIMPLE,
								},
							}},
						},
					}
					virtualService = &istionetworkingv1beta1.VirtualService{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "perses-seed",
							Namespace: namespace,
							Labels: map[string]string{
								"app":  "perses",
								"role": "monitoring",
								"reference.gardener.cloud/basic-auth-secret-name":    "auth-secret",
								"reference.gardener.cloud/basic-auth-server-name":    "istio-basic-auth-server",
								"reference.gardener.cloud/basic-auth-secret-managed": "true",
							},
						},
						Spec: istionetworkingv1alpha3.VirtualService{
							ExportTo: []string{"istio-ingress"},
							Gateways: []string{"perses-seed"},
							Hosts:    []string{"perses-seed.example.com"},
							Http: []*istionetworkingv1alpha3.HTTPRoute{
								{
									Name: "block-debug",
									Match: []*istionetworkingv1alpha3.HTTPMatchRequest{{
										Uri: &istionetworkingv1alpha3.StringMatch{
											MatchType: &istionetworkingv1alpha3.StringMatch_Prefix{Prefix: "/debug/"},
										},
									}},
									DirectResponse: &istionetworkingv1alpha3.HTTPDirectResponse{Status: 403},
								},
								{
									Name: "block-unsaved-proxy",
									Match: []*istionetworkingv1alpha3.HTTPMatchRequest{{
										Uri: &istionetworkingv1alpha3.StringMatch{
											MatchType: &istionetworkingv1alpha3.StringMatch_Prefix{Prefix: "/proxy/unsaved"},
										},
									}},
									DirectResponse: &istionetworkingv1alpha3.HTTPDirectResponse{Status: 403},
								},
								{
									Name: "block-api-writes",
									Match: []*istionetworkingv1alpha3.HTTPMatchRequest{{
										Uri: &istionetworkingv1alpha3.StringMatch{
											MatchType: &istionetworkingv1alpha3.StringMatch_Prefix{Prefix: "/api/v1/"},
										},
										Method: &istionetworkingv1alpha3.StringMatch{
											MatchType: &istionetworkingv1alpha3.StringMatch_Regex{Regex: "POST|PUT|PATCH|DELETE"},
										},
									}},
									DirectResponse: &istionetworkingv1alpha3.HTTPDirectResponse{Status: 403},
								},
								{
									Route: []*istionetworkingv1alpha3.HTTPRouteDestination{{
										Destination: &istionetworkingv1alpha3.Destination{
											Host: "perses-seed." + namespace + ".svc.cluster.local",
											Port: &istionetworkingv1alpha3.PortSelector{Number: 8080},
										},
									}},
								},
							},
						},
					}
					destinationRule = &istionetworkingv1beta1.DestinationRule{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "perses-seed",
							Namespace: namespace,
							Labels: map[string]string{
								"app":  "perses",
								"role": "monitoring",
							},
						},
						Spec: istionetworkingv1alpha3.DestinationRule{
							Host: "perses-seed." + namespace + ".svc.cluster.local",
							TrafficPolicy: &istionetworkingv1alpha3.TrafficPolicy{
								LoadBalancer: &istionetworkingv1alpha3.LoadBalancerSettings{
									LocalityLbSetting: &istionetworkingv1alpha3.LocalityLoadBalancerSetting{
										FailoverPriority: []string{"topology.kubernetes.io/zone"},
										Enabled:          &wrapperspb.BoolValue{Value: true},
									},
								},
								ConnectionPool: &istionetworkingv1alpha3.ConnectionPoolSettings{
									Tcp: &istionetworkingv1alpha3.ConnectionPoolSettings_TCPSettings{
										TcpKeepalive: &istionetworkingv1alpha3.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
											Time:     &durationpb.Duration{Seconds: 7200},
											Interval: &durationpb.Duration{Seconds: 75},
										},
										MaxConnectionDuration: &durationpb.Duration{Seconds: 86400},
									},
								},
								OutlierDetection: &istionetworkingv1alpha3.OutlierDetection{},
								Tls:              &istionetworkingv1alpha3.ClientTLSSettings{},
							},
							ExportTo: []string{"istio-ingress"},
						},
					}
				})

				It("should include istio exposure resources", func() {
					Expect(managedResource).To(consistOf(
						persesCR,
						dsAggregate,
						dsSeed,
						tlsSecret,
						gateway,
						virtualService,
						destinationRule,
						seedServiceMonitor,
					))
				})
			})

			Context("seed cluster with VPA enabled", func() {
				BeforeEach(func() {
					values.VPAEnabled = true
				})

				It("should include VPA resource", func() {
					Expect(managedResource).To(consistOf(
						persesCR,
						dsAggregate,
						dsSeed,
						&vpaautoscalingv1.VerticalPodAutoscaler{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "perses-seed",
								Namespace: namespace,
								Labels: map[string]string{
									"app":  "perses",
									"role": "monitoring",
								},
							},
							Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
								TargetRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "apps/v1",
									Kind:       "Deployment",
									Name:       "perses-seed",
								},
								UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
									UpdateMode: new(vpaautoscalingv1.UpdateModeAuto),
								},
								ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
									ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
										{
											ContainerName: "perses",
											MinAllowed: corev1.ResourceList{
												corev1.ResourceMemory: resource.MustParse("32Mi"),
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
						},
						seedServiceMonitor,
					))
				})
			})

			Context("seed cluster with VictoriaLogs enabled", func() {
				BeforeEach(func() {
					values.VictoriaLogsEnabled = true

					persesCR.Spec.Metadata.Labels[gardenerutils.NetworkPolicyLabel("logging-vl", 9428)] = v1beta1constants.LabelNetworkPolicyAllowed
				})

				It("should include VictoriaLogs datasource", func() {
					Expect(managedResource).To(consistOf(
						persesCR,
						dsAggregate,
						dsSeed,
						newExpectedDatasource("victorialogs", "VictoriaLogsDatasource", "http://logging-vl."+namespace+".svc:9428", false, "perses-seed"),
						seedServiceMonitor,
					))
				})
			})

			Context("garden cluster", func() {
				BeforeEach(func() {
					values.IsGardenCluster = true

					persesCR.Name = "perses-garden"
					persesCR.Labels["app.kubernetes.io/instance"] = "perses-garden"
					persesCR.Spec.Service = &persesv1alpha2.PersesService{
						Annotations: map[string]string{
							"networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports":   `[{"protocol":"TCP","port":8080}]`,
							"networking.resources.gardener.cloud/from-all-garden-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":8080}]`,
						},
					}
					persesCR.Spec.Metadata.Labels = map[string]string{
						v1beta1constants.LabelObservabilityApplication:                 "perses",
						v1beta1constants.LabelNetworkPolicyToDNS:                       v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:          v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelRole:                                     v1beta1constants.LabelMonitoring,
						gardenerutils.NetworkPolicyLabel("prometheus-garden", 9090):    v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("prometheus-longterm", 9091):  v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("prometheus-aggregate", 9090): v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel("prometheus-seed", 9090):      v1beta1constants.LabelNetworkPolicyAllowed,
					}
				})

				It("should successfully deploy all resources", func() {
					gardenServiceMonitor := &monitoringv1.ServiceMonitor{
						ObjectMeta: monitoringutils.ConfigObjectMeta("perses", namespace, "garden"),
						Spec: monitoringv1.ServiceMonitorSpec{
							Selector: metav1.LabelSelector{MatchLabels: map[string]string{
								"app.kubernetes.io/instance": "perses-garden",
							}},
							Endpoints: []monitoringv1.Endpoint{{
								TargetPort: new(intstr.FromInt32(8080)),
							}},
						},
					}
					Expect(managedResource).To(consistOf(
						persesCR,
						dsGarden,
						dsLongterm,
						gardenServiceMonitor,
					))
				})
			})

			Context("only datasources and dashboards", func() {
				BeforeEach(func() {
					values.OnlyDeployDatasourcesAndDashboards = true

					managedResourceName = "perses-seed-config-only"
					managedResource = &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:      managedResourceName,
							Namespace: namespace,
						},
					}
					managedResourceSecret = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "managedresource-" + managedResourceName,
							Namespace: namespace,
						},
					}

					Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceName,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())

					dsAggregate = newExpectedDatasource("prometheus-aggregate", "PrometheusDatasource", "http://prometheus-aggregate:80", false, "perses-garden")
				})

				It("should only deploy datasources", func() {
					dsSeed = newExpectedDatasource("prometheus-seed", "PrometheusDatasource", "http://prometheus-seed:80", false, "perses-garden")
					Expect(managedResource).To(consistOf(
						dsAggregate,
						dsSeed,
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
