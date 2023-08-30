// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dnsrecord_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
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
	mockmanager "github.com/gardener/gardener/pkg/mock/controller-runtime/manager"
	. "github.com/gardener/gardener/pkg/provider-local/controller/dnsrecord"
)

func init() {
	format.CharactersAroundMismatchToInclude = 500
}

var _ = Describe("Actuator", func() {
	var (
		hostname  = "foo.bar.com"
		value1    = "1.2.3.4"
		value2    = "5.6.7.8"
		dnsRecord = &extensionsv1alpha1.DNSRecord{
			Spec: extensionsv1alpha1.DNSRecordSpec{
				Name:   hostname,
				Values: []string{value1, value2},
			},
		}

		etcdHostsContentWithoutSection = []byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`)

		etcdHostsContentWithEmptySection = []byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`)

		etcdHostsContentWithUpToDateSection = []byte(fmt.Sprintf(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
%s %s
%s %s
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`, value1, hostname, value2, hostname))

		etcdHostsContentWithUpToDateSectionAtFileEnd = []byte(fmt.Sprintf(`%s# Begin of gardener-extension-provider-local section
%s %s
%s %s
# End of gardener-extension-provider-local section
`, etcdHostsContentWithoutSection, value1, hostname, value2, hostname))

		etcdHostsContentWithUpToDateSectionAndAdditionalValues = []byte(fmt.Sprintf(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
%s %s
%s %s
bar baz
baz foo
foo bar
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`, value1, hostname, value2, hostname))
	)

	Describe("#CreateOrUpdateValuesInEtcHostsFile", func() {
		Context("section does not exist", func() {
			It("should add the provided values", func() {
				Expect(CreateOrUpdateValuesInEtcHostsFile(etcdHostsContentWithoutSection, dnsRecord)).To(Equal(etcdHostsContentWithUpToDateSectionAtFileEnd))
			})
		})

		Context("section exists but empty", func() {
			It("should add the provided values", func() {
				Expect(CreateOrUpdateValuesInEtcHostsFile(etcdHostsContentWithEmptySection, dnsRecord)).To(Equal(etcdHostsContentWithUpToDateSection))
			})
		})

		Context("section exists with different hostnames", func() {
			etcdHostsContent := []byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
foo bar
baz foo
bar baz
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`)

			It("should add the provided values", func() {
				Expect(CreateOrUpdateValuesInEtcHostsFile(etcdHostsContent, dnsRecord)).To(Equal(etcdHostsContentWithUpToDateSectionAndAdditionalValues))
			})
		})

		Context("section exists with same hostnames", func() {
			etcdHostsContent := []byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
foo bar
baz foo
bar baz
oldvalue ` + hostname + `
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`)

			It("should add the provided values", func() {
				Expect(CreateOrUpdateValuesInEtcHostsFile(etcdHostsContent, dnsRecord)).To(Equal(etcdHostsContentWithUpToDateSectionAndAdditionalValues))
			})
		})
	})

	Describe("#DeleteValuesInEtcHostsFile", func() {
		Context("section does not exist", func() {
			It("should do nothing", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContentWithoutSection, dnsRecord)).To(Equal(etcdHostsContentWithoutSection))
			})
		})

		Context("section exists but empty", func() {
			It("should drop the section", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContentWithEmptySection, dnsRecord)).To(Equal(etcdHostsContentWithoutSection))
			})
		})

		Context("section exists with different hostnames", func() {
			etcdHostsContent := []byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
# Begin of gardener-extension-provider-local section
bar baz
baz foo
foo bar
# End of gardener-extension-provider-local section
`)

			It("should do nothing", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContent, dnsRecord)).To(Equal(etcdHostsContent))
			})
		})

		Context("section exists with same hostnames", func() {
			etcdHostsContent := []byte(fmt.Sprintf(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
%s %s
oldvalue %s
bar baz
baz foo
foo bar
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`, value1, hostname, hostname))

			It("should delete the provided values", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContent, dnsRecord)).To(Equal([]byte(`##
# Host Database
#
# localhost is used to configure the loopback interface
# when the system is booting.  Do not change this entry.
##
127.0.0.1	localhost
255.255.255.255	broadcasthost
::1             localhost
# Begin of gardener-extension-provider-local section
bar baz
baz foo
foo bar
# End of gardener-extension-provider-local section
# Added by Docker Desktop
# To allow the same kube context to work on the host and the container:
127.0.0.1 kubernetes.docker.internal
# End of section
`)))
			})
		})

		Context("section exists with only hostnames and ips", func() {
			It("should delete the provided values", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContentWithUpToDateSection, dnsRecord)).To(Equal(etcdHostsContentWithoutSection))
			})
		})

		Context("section exists with only hostnames and ips at the end of the file", func() {
			It("should delete the provided values", func() {
				Expect(DeleteValuesInEtcHostsFile(etcdHostsContentWithUpToDateSectionAtFileEnd, dnsRecord)).To(Equal(etcdHostsContentWithoutSection))
			})
		})
	})

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
				actuator = NewActuator(mgr, false)
				Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).NotTo(HaveOccurred())
				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).NotTo(HaveOccurred())
				Expect(result.Data[apiDNSRecord.Spec.Name+".override"]).To(
					Equal("rewrite stop name regex api\\.something\\.local\\.gardener\\.cloud istio-ingressgateway.istio-ingress--a.svc.cluster.local answer auto"))
			})

			It("Should add multi zone rewrite rule", func() {
				c = initializeClient(multiZoneNamespace, extensionNamespace, emptyConfigMap)
				mgr.EXPECT().GetClient().Return(c)
				actuator = NewActuator(mgr, false)
				Expect(actuator.Reconcile(ctx, log, apiDNSRecord, cluster)).NotTo(HaveOccurred())
				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(emptyConfigMap), result)).NotTo(HaveOccurred())
				Expect(result.Data[apiDNSRecord.Spec.Name+".override"]).To(
					Equal("rewrite stop name regex api\\.something\\.local\\.gardener\\.cloud istio-ingressgateway.istio-ingress.svc.cluster.local answer auto"))
			})

			It("Should ignore other dns entries", func() {
				c = initializeClient(singleZoneNamespace, extensionNamespace, emptyConfigMap)
				mgr.EXPECT().GetClient().Return(c)
				actuator = NewActuator(mgr, false)
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
				actuator = NewActuator(mgr, false)
				Expect(actuator.Delete(ctx, log, apiDNSRecord, cluster)).NotTo(HaveOccurred())
				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapWithRule), result)).NotTo(HaveOccurred())
				Expect(result.Data).To(Equal(map[string]string{"test": "data"}))
			})

			It("Should remove multi zone rewrite rule", func() {
				c = initializeClient(multiZoneNamespace, extensionNamespace, configMapWithRule)
				mgr.EXPECT().GetClient().Return(c)
				actuator = NewActuator(mgr, false)
				Expect(actuator.Delete(ctx, log, apiDNSRecord, cluster)).NotTo(HaveOccurred())
				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(configMapWithRule), result)).NotTo(HaveOccurred())
				Expect(result.Data).To(Equal(map[string]string{"test": "data"}))
			})

			It("Should ignore other dns entries", func() {
				c = initializeClient(singleZoneNamespace, extensionNamespace, configMapWithRule)
				mgr.EXPECT().GetClient().Return(c)
				actuator = NewActuator(mgr, false)
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
