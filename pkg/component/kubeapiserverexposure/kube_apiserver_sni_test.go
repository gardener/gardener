// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserverexposure_test

import (
	"context"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/test"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	comptest "github.com/gardener/gardener/pkg/component/test"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#SNI", func() {
	var (
		ctx     context.Context
		c       client.Client
		applier kubernetes.Applier

		defaultDepWaiter component.DeployWaiter
		namespace        = "test-namespace"
		namespaceUID     = types.UID("123456")
		istioLabels      = map[string]string{"foo": "bar"}
		istioNamespace   = "istio-foo"
		hosts            = []string{"foo.bar"}
		hostName         = "kube-apiserver." + namespace + ".svc.cluster.local"

		expectedDestinationRule       *istionetworkingv1beta1.DestinationRule
		expectedGateway               *istionetworkingv1beta1.Gateway
		expectedVirtualService        *istionetworkingv1beta1.VirtualService
		expectedEnvoyFilterObjectMeta metav1.ObjectMeta
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(istionetworkingv1beta1.AddToScheme(s)).To(Succeed())
		Expect(istionetworkingv1alpha3.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		var err error
		applier, err = test.NewTestApplier(c, &fakediscovery.FakeDiscovery{
			Fake:               &testing.Fake{},
			FakedServerVersion: &version.Info{GitVersion: "v1.21.0"},
		})
		Expect(err).NotTo(HaveOccurred())

		expectedDestinationRule = &istionetworkingv1beta1.DestinationRule{
			TypeMeta: metav1.TypeMeta{
				APIVersion: istionetworkingv1beta1.SchemeGroupVersion.String(),
				Kind:       "DestinationRule",
			},
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
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "v1",
				Kind:               "Namespace",
				Name:               namespace,
				UID:                namespaceUID,
				BlockOwnerDeletion: pointer.Bool(false),
				Controller:         pointer.Bool(false),
			}},
			ResourceVersion: "1",
		}
		expectedGateway = &istionetworkingv1beta1.Gateway{
			TypeMeta: metav1.TypeMeta{
				APIVersion: istionetworkingv1beta1.SchemeGroupVersion.String(),
				Kind:       "Gateway",
			},
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
			TypeMeta: metav1.TypeMeta{
				APIVersion: istionetworkingv1beta1.SchemeGroupVersion.String(),
				Kind:       "VirtualService",
			},
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
	})

	JustBeforeEach(func() {
		defaultDepWaiter = NewSNI(c, applier, namespace, func() *SNIValues {
			return &SNIValues{
				Hosts:              hosts,
				APIServerClusterIP: "1.1.1.1",
				IstioIngressGateway: IstioIngressGateway{
					Namespace: istioNamespace,
					Labels:    istioLabels,
				},
				NamespaceUID: namespaceUID,
			}
		})
	})

	Describe("#Deploy", func() {
		It("succeeds", func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actualDestinationRule := &istionetworkingv1beta1.DestinationRule{}
			Expect(c.Get(ctx, kubernetesutils.Key(expectedDestinationRule.Namespace, expectedDestinationRule.Name), actualDestinationRule)).To(Succeed())
			Expect(actualDestinationRule).To(BeComparableTo(expectedDestinationRule, comptest.CmpOptsForDestinationRule()))

			actualGateway := &istionetworkingv1beta1.Gateway{}
			Expect(c.Get(ctx, kubernetesutils.Key(expectedGateway.Namespace, expectedGateway.Name), actualGateway)).To(Succeed())
			Expect(actualGateway).To(BeComparableTo(expectedGateway, comptest.CmpOptsForGateway()))

			actualVirtualService := &istionetworkingv1beta1.VirtualService{}
			Expect(c.Get(ctx, kubernetesutils.Key(expectedVirtualService.Namespace, expectedVirtualService.Name), actualVirtualService)).To(Succeed())
			Expect(actualVirtualService).To(BeComparableTo(expectedVirtualService, comptest.CmpOptsForVirtualService()))

			actualEnvoyFilter := &istionetworkingv1alpha3.EnvoyFilter{}
			Expect(c.Get(ctx, kubernetesutils.Key(expectedEnvoyFilterObjectMeta.Namespace, expectedEnvoyFilterObjectMeta.Name), actualEnvoyFilter)).To(Succeed())
			// cannot validate the Spec as there is meaningful way to unmarshal the data into the Golang structure
			Expect(actualEnvoyFilter.ObjectMeta).To(DeepEqual(expectedEnvoyFilterObjectMeta))
		})
	})

	It("destroy succeeds", func() {
		Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

		Expect(c.Get(ctx, kubernetesutils.Key(expectedDestinationRule.Namespace, expectedDestinationRule.Name), &istionetworkingv1beta1.DestinationRule{})).To(Succeed())
		Expect(c.Get(ctx, kubernetesutils.Key(expectedGateway.Namespace, expectedGateway.Name), &istionetworkingv1beta1.Gateway{})).To(Succeed())
		Expect(c.Get(ctx, kubernetesutils.Key(expectedVirtualService.Namespace, expectedVirtualService.Name), &istionetworkingv1beta1.VirtualService{})).To(Succeed())
		Expect(c.Get(ctx, kubernetesutils.Key(expectedEnvoyFilterObjectMeta.Namespace, expectedEnvoyFilterObjectMeta.Name), &istionetworkingv1alpha3.EnvoyFilter{})).To(Succeed())

		Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())

		Expect(c.Get(ctx, kubernetesutils.Key(expectedDestinationRule.Namespace, expectedDestinationRule.Name), &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
		Expect(c.Get(ctx, kubernetesutils.Key(expectedGateway.Namespace, expectedGateway.Name), &istionetworkingv1beta1.Gateway{})).To(BeNotFoundError())
		Expect(c.Get(ctx, kubernetesutils.Key(expectedVirtualService.Namespace, expectedVirtualService.Name), &istionetworkingv1beta1.VirtualService{})).To(BeNotFoundError())
		Expect(c.Get(ctx, kubernetesutils.Key(expectedEnvoyFilterObjectMeta.Namespace, expectedEnvoyFilterObjectMeta.Name), &istionetworkingv1alpha3.EnvoyFilter{})).To(BeNotFoundError())
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

	Describe("#AnyDeployedSNI", func() {
		var (
			c        client.Client
			createVS = func(name string, namespace string) *unstructured.Unstructured {
				return &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "networking.istio.io/v1beta1",
						"kind":       "VirtualService",
						"metadata": map[string]interface{}{
							"name":      name,
							"namespace": namespace,
						},
					},
				}
			}
		)

		Context("CRD available", func() {
			BeforeEach(func() {
				s := runtime.NewScheme()
				s.AddKnownTypeWithName(schema.FromAPIVersionAndKind("networking.istio.io/v1beta1", "VirtualServiceList"), &unstructured.UnstructuredList{})
				s.AddKnownTypeWithName(schema.FromAPIVersionAndKind("networking.istio.io/v1beta1", "VirtualService"), &unstructured.Unstructured{})
				virtualServiceObj := &unstructured.Unstructured{}
				virtualServiceObj.SetGroupVersionKind(schema.FromAPIVersionAndKind("networking.istio.io/v1beta1", "VirtualService"))

				c = fake.NewClientBuilder().
					WithScheme(s).
					WithIndex(
						virtualServiceObj, "metadata.name", func(o client.Object) []string {
							return []string{o.GetName()}
						},
					).
					Build()
			})

			It("returns true when exists", func() {
				Expect(c.Create(ctx, createVS("kube-apiserver", "test"))).NotTo(HaveOccurred())
				any, err := AnyDeployedSNI(ctx, c)

				Expect(err).NotTo(HaveOccurred())
				Expect(any).To(BeTrue())
			})

			It("returns false when does not exists", func() {
				any, err := AnyDeployedSNI(ctx, c)

				Expect(err).NotTo(HaveOccurred())
				Expect(any).To(BeFalse())
			})
		})

		Context("CRD not available", func() {
			var (
				ctrl   *gomock.Controller
				client *mockclient.MockClient
			)

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				client = mockclient.NewMockClient(ctrl)
			})

			AfterEach(func() {
				ctrl.Finish()
			})

			It("returns false", func() {
				client.EXPECT().List(ctx, gomock.AssignableToTypeOf(&unstructured.UnstructuredList{}), gomock.Any()).Return(&meta.NoKindMatchError{})
				any, err := AnyDeployedSNI(ctx, client)

				Expect(err).NotTo(HaveOccurred())
				Expect(any).To(BeFalse())
			})
		})
	})
})
