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

package botanist

import (
	"context"
	"fmt"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	mockdnsrecord "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord/mock"
	"github.com/gardener/gardener/pkg/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("dns", func() {
	const (
		seedNS  = "test-ns"
		shootNS = "shoot-ns"
	)

	var (
		b                        *Botanist
		seedClient, gardenClient client.Client
		s                        *runtime.Scheme
		ctx                      context.Context

		dnsEntryTTL int64 = 1234
	)

	BeforeEach(func() {
		ctx = context.TODO()
		b = &Botanist{
			Operation: &operation.Operation{
				Config: &config.GardenletConfiguration{
					Controllers: &config.GardenletControllerConfiguration{
						Shoot: &config.ShootControllerConfiguration{
							DNSEntryTTLSeconds: &dnsEntryTTL,
						},
					},
				},
				Shoot: &shootpkg.Shoot{
					Components: &shootpkg.Components{
						Extensions: &shootpkg.Extensions{
							DNS: &shootpkg.DNS{},
						},
					},
					SeedNamespace: seedNS,
				},
				Garden: &garden.Garden{},
				Logger: logrus.NewEntry(logger.NewNopLogger()),
			},
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Namespace: shootNS},
		})

		s = runtime.NewScheme()
		Expect(dnsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(s)).NotTo(HaveOccurred())

		gardenClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		seedClient = fake.NewClientBuilder().WithScheme(s).Build()

		renderer := cr.NewWithServerVersion(&version.Info{})
		chartApplier := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(seedClient, meta.NewDefaultRESTMapper([]schema.GroupVersion{})))
		Expect(chartApplier).NotTo(BeNil(), "should return chart applier")

		b.K8sGardenClient = fakeclientset.NewClientSetBuilder().
			WithClient(gardenClient).
			Build()
		b.K8sSeedClient = fakeclientset.NewClientSetBuilder().
			WithClient(seedClient).
			WithChartApplier(chartApplier).
			Build()
	})

	Context("DefaultExternalDNSProvider", func() {
		It("should create when calling Deploy and dns is enabled", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: pointer.String("foo")}
			b.Shoot.ExternalClusterDomain = pointer.String("baz")
			b.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}

			Expect(b.DefaultExternalDNSProvider().Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSProvider{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: "external", Namespace: seedNS}, found)
			Expect(err).ToNot(HaveOccurred())

			expected := &dnsv1alpha1.DNSProvider{
				TypeMeta: metav1.TypeMeta{Kind: "DNSProvider", APIVersion: "dns.gardener.cloud/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "external",
					Namespace:       seedNS,
					ResourceVersion: "1",
					Annotations: map[string]string{
						"dns.gardener.cloud/realms": "test-ns,",
					},
				},
				Spec: dnsv1alpha1.DNSProviderSpec{
					Type: "valid-provider",
					SecretRef: &corev1.SecretReference{
						Name: "extensions-dns-external",
					},
					Domains: &dnsv1alpha1.DNSSelection{
						Include: []string{"baz"},
					},
				},
			}
			Expect(found).To(DeepDerivativeEqual(expected))
		})
		It("should delete when calling Deploy and dns is disabled", func() {
			b.Shoot.DisableDNS = true
			Expect(seedClient.Create(ctx, &dnsv1alpha1.DNSProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: seedNS},
			})).NotTo(HaveOccurred())

			Expect(b.DefaultExternalDNSProvider().Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSProvider{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: "external", Namespace: seedNS}, found)
			Expect(err).To(BeNotFoundError())
		})
	})

	Context("DefaultInternalDNSProvider", func() {
		It("should create when calling Deploy and dns is enabled", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.DisableDNS = false
			b.Shoot.InternalClusterDomain = "foo.com"
			b.Garden.InternalDomain = &garden.Domain{
				Provider:     "valid-provider",
				IncludeZones: []string{"zone-a"},
				ExcludeZones: []string{"zone-b"},
			}

			Expect(b.DefaultInternalDNSProvider().Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSProvider{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: "internal", Namespace: seedNS}, found)
			Expect(err).ToNot(HaveOccurred())

			expected := &dnsv1alpha1.DNSProvider{
				TypeMeta: metav1.TypeMeta{Kind: "DNSProvider", APIVersion: "dns.gardener.cloud/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "internal",
					Namespace:       seedNS,
					ResourceVersion: "1",
				},
				Spec: dnsv1alpha1.DNSProviderSpec{
					Type: "valid-provider",
					SecretRef: &corev1.SecretReference{
						Name: "extensions-dns-internal",
					},
					Domains: &dnsv1alpha1.DNSSelection{
						Include: []string{"foo.com"},
					},
					Zones: &dnsv1alpha1.DNSSelection{
						Include: []string{"zone-a"},
						Exclude: []string{"zone-b"},
					},
				},
			}
			Expect(found).To(DeepDerivativeEqual(expected))
			delete(found.Annotations, v1beta1constants.GardenerTimestamp)
			Expect(found.Annotations).To(BeEmpty())
		})
		It("should delete when calling Deploy and dns is disabled", func() {
			b.Shoot.DisableDNS = true
			Expect(seedClient.Create(ctx, &dnsv1alpha1.DNSProvider{
				ObjectMeta: metav1.ObjectMeta{Name: "internal", Namespace: seedNS},
			})).ToNot(HaveOccurred())

			Expect(b.DefaultInternalDNSProvider().Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSProvider{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: "internal", Namespace: seedNS}, found)
			Expect(err).To(BeNotFoundError())
		})
	})

	Context("DefaultExternalDNSEntry", func() {
		It("should delete the entry when calling Deploy", func() {
			Expect(seedClient.Create(ctx, &dnsv1alpha1.DNSEntry{
				ObjectMeta: metav1.ObjectMeta{Name: "external", Namespace: seedNS},
			})).ToNot(HaveOccurred())

			Expect(b.DefaultExternalDNSEntry().Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSEntry{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: "external", Namespace: seedNS}, found)
			Expect(err).To(BeNotFoundError())
		})
	})

	Context("DefaultExternalDNSOwner", func() {
		It("should delete the owner when calling Deploy", func() {
			Expect(seedClient.Create(ctx, &dnsv1alpha1.DNSOwner{
				ObjectMeta: metav1.ObjectMeta{Name: seedNS + "-external"},
			})).ToNot(HaveOccurred())

			Expect(b.DefaultExternalDNSOwner().Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSOwner{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: seedNS + "-external"}, found)
			Expect(err).To(BeNotFoundError())
		})
	})

	Context("DefaultInternalDNSEntry", func() {
		It("should delete when calling Deploy", func() {
			Expect(seedClient.Create(ctx, &dnsv1alpha1.DNSEntry{
				ObjectMeta: metav1.ObjectMeta{Name: "internal", Namespace: seedNS},
			})).ToNot(HaveOccurred())

			Expect(b.DefaultInternalDNSEntry().Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSEntry{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: "internal", Namespace: seedNS}, found)
			Expect(err).To(BeNotFoundError())
		})
	})

	Context("DefaultInternalDNSOwner", func() {
		It("should delete the owner when calling Deploy", func() {
			Expect(seedClient.Create(ctx, &dnsv1alpha1.DNSOwner{
				ObjectMeta: metav1.ObjectMeta{Name: seedNS + "-internal"},
			})).ToNot(HaveOccurred())

			Expect(b.DefaultInternalDNSOwner().Deploy(ctx)).ToNot(HaveOccurred())

			found := &dnsv1alpha1.DNSOwner{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: seedNS + "-internal"}, found)
			Expect(err).To(BeNotFoundError())
		})
	})

	Context("AdditionalDNSProviders", func() {
		It("should remove unneeded providers", func() {
			b.Shoot.DisableDNS = true

			providerOne := &dnsv1alpha1.DNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "to-remove",
					Namespace: seedNS,
					Labels: map[string]string{
						"gardener.cloud/role": "managed-dns-provider",
					},
					Annotations: map[string]string{
						"dns.gardener.cloud/realms": "test-ns,",
					},
				},
			}

			providerTwo := providerOne.DeepCopy()
			providerTwo.Name = "to-also-remove"

			Expect(seedClient.Create(ctx, providerOne)).ToNot(HaveOccurred())
			Expect(seedClient.Create(ctx, providerTwo)).ToNot(HaveOccurred())

			ap, err := b.AdditionalDNSProviders(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(ap).To(HaveLen(2))
			Expect(ap).To(HaveKey("to-remove"))
			Expect(ap).To(HaveKey("to-also-remove"))

			Expect(ap["to-remove"].Deploy(ctx)).NotTo(HaveOccurred(), "deploy (destroy) succeeds")
			Expect(ap["to-also-remove"].Deploy(ctx)).NotTo(HaveOccurred(), "deploy (destroy) succeeds")

			leftProviders := &dnsv1alpha1.DNSProviderList{}
			Expect(seedClient.List(ctx, leftProviders)).ToNot(HaveOccurred(), "listing of leftover providers succeeds")

			Expect(leftProviders.Items).To(BeEmpty())
		})

		It("should return error when provider is without Type", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
				Providers: []gardencorev1beta1.DNSProvider{{}},
			}

			ap, err := b.AdditionalDNSProviders(ctx)
			Expect(err).To(HaveOccurred())
			Expect(ap).To(HaveLen(0))
		})

		It("should return error when provider is without secretName", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
				Providers: []gardencorev1beta1.DNSProvider{{
					Type: pointer.String("foo"),
				}},
			}

			ap, err := b.AdditionalDNSProviders(ctx)
			Expect(err).To(HaveOccurred())
			Expect(ap).To(HaveLen(0))
		})

		It("should return error when provider is without secret", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
				Providers: []gardencorev1beta1.DNSProvider{{
					Type:       pointer.String("foo"),
					SecretName: pointer.String("not-existing-secret"),
				}},
			}

			ap, err := b.AdditionalDNSProviders(ctx)
			Expect(err).To(HaveOccurred())
			Expect(ap).To(HaveLen(0))
		})

		It("should add providers", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
				Providers: []gardencorev1beta1.DNSProvider{
					{
						Type:    pointer.String("primary-skip"),
						Primary: pointer.Bool(true),
					},
					{
						Type: pointer.String("unmanaged"),
					},
					{
						Type:       pointer.String("provider-one"),
						SecretName: pointer.String("secret-one"),
						Domains: &gardencorev1beta1.DNSIncludeExclude{
							Include: []string{"domain-1-include"},
							Exclude: []string{"domain-2-exclude"},
						},
						Zones: &gardencorev1beta1.DNSIncludeExclude{
							Include: []string{"zone-1-include"},
							Exclude: []string{"zone-1-exclude"},
						},
					},
					{
						Type:       pointer.String("provider-two"),
						SecretName: pointer.String("secret-two"),
					},
				},
			}

			Expect(seedClient.Create(ctx, &dnsv1alpha1.DNSProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "to-remove",
					Namespace: seedNS,
					Labels: map[string]string{
						"gardener.cloud/role": "managed-dns-provider",
					},
				},
			})).ToNot(HaveOccurred())

			secretOne := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-one",
					Namespace: shootNS,
				},
			}
			secretTwo := secretOne.DeepCopy()
			secretTwo.Name = "secret-two"

			Expect(gardenClient.Create(ctx, secretOne)).NotTo(HaveOccurred())
			Expect(gardenClient.Create(ctx, secretTwo)).NotTo(HaveOccurred())

			ap, err := b.AdditionalDNSProviders(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(ap).To(HaveLen(3))
			Expect(ap).To(HaveKey("to-remove"))
			Expect(ap).To(HaveKey("provider-one-secret-one"))
			Expect(ap).To(HaveKey("provider-two-secret-two"))

			for k, p := range ap {
				Expect(p.Deploy(ctx)).NotTo(HaveOccurred(), fmt.Sprintf("deploy of %s succeeds", k))
			}

			// can't use list - item[0]: can't assign or convert unstructured.Unstructured into v1alpha1.DNSProvider
			Expect(seedClient.Get(
				ctx,
				types.NamespacedName{Namespace: seedNS, Name: "to-remove"},
				&dnsv1alpha1.DNSProvider{},
			)).To(BeNotFoundError())

			providerOne := &dnsv1alpha1.DNSProvider{}
			Expect(seedClient.Get(
				ctx,
				types.NamespacedName{Namespace: seedNS, Name: "provider-one-secret-one"},
				providerOne,
			)).ToNot(HaveOccurred())

			Expect(providerOne.Spec.Domains).To(Equal(&dnsv1alpha1.DNSSelection{
				Include: []string{"domain-1-include"},
				Exclude: []string{"domain-2-exclude"},
			}))
			Expect(providerOne.Spec.Zones).To(Equal(&dnsv1alpha1.DNSSelection{
				Include: []string{"zone-1-include"},
				Exclude: []string{"zone-1-exclude"},
			}))

			Expect(seedClient.Get(
				ctx,
				types.NamespacedName{Namespace: seedNS, Name: "provider-two-secret-two"},
				&dnsv1alpha1.DNSProvider{},
			)).ToNot(HaveOccurred())
		})
	})

	Context("NeedsExternalDNS", func() {
		It("should be false when dns disabled", func() {
			b.Shoot.DisableDNS = true
			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be false when Shoot's DNS is nil", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = nil
			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be false when Shoot DNS's domain is nil", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: nil}
			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be false when Shoot ExternalClusterDomain is nil", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: pointer.String("foo")}
			b.Shoot.ExternalClusterDomain = nil
			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be false when Shoot ExternalClusterDomain is in nip.io", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: pointer.String("foo")}
			b.Shoot.ExternalClusterDomain = pointer.String("foo.nip.io")
			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be false when Shoot ExternalDomain is nil", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: pointer.String("foo")}
			b.Shoot.ExternalClusterDomain = pointer.String("baz")
			b.Shoot.ExternalDomain = nil

			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be false when Shoot ExternalDomain provider is unamanaged", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: pointer.String("foo")}
			b.Shoot.ExternalClusterDomain = pointer.String("baz")
			b.Shoot.ExternalDomain = &garden.Domain{Provider: "unmanaged"}

			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be true when Shoot ExternalDomain provider is valid", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: pointer.String("foo")}
			b.Shoot.ExternalClusterDomain = pointer.String("baz")
			b.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}

			Expect(b.NeedsExternalDNS()).To(BeTrue())
		})
	})

	Context("NeedsInternalDNS", func() {
		It("should be false when dns disabled", func() {
			b.Shoot.DisableDNS = true
			Expect(b.NeedsInternalDNS()).To(BeFalse())
		})

		It("should be false when the internal domain is nil", func() {
			b.Shoot.DisableDNS = false
			b.Garden.InternalDomain = nil
			Expect(b.NeedsInternalDNS()).To(BeFalse())
		})

		It("should be false when the internal domain provider is unmanaged", func() {
			b.Shoot.DisableDNS = false
			b.Garden.InternalDomain = &garden.Domain{Provider: "unmanaged"}
			Expect(b.NeedsInternalDNS()).To(BeFalse())
		})

		It("should be true when the internal domain provider is not unmanaged", func() {
			b.Shoot.DisableDNS = false
			b.Garden.InternalDomain = &garden.Domain{Provider: "some-provider"}
			Expect(b.NeedsInternalDNS()).To(BeTrue())
		})
	})

	Context("NeedsAdditionalDNSProviders", func() {
		It("should be false when dns disabled", func() {
			b.Shoot.DisableDNS = true
			Expect(b.NeedsAdditionalDNSProviders()).To(BeFalse())
		})

		It("should be false when Shoot's DNS is nil", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = nil
			Expect(b.NeedsAdditionalDNSProviders()).To(BeFalse())
		})

		It("should be false when there are no Shoot DNS Providers", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{}
			Expect(b.NeedsAdditionalDNSProviders()).To(BeFalse())
		})

		It("should be true when there are Shoot DNS Providers", func() {
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
				Providers: []gardencorev1beta1.DNSProvider{
					{Type: pointer.String("foo")},
					{Type: pointer.String("bar")},
				},
			}
			Expect(b.NeedsAdditionalDNSProviders()).To(BeTrue())
		})
	})

	Context("APIServerSNIEnabled", func() {
		BeforeEach(func() {
			gardenletfeatures.RegisterFeatureGates()
		})

		It("returns false when feature gate is disabled", func() {
			Expect(gardenletfeatures.FeatureGate.Set("APIServerSNI=false")).ToNot(HaveOccurred())

			Expect(b.APIServerSNIEnabled()).To(BeFalse())
		})

		It("returns true when feature gate is enabled", func() {
			Expect(gardenletfeatures.FeatureGate.Set("APIServerSNI=true")).ToNot(HaveOccurred())
			b.Garden.InternalDomain = &garden.Domain{Provider: "some-provider"}
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: pointer.String("foo")}
			b.Shoot.ExternalClusterDomain = pointer.String("baz")
			b.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}

			Expect(b.APIServerSNIEnabled()).To(BeTrue())
		})
	})

	Context("APIServerSNIPodMutatorEnabled", func() {
		BeforeEach(func() {
			gardenletfeatures.RegisterFeatureGates()
		})

		It("returns false when the feature gate is disabled", func() {
			Expect(gardenletfeatures.FeatureGate.Set("APIServerSNI=false")).ToNot(HaveOccurred())

			Expect(b.APIServerSNIPodMutatorEnabled()).To(BeFalse())
		})

		Context("APIServerSNI feature gate is enabled", func() {
			BeforeEach(func() {
				Expect(gardenletfeatures.FeatureGate.Set("APIServerSNI=true")).ToNot(HaveOccurred())
				b.Garden.InternalDomain = &garden.Domain{Provider: "some-provider"}
				b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: pointer.String("foo")}
				b.Shoot.ExternalClusterDomain = pointer.String("baz")
				b.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}
			})

			It("returns true when Shoot annotations are nil", func() {
				b.Shoot.GetInfo().Annotations = nil

				Expect(b.APIServerSNIPodMutatorEnabled()).To(BeTrue())
			})

			It("returns true when Shoot annotations does not have the annotation", func() {
				b.Shoot.GetInfo().Annotations = map[string]string{"foo": "bar"}

				Expect(b.APIServerSNIPodMutatorEnabled()).To(BeTrue())
			})

			It("returns true when Shoot annotations exist, but it's not a 'disable", func() {
				b.Shoot.GetInfo().Annotations = map[string]string{
					"alpha.featuregates.shoot.gardener.cloud/apiserver-sni-pod-injector": "not-disable",
				}

				Expect(b.APIServerSNIPodMutatorEnabled()).To(BeTrue())
			})

			It("returns false when Shoot annotations exist and it's a disable", func() {
				b.Shoot.GetInfo().Annotations = map[string]string{
					"alpha.featuregates.shoot.gardener.cloud/apiserver-sni-pod-injector": "disable",
				}

				Expect(b.APIServerSNIPodMutatorEnabled()).To(BeFalse())
			})
		})
	})

	Context("newDNSComponentsTargetingAPIServerAddress", func() {
		var (
			ctrl              *gomock.Controller
			externalDNSRecord *mockdnsrecord.MockInterface
			internalDNSRecord *mockdnsrecord.MockInterface
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			externalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
			internalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

			b.APIServerAddress = "1.2.3.4"
			b.Shoot.Components.Extensions.ExternalDNSRecord = externalDNSRecord
			b.Shoot.Components.Extensions.InternalDNSRecord = internalDNSRecord
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("does nothing when DNS is disabled", func() {
			b.Shoot.DisableDNS = true

			b.newDNSComponentsTargetingAPIServerAddress()

			Expect(b.Shoot.Components.Extensions.DNS.InternalOwner).To(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.InternalEntry).To(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalOwner).To(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalEntry).To(BeNil())
		})

		It("sets owners and entries which create DNSOwner and DNSEntry", func() {
			b.Shoot.GetInfo().Status.ClusterIdentity = pointer.String("shoot-cluster-identity")
			b.Shoot.DisableDNS = false
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: pointer.String("foo")}
			b.Shoot.InternalClusterDomain = "bar"
			b.Shoot.ExternalClusterDomain = pointer.String("baz")
			b.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}
			b.Garden.InternalDomain = &garden.Domain{Provider: "valid-provider"}

			externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			externalDNSRecord.EXPECT().SetValues([]string{"1.2.3.4"})
			internalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			internalDNSRecord.EXPECT().SetValues([]string{"1.2.3.4"})

			b.newDNSComponentsTargetingAPIServerAddress()

			Expect(b.Shoot.Components.Extensions.DNS.InternalOwner).ToNot(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.InternalOwner.Deploy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.InternalEntry).ToNot(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.InternalEntry.Deploy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalOwner).ToNot(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalOwner.Deploy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalEntry).ToNot(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalEntry.Deploy(ctx)).ToNot(HaveOccurred())

			internalOwner := &dnsv1alpha1.DNSOwner{}
			Expect(seedClient.Get(ctx, types.NamespacedName{Name: seedNS + "-internal"}, internalOwner)).ToNot(HaveOccurred())

			internalEntry := &dnsv1alpha1.DNSEntry{}
			Expect(seedClient.Get(ctx, types.NamespacedName{Name: "internal", Namespace: seedNS}, internalEntry)).ToNot(HaveOccurred())

			externalOwner := &dnsv1alpha1.DNSOwner{}
			Expect(seedClient.Get(ctx, types.NamespacedName{Name: seedNS + "-external"}, externalOwner)).ToNot(HaveOccurred())

			externalEntry := &dnsv1alpha1.DNSEntry{}
			Expect(seedClient.Get(ctx, types.NamespacedName{Name: "external", Namespace: seedNS}, externalEntry)).ToNot(HaveOccurred())

			Expect(internalOwner).To(DeepDerivativeEqual(&dnsv1alpha1.DNSOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-ns-internal",
					ResourceVersion: "1",
				},
				Spec: dnsv1alpha1.DNSOwnerSpec{
					OwnerId: "shoot-cluster-identity-internal",
					Active:  pointer.Bool(true),
				},
			}))
			Expect(internalEntry).To(DeepDerivativeEqual(&dnsv1alpha1.DNSEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "internal",
					Namespace:       "test-ns",
					ResourceVersion: "1",
				},
				Spec: dnsv1alpha1.DNSEntrySpec{
					DNSName: "api.bar",
					TTL:     &dnsEntryTTL,
					Targets: []string{"1.2.3.4"},
				},
			}))
			Expect(externalOwner).To(DeepDerivativeEqual(&dnsv1alpha1.DNSOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-ns-external",
					ResourceVersion: "1",
				},
				Spec: dnsv1alpha1.DNSOwnerSpec{
					OwnerId: "shoot-cluster-identity-external",
					Active:  pointer.Bool(true),
				},
			}))
			Expect(externalEntry).To(DeepDerivativeEqual(&dnsv1alpha1.DNSEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "external",
					Namespace:       "test-ns",
					ResourceVersion: "1",
				},
				Spec: dnsv1alpha1.DNSEntrySpec{
					DNSName: "api.baz",
					TTL:     &dnsEntryTTL,
					Targets: []string{"1.2.3.4"},
				},
			}))

			Expect(b.Shoot.Components.Extensions.DNS.InternalOwner.Destroy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.InternalEntry.Destroy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalOwner.Destroy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalEntry.Destroy(ctx)).ToNot(HaveOccurred())

			internalOwner = &dnsv1alpha1.DNSOwner{}
			Expect(seedClient.Get(ctx, types.NamespacedName{Name: seedNS + "-internal"}, internalOwner)).To(BeNotFoundError())

			internalEntry = &dnsv1alpha1.DNSEntry{}
			Expect(seedClient.Get(ctx, types.NamespacedName{Name: "internal", Namespace: seedNS}, internalEntry)).To(BeNotFoundError())

			externalOwner = &dnsv1alpha1.DNSOwner{}
			Expect(seedClient.Get(ctx, types.NamespacedName{Name: seedNS + "-external"}, externalOwner)).To(BeNotFoundError())

			externalEntry = &dnsv1alpha1.DNSEntry{}
			Expect(seedClient.Get(ctx, types.NamespacedName{Name: "external", Namespace: seedNS}, externalEntry)).To(BeNotFoundError())
		})
	})
})
