// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	cr "github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("controlplane", func() {
	const (
		seedNS  = "test-ns"
		shootNS = "shoot-ns"
	)

	var (
		b          *Botanist
		seedClient client.Client
		s          *runtime.Scheme
		ctx        context.Context

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
				Shoot: &shoot.Shoot{
					Info: &v1beta1.Shoot{
						ObjectMeta: metav1.ObjectMeta{Namespace: shootNS},
					},
					SeedNamespace: seedNS,
					Components: &shoot.Components{
						Extensions: &shoot.Extensions{
							DNS: &shoot.DNS{},
						},
					},
				},
				Garden:         &garden.Garden{},
				Logger:         logrus.NewEntry(logger.NewNopLogger()),
				ChartsRootPath: "../../../charts",
			},
		}

		s = runtime.NewScheme()
		Expect(dnsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		Expect(corev1.AddToScheme(s)).NotTo(HaveOccurred())

		seedClient = fake.NewFakeClientWithScheme(s)

		renderer := cr.NewWithServerVersion(&version.Info{})
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion, dnsv1alpha1.SchemeGroupVersion})
		mapper.Add(dnsv1alpha1.SchemeGroupVersion.WithKind("DNSOwner"), meta.RESTScopeRoot)
		chartApplier := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(seedClient, mapper))
		Expect(chartApplier).NotTo(BeNil(), "should return chart applier")

		fakeClientSet := fakeclientset.NewClientSetBuilder().
			WithChartApplier(chartApplier).
			WithDirectClient(seedClient).
			Build()

		b.K8sSeedClient = fakeClientSet
	})

	Describe("#ValidateAuditPolicyApiGroupVersionKind", func() {
		var (
			kind = "Policy"
		)

		It("should return false without error because of version incompatibility", func() {
			incompatibilityMatrix := map[string][]schema.GroupVersionKind{
				"1.10.0": {
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.11.0": {
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
			}

			for shootVersion, gvks := range incompatibilityMatrix {
				for _, gvk := range gvks {
					ok, err := IsValidAuditPolicyVersion(shootVersion, &gvk)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeFalse())
				}
			}
		})

		It("should return true without error because of version compatibility", func() {
			compatibilityMatrix := map[string][]schema.GroupVersionKind{
				"1.10.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
				},
				"1.11.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
				},
				"1.12.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.13.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.14.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
				"1.15.0": {
					auditv1alpha1.SchemeGroupVersion.WithKind(kind),
					auditv1beta1.SchemeGroupVersion.WithKind(kind),
					auditv1.SchemeGroupVersion.WithKind(kind),
				},
			}

			for shootVersion, gvks := range compatibilityMatrix {
				for _, gvk := range gvks {
					ok, err := IsValidAuditPolicyVersion(shootVersion, &gvk)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(BeTrue())
				}
			}
		})

		It("should return false with error because of not valid semver version", func() {
			shootVersion := "1.ab.0"
			gvk := auditv1.SchemeGroupVersion.WithKind(kind)

			ok, err := IsValidAuditPolicyVersion(shootVersion, &gvk)
			Expect(err).To(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	DescribeTable("#getResourcesForAPIServer",
		func(nodes int, storageClass, expectedCPURequest, expectedMemoryRequest, expectedCPULimit, expectedMemoryLimit string) {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit := getResourcesForAPIServer(int32(nodes), storageClass)

			Expect(cpuRequest).To(Equal(expectedCPURequest))
			Expect(memoryRequest).To(Equal(expectedMemoryRequest))
			Expect(cpuLimit).To(Equal(expectedCPULimit))
			Expect(memoryLimit).To(Equal(expectedMemoryLimit))
		},

		// nodes tests
		Entry("nodes <= 2", 2, "", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("nodes <= 10", 10, "", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("nodes <= 50", 50, "", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("nodes <= 100", 100, "", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("nodes > 100", 1000, "", "3000m", "5200Mi", "4000m", "7800Mi"),

		// scaling class tests
		Entry("scaling class small", -1, "small", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("scaling class medium", -1, "medium", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("scaling class large", -1, "large", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("scaling class xlarge", -1, "xlarge", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("scaling class 2xlarge", -1, "2xlarge", "3000m", "5200Mi", "4000m", "7800Mi"),

		// scaling class always decides if provided
		Entry("nodes > 100, scaling class small", 100, "small", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("nodes <= 100, scaling class medium", 100, "medium", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("nodes <= 50, scaling class large", 50, "large", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("nodes <= 10, scaling class xlarge", 10, "xlarge", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("nodes <= 2, scaling class 2xlarge", 2, "2xlarge", "3000m", "5200Mi", "4000m", "7800Mi"),
	)

	Context("setAPIServerAddress", func() {
		It("does nothing when DNS is disabled", func() {
			b.Shoot.DisableDNS = true

			b.setAPIServerAddress("1.2.3.4", seedClient)

			Expect(b.Shoot.Components.Extensions.DNS.InternalOwner).To(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.InternalEntry).To(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalOwner).To(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalEntry).To(BeNil())
		})

		It("sets owners and entries which create DNSOwner and DNSEntry", func() {
			b.Shoot.Info.Status.ClusterIdentity = pointer.StringPtr("shoot-cluster-identity")
			b.Shoot.DisableDNS = false
			b.Shoot.Info.Spec.DNS = &v1beta1.DNS{Domain: pointer.StringPtr("foo")}
			b.Shoot.InternalClusterDomain = "bar"
			b.Shoot.ExternalClusterDomain = pointer.StringPtr("baz")
			b.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}
			b.Garden.InternalDomain = &garden.Domain{Provider: "valid-provider"}

			b.setAPIServerAddress("1.2.3.4", seedClient)

			Expect(b.Shoot.Components.Extensions.DNS.InternalOwner).ToNot(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.InternalOwner.Deploy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.InternalEntry).ToNot(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.InternalEntry.Deploy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalOwner).ToNot(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalOwner.Deploy(ctx)).ToNot(HaveOccurred())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalEntry).ToNot(BeNil())
			Expect(b.Shoot.Components.Extensions.DNS.ExternalEntry.Deploy(ctx)).ToNot(HaveOccurred())

			internalOwner := &dnsv1alpha1.DNSOwner{}
			err := seedClient.Get(ctx, types.NamespacedName{Name: seedNS + "-internal"}, internalOwner)
			Expect(err).ToNot(HaveOccurred())
			internalEntry := &dnsv1alpha1.DNSEntry{}
			err = seedClient.Get(ctx, types.NamespacedName{Name: "internal", Namespace: seedNS}, internalEntry)
			Expect(err).ToNot(HaveOccurred())
			externalOwner := &dnsv1alpha1.DNSOwner{}
			err = seedClient.Get(ctx, types.NamespacedName{Name: seedNS + "-external"}, externalOwner)
			Expect(err).ToNot(HaveOccurred())
			externalEntry := &dnsv1alpha1.DNSEntry{}
			err = seedClient.Get(ctx, types.NamespacedName{Name: "external", Namespace: seedNS}, externalEntry)
			Expect(err).ToNot(HaveOccurred())

			Expect(internalOwner).To(DeepDerivativeEqual(&dnsv1alpha1.DNSOwner{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-ns-internal",
					ResourceVersion: "1",
				},
				Spec: dnsv1alpha1.DNSOwnerSpec{
					OwnerId: "shoot-cluster-identity-internal",
					Active:  pointer.BoolPtr(true),
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
					Active:  pointer.BoolPtr(true),
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
			err = seedClient.Get(ctx, types.NamespacedName{Name: seedNS + "-internal"}, internalOwner)
			Expect(err).To(BeNotFoundError())
			internalEntry = &dnsv1alpha1.DNSEntry{}
			err = seedClient.Get(ctx, types.NamespacedName{Name: "internal", Namespace: seedNS}, internalEntry)
			Expect(err).To(BeNotFoundError())
			externalOwner = &dnsv1alpha1.DNSOwner{}
			err = seedClient.Get(ctx, types.NamespacedName{Name: seedNS + "-external"}, externalOwner)
			Expect(err).To(BeNotFoundError())
			externalEntry = &dnsv1alpha1.DNSEntry{}
			err = seedClient.Get(ctx, types.NamespacedName{Name: "external", Namespace: seedNS}, externalEntry)
			Expect(err).To(BeNotFoundError())
		})
	})

	Describe("SNIPhase", func() {
		var (
			svc *corev1.Service
		)
		BeforeEach(func() {
			gardenletfeatures.RegisterFeatureGates()

			svc = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-apiserver",
					Namespace: seedNS,
				},
			}
		})

		Context("sni enabled", func() {
			BeforeEach(func() {
				Expect(gardenletfeatures.FeatureGate.Set("APIServerSNI=true")).ToNot(HaveOccurred())
				b.Garden.InternalDomain = &garden.Domain{Provider: "some-provider"}
				b.Shoot.Info.Spec.DNS = &v1beta1.DNS{Domain: pointer.StringPtr("foo")}
				b.Shoot.ExternalClusterDomain = pointer.StringPtr("baz")
				b.Shoot.ExternalDomain = &garden.Domain{Provider: "valid-provider"}
			})

			It("returns Enabled for not existing services", func() {
				phase, err := b.SNIPhase(ctx)

				Expect(err).NotTo(HaveOccurred())
				Expect(phase).To(Equal(component.PhaseEnabled))
			})

			It("returns Enabling for service of type LoadBalancer", func() {
				svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				Expect(seedClient.Create(ctx, svc)).NotTo(HaveOccurred())

				phase, err := b.SNIPhase(ctx)

				Expect(err).NotTo(HaveOccurred())
				Expect(phase).To(Equal(component.PhaseEnabling))
			})

			It("returns Enabled for service of type ClusterIP", func() {
				svc.Spec.Type = corev1.ServiceTypeClusterIP
				Expect(seedClient.Create(ctx, svc)).NotTo(HaveOccurred())

				phase, err := b.SNIPhase(ctx)

				Expect(err).NotTo(HaveOccurred())
				Expect(phase).To(Equal(component.PhaseEnabled))
			})

			DescribeTable(
				"return Enabled for service of type",
				func(svcType corev1.ServiceType) {
					svc.Spec.Type = svcType
					Expect(seedClient.Create(ctx, svc)).NotTo(HaveOccurred())

					phase, err := b.SNIPhase(ctx)

					Expect(err).NotTo(HaveOccurred())
					Expect(phase).To(Equal(component.PhaseEnabled))
				},
				Entry("ExternalName", corev1.ServiceTypeExternalName),
				Entry("NodePort", corev1.ServiceTypeNodePort),
			)
		})

		Context("sni disabled", func() {
			BeforeEach(func() {
				Expect(gardenletfeatures.FeatureGate.Set("APIServerSNI=false")).ToNot(HaveOccurred())
			})

			It("returns Disabled for not existing services", func() {
				phase, err := b.SNIPhase(ctx)

				Expect(err).NotTo(HaveOccurred())
				Expect(phase).To(Equal(component.PhaseDisabled))
			})

			It("returns Disabling for service of type ClusterIP", func() {
				svc.Spec.Type = corev1.ServiceTypeClusterIP
				Expect(seedClient.Create(ctx, svc)).NotTo(HaveOccurred())

				phase, err := b.SNIPhase(ctx)

				Expect(err).NotTo(HaveOccurred())
				Expect(phase).To(Equal(component.PhaseDisabling))
			})

			DescribeTable(
				"return Disabled for service of type",
				func(svcType corev1.ServiceType) {
					svc.Spec.Type = svcType
					Expect(seedClient.Create(ctx, svc)).NotTo(HaveOccurred())

					phase, err := b.SNIPhase(ctx)

					Expect(err).NotTo(HaveOccurred())
					Expect(phase).To(Equal(component.PhaseDisabled))
				},
				Entry("ExternalName", corev1.ServiceTypeExternalName),
				Entry("LoadBalancer", corev1.ServiceTypeLoadBalancer),
				Entry("NodePort", corev1.ServiceTypeNodePort),
			)
		})
	})
})
