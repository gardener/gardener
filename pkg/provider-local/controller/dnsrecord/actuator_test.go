// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/networking/coredns"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/provider-local/controller/dnsrecord"
)

var _ = Describe("Actuator", func() {
	Describe("DNS Rewriting", func() {
		var (
			log        logr.Logger
			fakeClient client.Client

			actuator dnsrecord.Actuator

			namespace           string
			cluster             *extensionscontroller.Cluster
			singleZoneNamespace *corev1.Namespace
			multiZoneNamespace  *corev1.Namespace
			apiDNSRecord        *extensionsv1alpha1.DNSRecord
			otherDNSRecord      *extensionsv1alpha1.DNSRecord

			extensionNamespace *corev1.Namespace
			emptyConfigMap     *corev1.ConfigMap
			configMapWithRule  *corev1.ConfigMap
		)

		BeforeEach(func() {
			log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter))
			fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme.Scheme).Build()

			actuator = &Actuator{
				Client: fakeClient,
			}

			namespace = "foo"
			cluster = &extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{},
				Seed: &gardencorev1beta1.Seed{
					Spec: gardencorev1beta1.SeedSpec{
						Provider: gardencorev1beta1.SeedProvider{
							Zones: []string{"a", "b", "c"},
						},
					},
				},
			}
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
					Name:   "api.something.local.gardener.cloud",
					Values: []string{"1.2.3.4", "5.6.7.8"},
					TTL:    ptr.To[int64](123),
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
					Name:      coredns.CustomConfigMapName,
					Namespace: extensionNamespace.Name,
				},
				Data: map[string]string{"test": "data"},
			}
			configMapWithRule = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      coredns.CustomConfigMapName,
					Namespace: extensionNamespace.Name,
				},
				Data: map[string]string{
					"test":                               "data",
					apiDNSRecord.Spec.Name + ".override": "some rule",
				},
			}
		})

		Describe("Reconcile", func() {
			It("should add single zone rewrite rule", func(ctx SpecContext) {
				createObjects(fakeClient, singleZoneNamespace, extensionNamespace, emptyConfigMap)

				Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).To(Succeed())

				result := &corev1.ConfigMap{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).To(Succeed())
				Expect(result.Data[apiDNSRecord.Spec.Name+".override"]).To(
					Equal("rewrite stop name regex api\\.something\\.local\\.gardener\\.cloud istio-ingressgateway.istio-ingress--a.svc.cluster.local answer auto"))
			})

			It("should add multi zone rewrite rule", func(ctx SpecContext) {
				createObjects(fakeClient, multiZoneNamespace, extensionNamespace, emptyConfigMap)

				Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).To(Succeed())

				result := &corev1.ConfigMap{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).To(Succeed())
				Expect(result.Data[apiDNSRecord.Spec.Name+".override"]).To(
					Equal("rewrite stop name regex api\\.something\\.local\\.gardener\\.cloud istio-ingressgateway.istio-ingress.svc.cluster.local answer auto"))
			})

			It("should ignore other dns entries", func(ctx SpecContext) {
				createObjects(fakeClient, singleZoneNamespace, extensionNamespace, emptyConfigMap)

				Expect(actuator.Reconcile(ctx, log, otherDNSRecord, cluster)).To(Succeed())

				result := &corev1.ConfigMap{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).To(Succeed())
				Expect(result.Data).To(Equal(map[string]string{"test": "data"}))
			})
		})

		Describe("Delete", func() {
			It("should remove single zone rewrite rule", func(ctx SpecContext) {
				createObjects(fakeClient, singleZoneNamespace, extensionNamespace, configMapWithRule)

				Expect(actuator.Delete(ctx, log, apiDNSRecord, cluster)).To(Succeed())

				result := &corev1.ConfigMap{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapWithRule), result)).To(Succeed())
				Expect(result.Data).To(Equal(map[string]string{"test": "data"}))
			})

			It("should remove multi zone rewrite rule", func(ctx SpecContext) {
				createObjects(fakeClient, multiZoneNamespace, extensionNamespace, configMapWithRule)

				Expect(actuator.Delete(ctx, log, apiDNSRecord, cluster)).To(Succeed())

				result := &corev1.ConfigMap{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapWithRule), result)).To(Succeed())
				Expect(result.Data).To(Equal(map[string]string{"test": "data"}))
			})

			It("should ignore other dns entries", func(ctx SpecContext) {
				createObjects(fakeClient, singleZoneNamespace, extensionNamespace, configMapWithRule)

				Expect(actuator.Delete(ctx, log, otherDNSRecord, cluster)).To(Succeed())

				result := &corev1.ConfigMap{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapWithRule), result)).To(Succeed())
				Expect(result.Data).To(Equal(map[string]string{"test": "data", apiDNSRecord.Spec.Name + ".override": "some rule"}))
			})
		})

		Describe("Migrate", func() {
			It("should remove the rewrite rule", func(ctx SpecContext) {
				createObjects(fakeClient, extensionNamespace, configMapWithRule)

				Expect(actuator.Migrate(ctx, log, apiDNSRecord, cluster)).To(Succeed())

				result := &corev1.ConfigMap{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMapWithRule), result)).To(Succeed())
				Expect(result.Data).To(Equal(map[string]string{"test": "data"}))
			})
		})

		Context("self-hosted shoots", func() {
			BeforeEach(func() {
				cluster.Shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{
					ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
				}}
			})

			Describe("Reconcile", func() {
				It("should add config for A record", func(ctx SpecContext) {
					createObjects(fakeClient, extensionNamespace, emptyConfigMap)

					apiDNSRecord.Spec.RecordType = "A"
					Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).To(Succeed())

					result := &corev1.ConfigMap{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).To(Succeed())
					Expect(result.Data[apiDNSRecord.Spec.Name+".override"]).To(
						Equal(`template IN A local.gardener.cloud {
  match "^api\.something\.local\.gardener\.cloud\.$"
  answer "{{ .Name }} 123 IN A 1.2.3.4 5.6.7.8"
  fallthrough
}
`))
				})

				It("should add config for AAAA record", func(ctx SpecContext) {
					createObjects(fakeClient, extensionNamespace, emptyConfigMap)

					apiDNSRecord.Spec.RecordType = "AAAA"
					Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).To(Succeed())

					result := &corev1.ConfigMap{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).To(Succeed())
					Expect(result.Data[apiDNSRecord.Spec.Name+".override"]).To(
						Equal(`template IN AAAA local.gardener.cloud {
  match "^api\.something\.local\.gardener\.cloud\.$"
  answer "{{ .Name }} 123 IN AAAA 1.2.3.4 5.6.7.8"
  fallthrough
}
`))
				})

				It("should fail for CNAME record", func(ctx SpecContext) {
					createObjects(fakeClient, extensionNamespace, emptyConfigMap)

					apiDNSRecord.Spec.RecordType = "CNAME"
					apiDNSRecord.Spec.Values = []string{"some.other.name.gardener.cloud"}
					Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).To(MatchError(ContainSubstring(`unsupported record type "CNAME" for self-hosted shoot`)))
				})
			})

			Describe("Migrate", func() {
				It("should not delete rule from ConfigMap", func(ctx SpecContext) {
					createObjects(fakeClient, extensionNamespace, configMapWithRule)

					Expect(actuator.Migrate(ctx, log, apiDNSRecord, cluster)).To(Succeed())

					result := &corev1.ConfigMap{}
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).To(Succeed())
					Expect(result.Data[apiDNSRecord.Spec.Name+".override"]).To(
						Equal("some rule"))
				})
			})
		})
	})
})

func createObjects(c client.Client, objs ...client.Object) {
	GinkgoHelper()

	for _, obj := range objs {
		Expect(c.Create(context.Background(), obj)).To(Succeed())
	}
}
