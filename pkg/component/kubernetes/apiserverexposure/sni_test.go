// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverexposure_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	comptest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#SNI", func() {
	var (
		ctx context.Context
		c   client.Client

		defaultDepWaiter component.DeployWaiter
		namespace        = "test-namespace"
		istioLabels      = map[string]string{"foo": "bar"}
		istioNamespace   = "istio-foo"
		hosts            = []string{"foo.bar"}
		hostName         = "kube-apiserver." + namespace + ".svc.cluster.local"

		apiServerProxyValues *APIServerProxy

		expectedDestinationRule       *istionetworkingv1beta1.DestinationRule
		expectedGateway               *istionetworkingv1beta1.Gateway
		expectedVirtualService        *istionetworkingv1beta1.VirtualService
		expectedEnvoyFilterObjectMeta metav1.ObjectMeta
		expectedManagedResource       *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).To(Succeed())
		Expect(resourcesv1alpha1.AddToScheme(s)).To(Succeed())
		Expect(istionetworkingv1beta1.AddToScheme(s)).To(Succeed())
		Expect(istionetworkingv1alpha3.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()
		apiServerProxyValues = &APIServerProxy{
			APIServerClusterIP: "1.1.1.1",
		}

		expectedDestinationRule = &istionetworkingv1beta1.DestinationRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
				ResourceVersion: "1",
			},
			Spec: istioapinetworkingv1beta1.DestinationRule{
				ExportTo: []string{"*"},
				Host:     hostName,
				TrafficPolicy: &istioapinetworkingv1beta1.TrafficPolicy{
					ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
						Tcp: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
							MaxConnections: 5000,
							TcpKeepalive: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
								Time:     &durationpb.Duration{Seconds: 7200},
								Interval: &durationpb.Duration{Seconds: 75},
							},
						},
					},
					LoadBalancer: &istioapinetworkingv1beta1.LoadBalancerSettings{
						LocalityLbSetting: &istioapinetworkingv1beta1.LocalityLoadBalancerSetting{
							Enabled:          &wrapperspb.BoolValue{Value: true},
							FailoverPriority: []string{"topology.kubernetes.io/zone"},
						},
					},
					OutlierDetection: &istioapinetworkingv1beta1.OutlierDetection{
						MinHealthPercent: 0,
					},
					Tls: &istioapinetworkingv1beta1.ClientTLSSettings{
						Mode: istioapinetworkingv1beta1.ClientTLSSettings_DISABLE,
					},
				},
			},
		}
		expectedEnvoyFilterObjectMeta = metav1.ObjectMeta{
			Name:      namespace,
			Namespace: istioNamespace,
		}
		expectedGateway = &istionetworkingv1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
				ResourceVersion: "1",
			},
			Spec: istioapinetworkingv1beta1.Gateway{
				Selector: istioLabels,
				Servers: []*istioapinetworkingv1beta1.Server{{
					Hosts: hosts,
					Port: &istioapinetworkingv1beta1.Port{
						Number:   443,
						Name:     "tls",
						Protocol: "TLS",
					},
					Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
						Mode: istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH,
					},
				}},
			},
		}
		expectedVirtualService = &istionetworkingv1beta1.VirtualService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
				ResourceVersion: "1",
			},
			Spec: istioapinetworkingv1beta1.VirtualService{
				ExportTo: []string{"*"},
				Hosts:    hosts,
				Gateways: []string{expectedGateway.Name},
				Tls: []*istioapinetworkingv1beta1.TLSRoute{{
					Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
						Port:     443,
						SniHosts: hosts,
					}},
					Route: []*istioapinetworkingv1beta1.RouteDestination{{
						Destination: &istioapinetworkingv1beta1.Destination{
							Host: hostName,
							Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
						},
					}},
				}},
			},
		}
		expectedManagedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "kube-apiserver-sni",
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class:       ptr.To("seed"),
				KeepObjects: ptr.To(false),
			},
		}
	})

	JustBeforeEach(func() {
		defaultDepWaiter = NewSNI(c, v1beta1constants.DeploymentNameKubeAPIServer, namespace, func() *SNIValues {
			val := &SNIValues{
				Hosts:          hosts,
				APIServerProxy: apiServerProxyValues,
				IstioIngressGateway: IstioIngressGateway{
					Namespace: istioNamespace,
					Labels:    istioLabels,
				},
			}
			return val
		})
	})

	Describe("#Deploy", func() {
		test := func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actualDestinationRule := &istionetworkingv1beta1.DestinationRule{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, actualDestinationRule)).To(Succeed())
			Expect(actualDestinationRule).To(BeComparableTo(expectedDestinationRule, comptest.CmpOptsForDestinationRule()))

			actualGateway := &istionetworkingv1beta1.Gateway{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedGateway.Namespace, Name: expectedGateway.Name}, actualGateway)).To(Succeed())
			Expect(actualGateway).To(BeComparableTo(expectedGateway, comptest.CmpOptsForGateway()))

			actualVirtualService := &istionetworkingv1beta1.VirtualService{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVirtualService.Namespace, Name: expectedVirtualService.Name}, actualVirtualService)).To(Succeed())
			Expect(actualVirtualService).To(BeComparableTo(expectedVirtualService, comptest.CmpOptsForVirtualService()))

			if apiServerProxyValues != nil {
				managedResource := &resourcesv1alpha1.ManagedResource{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResource.Namespace, Name: expectedManagedResource.Name}, managedResource)).To(Succeed())
				expectedManagedResource.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}}
				utilruntime.Must(references.InjectAnnotations(expectedManagedResource))
				Expect(managedResource).To(DeepEqual(expectedManagedResource))

				managedResourceSecret := &corev1.Secret{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResource.Namespace, Name: expectedManagedResource.Spec.SecretRefs[0].Name}, managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
				Expect(managedResourceSecret.Data).To(HaveLen(1))
				Expect(managedResourceSecret.Data).To(HaveKey("data.yaml.br"))

				mrData, err := test.BrotliDecompression(managedResourceSecret.Data["data.yaml.br"])
				Expect(err).NotTo(HaveOccurred())

				managedResourceEnvoyFilter, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode(mrData, nil, &istionetworkingv1alpha3.EnvoyFilter{})
				Expect(err).ToNot(HaveOccurred())
				Expect(managedResourceEnvoyFilter.GetObjectKind()).To(Equal(&metav1.TypeMeta{Kind: "EnvoyFilter", APIVersion: "networking.istio.io/v1alpha3"}))
				actualEnvoyFilter := managedResourceEnvoyFilter.(*istionetworkingv1alpha3.EnvoyFilter)
				// cannot validate the Spec as there is no meaningful way to unmarshal the data into the Golang structure
				Expect(actualEnvoyFilter.ObjectMeta).To(DeepEqual(expectedEnvoyFilterObjectMeta))
			}
		}

		Context("when APIServer Proxy is configured", func() {
			It("should succeed deploying", func() {
				test()
			})
		})

		Context("when APIServer Proxy is not configured", func() {
			BeforeEach(func() {
				apiServerProxyValues = nil
			})

			It("should succeed deploying", func() {
				test()
			})
		})
	})

	It("should succeed destroying", func() {
		Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, &istionetworkingv1beta1.DestinationRule{})).To(Succeed())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedGateway.Namespace, Name: expectedGateway.Name}, &istionetworkingv1beta1.Gateway{})).To(Succeed())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVirtualService.Namespace, Name: expectedVirtualService.Name}, &istionetworkingv1beta1.VirtualService{})).To(Succeed())
		managedResource := &resourcesv1alpha1.ManagedResource{}
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResource.Namespace, Name: expectedManagedResource.Name}, managedResource)).To(Succeed())
		managedResourceSecretName := managedResource.Spec.SecretRefs[0].Name
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResource.Namespace, Name: managedResourceSecretName}, &corev1.Secret{})).To(Succeed())

		Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())

		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedGateway.Namespace, Name: expectedGateway.Name}, &istionetworkingv1beta1.Gateway{})).To(BeNotFoundError())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVirtualService.Namespace, Name: expectedVirtualService.Name}, &istionetworkingv1beta1.VirtualService{})).To(BeNotFoundError())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResource.Namespace, Name: expectedManagedResource.Name}, managedResource)).To(BeNotFoundError())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResource.Namespace, Name: managedResourceSecretName}, &corev1.Secret{})).To(BeNotFoundError())
	})

	Describe("#Wait", func() {
		It("should succeed because it's not implemented", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should succeed because it's not implemented", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
