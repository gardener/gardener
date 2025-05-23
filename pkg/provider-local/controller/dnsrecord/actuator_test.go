// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/provider-local/controller/dnsrecord"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
)

var _ = Describe("Actuator", func() {
	Describe("DNS Rewriting", func() {
		var (
			actuator dnsrecord.Actuator
			c        client.Client
			ctrl     *gomock.Controller
			mgr      *mockmanager.MockManager

			ctx     = context.TODO()
			cluster = &extensionscontroller.Cluster{
				Seed: &v1beta1.Seed{
					Spec: v1beta1.SeedSpec{
						Provider: v1beta1.SeedProvider{
							Zones: []string{"a", "b", "c"},
						},
					},
				},
			}
			namespace           = "foo"
			singleZoneNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Annotations: map[string]string{
						"high-availability-config.resources.gardener.cloud/zones": "a",
					},
				},
			}
			multiZoneNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Annotations: map[string]string{
						"high-availability-config.resources.gardener.cloud/zones": "a,b,c",
					},
				},
			}
			apiDNSRecord = &extensionsv1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
				},
				Spec: extensionsv1alpha1.DNSRecordSpec{
					Name: "api.something.local.gardener.cloud",
				},
			}
			otherDNSRecord = &extensionsv1alpha1.DNSRecord{
				Spec: extensionsv1alpha1.DNSRecordSpec{
					Name: "foo.bar",
				},
			}
			extensionNamespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener-extension-provider-local-coredns",
				},
			}
			emptyConfigMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "coredns-custom",
					Namespace: extensionNamespace.Name,
				},
				Data: map[string]string{"test": "data"},
			}
			configMapWithRule = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "coredns-custom",
					Namespace: extensionNamespace.Name,
				},
				Data: map[string]string{
					"test":                               "data",
					apiDNSRecord.Spec.Name + ".override": "some rule",
				},
			}
			log = logf.Log.WithName("test")
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mgr = mockmanager.NewMockManager(ctrl)
		})

		Describe("Successful reconciliation", func() {
			It("Should add single zone rewrite rule", func() {
				c = initializeClient(singleZoneNamespace, extensionNamespace, emptyConfigMap)
				mgr.EXPECT().GetClient().Return(c)
				actuator = NewActuator(mgr)
				Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).NotTo(HaveOccurred())
				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).NotTo(HaveOccurred())
				Expect(result.Data[apiDNSRecord.Spec.Name+".override"]).To(
					Equal("rewrite stop name regex api\\.something\\.local\\.gardener\\.cloud istio-ingressgateway.istio-ingress--a.svc.cluster.local answer auto"))
			})

			It("Should add multi zone rewrite rule", func() {
				c = initializeClient(multiZoneNamespace, extensionNamespace, emptyConfigMap)
				mgr.EXPECT().GetClient().Return(c)
				actuator = NewActuator(mgr)
				Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).NotTo(HaveOccurred())
				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).NotTo(HaveOccurred())
				Expect(result.Data[apiDNSRecord.Spec.Name+".override"]).To(
					Equal("rewrite stop name regex api\\.something\\.local\\.gardener\\.cloud istio-ingressgateway.istio-ingress.svc.cluster.local answer auto"))
			})

			It("Should ignore other dns entries", func() {
				c = initializeClient(singleZoneNamespace, extensionNamespace, emptyConfigMap)
				mgr.EXPECT().GetClient().Return(c)
				actuator = NewActuator(mgr)
				Expect(actuator.Reconcile(ctx, log, otherDNSRecord, cluster)).NotTo(HaveOccurred())
				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).NotTo(HaveOccurred())
				Expect(result.Data).To(Equal(map[string]string{"test": "data"}))
			})
		})

		Describe("Successful deletion", func() {
			It("Should remove single zone rewrite rule", func() {
				c = initializeClient(singleZoneNamespace, extensionNamespace, configMapWithRule)
				mgr.EXPECT().GetClient().Return(c)
				actuator = NewActuator(mgr)
				Expect(actuator.Delete(ctx, log, apiDNSRecord, cluster)).NotTo(HaveOccurred())
				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapWithRule), result)).NotTo(HaveOccurred())
				Expect(result.Data).To(Equal(map[string]string{"test": "data"}))
			})

			It("Should remove multi zone rewrite rule", func() {
				c = initializeClient(multiZoneNamespace, extensionNamespace, configMapWithRule)
				mgr.EXPECT().GetClient().Return(c)
				actuator = NewActuator(mgr)
				Expect(actuator.Delete(ctx, log, apiDNSRecord, cluster)).NotTo(HaveOccurred())
				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapWithRule), result)).NotTo(HaveOccurred())
				Expect(result.Data).To(Equal(map[string]string{"test": "data"}))
			})

			It("Should ignore other dns entries", func() {
				c = initializeClient(singleZoneNamespace, extensionNamespace, configMapWithRule)
				mgr.EXPECT().GetClient().Return(c)
				actuator = NewActuator(mgr)
				Expect(actuator.Delete(ctx, log, otherDNSRecord, cluster)).NotTo(HaveOccurred())
				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapWithRule), result)).NotTo(HaveOccurred())
				Expect(result.Data).To(Equal(map[string]string{"test": "data", apiDNSRecord.Spec.Name + ".override": "some rule"}))
			})
		})
	})
})

func initializeClient(objects ...client.Object) client.Client {
	client := fakeclient.NewClientBuilder().WithObjects(objects...).WithScheme(scheme.Scheme).Build()
	return client
}
