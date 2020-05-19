// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/test"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/helm/pkg/engine"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/test/gomega"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("dns", func() {
	const (
		seedNS  = "test-ns"
		shootNS = "shoot-ns"
	)

	var (
		b          *Botanist
		seedClient client.Client
		s          *runtime.Scheme
		ctx        context.Context
	)

	BeforeEach(func() {
		ctx = context.TODO()
		b = &Botanist{
			Operation: &operation.Operation{
				Shoot: &shoot.Shoot{
					Info: &v1beta1.Shoot{
						ObjectMeta: metav1.ObjectMeta{Namespace: shootNS},
						Spec: v1beta1.ShootSpec{
							Addons: &v1beta1.Addons{},
						},
					},
					SeedNamespace: seedNS,
					Components: &shoot.Components{
						DNS: &shoot.DNS{},
					},
				},
				Garden:         &garden.Garden{},
				Logger:         logrus.NewEntry(logrus.New()),
				ChartsRootPath: "../../../charts",
			},
		}

		s = runtime.NewScheme()
		Expect(dnsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(s)).NotTo(HaveOccurred())

		seedClient = fake.NewFakeClientWithScheme(s)
		d := &test.FakeDiscovery{}
		cap, err := cr.DiscoverCapabilities(d)
		Expect(err).ToNot(HaveOccurred())

		renderer := cr.New(engine.New(), cap)
		a, err := test.NewTestApplier(seedClient, d)
		Expect(err).ToNot(HaveOccurred())

		b.ChartApplierSeed = kubernetes.NewChartApplier(renderer, a)
		Expect(b.ChartApplierSeed).NotTo(BeNil(), "should return chart applier")

	})

	Context("DefaultNginxIngressDNSEntry", func() {
		It("should delete when calling Deploy", func() {
			Expect(seedClient.Create(ctx, &dnsv1alpha1.DNSEntry{
				ObjectMeta: metav1.ObjectMeta{Name: "ingress", Namespace: seedNS},
			})).ToNot(HaveOccurred())

			Expect(b.DefaultNginxIngressDNSEntry(seedClient).Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSEntry{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: "ingress", Namespace: seedNS}, found)
			Expect(err).To(BeNotFoundError())
		})
	})

	Context("SetNginxIngressAddress", func() {
		It("does nothing when DNS is disabled", func() {
			b.Shoot.DisableDNS = true

			b.SetNginxIngressAddress("1.2.3.4", seedClient)

			Expect(b.Shoot.Components.DNS.NginxEntry).To(BeNil())
		})

		It("does nothing when hibernated", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.Info.Spec.DNS = &v1beta1.DNS{Domain: pointer.StringPtr("foo")}
			b.Shoot.ExternalClusterDomain = pointer.StringPtr("baz")
			b.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}
			b.Shoot.HibernationEnabled = true

			b.SetNginxIngressAddress("1.2.3.4", seedClient)

			Expect(b.Shoot.Components.DNS.NginxEntry).To(BeNil())
		})

		It("does nothing when nginx is disabled", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.Info.Spec.DNS = &v1beta1.DNS{Domain: pointer.StringPtr("foo")}
			b.Shoot.ExternalClusterDomain = pointer.StringPtr("baz")
			b.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}
			b.Shoot.HibernationEnabled = false
			b.Shoot.Info.Spec.Addons.NginxIngress = &v1beta1.NginxIngress{Addon: v1beta1.Addon{Enabled: false}}

			b.SetNginxIngressAddress("1.2.3.4", seedClient)

			Expect(b.Shoot.Components.DNS.NginxEntry).To(BeNil())
		})

		It("sets an entry which creates DNSEntry", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.Info.Spec.DNS = &v1beta1.DNS{Domain: pointer.StringPtr("foo")}
			b.Shoot.ExternalClusterDomain = pointer.StringPtr("baz")
			b.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}
			b.Shoot.HibernationEnabled = false
			b.Shoot.Info.Spec.Addons.NginxIngress = &v1beta1.NginxIngress{Addon: v1beta1.Addon{Enabled: true}}

			b.SetNginxIngressAddress("1.2.3.4", seedClient)

			Expect(b.Shoot.Components.DNS.NginxEntry).ToNot(BeNil())
			Expect(b.Shoot.Components.DNS.NginxEntry.Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSEntry{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: "ingress", Namespace: seedNS}, found)
			Expect(err).ToNot(HaveOccurred())

			Expect(found).To(DeepDerivativeEqual(&dnsv1alpha1.DNSEntry{
				TypeMeta: metav1.TypeMeta{
					Kind:       "DNSEntry",
					APIVersion: "dns.gardener.cloud/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "ingress",
					Namespace:       "test-ns",
					ResourceVersion: "1",
				},
				Spec: dnsv1alpha1.DNSEntrySpec{
					DNSName: "*.ingress.foo",
					TTL:     pointer.Int64Ptr(120),
					Targets: []string{"1.2.3.4"},
				},
			}))
		})
	})

})
