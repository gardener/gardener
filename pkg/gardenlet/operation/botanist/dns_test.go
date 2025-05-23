// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mockdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("dns", func() {
	const (
		controlPlaneNamespace = "test-ns"
		shootNS               = "shoot-ns"
	)

	var (
		b                        *Botanist
		seedClient, gardenClient client.Client
		s                        *runtime.Scheme

		dnsEntryTTL int64 = 1234
	)

	BeforeEach(func() {
		b = &Botanist{
			Operation: &operation.Operation{
				Config: &gardenletconfigv1alpha1.GardenletConfiguration{
					Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
						Shoot: &gardenletconfigv1alpha1.ShootControllerConfiguration{
							DNSEntryTTLSeconds: &dnsEntryTTL,
						},
					},
				},
				Shoot: &shootpkg.Shoot{
					Components: &shootpkg.Components{
						Extensions: &shootpkg.Extensions{},
					},
					ControlPlaneNamespace: controlPlaneNamespace,
				},
				Garden: &garden.Garden{},
				Logger: logr.Discard(),
			},
		}
		b.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Namespace: shootNS},
		})

		s = runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).NotTo(HaveOccurred())

		gardenClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		seedClient = fake.NewClientBuilder().WithScheme(s).Build()

		renderer := chartrenderer.NewWithServerVersion(&version.Info{})
		chartApplier := kubernetes.NewChartApplier(renderer, kubernetes.NewApplier(seedClient, meta.NewDefaultRESTMapper([]schema.GroupVersion{})))
		Expect(chartApplier).NotTo(BeNil(), "should return chart applier")

		b.GardenClient = gardenClient
		b.SeedClientSet = fakekubernetes.NewClientSetBuilder().
			WithClient(seedClient).
			WithChartApplier(chartApplier).
			Build()
	})

	Context("NeedsExternalDNS", func() {
		It("should be false when Shoot's DNS is nil", func() {
			b.Shoot.GetInfo().Spec.DNS = nil
			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be false when Shoot DNS's domain is nil", func() {
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: nil}
			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be false when Shoot ExternalClusterDomain is nil", func() {
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To("foo")}
			b.Shoot.ExternalClusterDomain = nil
			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be false when Shoot ExternalDomain is nil", func() {
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To("foo")}
			b.Shoot.ExternalClusterDomain = ptr.To("baz")
			b.Shoot.ExternalDomain = nil

			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be false when Shoot ExternalDomain provider is unamanaged", func() {
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To("foo")}
			b.Shoot.ExternalClusterDomain = ptr.To("baz")
			b.Shoot.ExternalDomain = &gardenerutils.Domain{Provider: "unmanaged"}

			Expect(b.NeedsExternalDNS()).To(BeFalse())
		})

		It("should be true when Shoot ExternalDomain provider is valid", func() {
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To("foo")}
			b.Shoot.ExternalClusterDomain = ptr.To("baz")
			b.Shoot.ExternalDomain = &gardenerutils.Domain{Provider: "valid-provider"}

			Expect(b.NeedsExternalDNS()).To(BeTrue())
		})
	})

	Context("NeedsInternalDNS", func() {
		It("should be false when the internal domain is nil", func() {
			b.Garden.InternalDomain = nil
			Expect(b.NeedsInternalDNS()).To(BeFalse())
		})

		It("should be false when the internal domain provider is unmanaged", func() {
			b.Garden.InternalDomain = &gardenerutils.Domain{Provider: "unmanaged"}
			Expect(b.NeedsInternalDNS()).To(BeFalse())
		})

		It("should be true when the internal domain provider is not unmanaged", func() {
			b.Garden.InternalDomain = &gardenerutils.Domain{Provider: "some-provider"}
			Expect(b.NeedsInternalDNS()).To(BeTrue())
		})
	})

	Context("ShootUsesDNS", func() {
		BeforeEach(func() {
			gardenletfeatures.RegisterFeatureGates()
		})

		It("returns true when DNS is used", func() {
			b.Garden.InternalDomain = &gardenerutils.Domain{Provider: "some-provider"}
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To("foo")}
			b.Shoot.ExternalClusterDomain = ptr.To("baz")
			b.Shoot.ExternalDomain = &gardenerutils.Domain{Provider: "valid-provider"}

			Expect(b.ShootUsesDNS()).To(BeTrue())
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

		It("sets internal and external DNSRecords", func() {
			b.Shoot.GetInfo().Status.ClusterIdentity = ptr.To("shoot-cluster-identity")
			b.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{Domain: ptr.To("foo")}
			b.Shoot.InternalClusterDomain = "bar"
			b.Shoot.ExternalClusterDomain = ptr.To("baz")
			b.Shoot.ExternalDomain = &gardenerutils.Domain{Provider: "valid-provider"}
			b.Garden.InternalDomain = &gardenerutils.Domain{Provider: "valid-provider"}

			externalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			externalDNSRecord.EXPECT().SetValues([]string{"1.2.3.4"})
			internalDNSRecord.EXPECT().SetRecordType(extensionsv1alpha1.DNSRecordTypeA)
			internalDNSRecord.EXPECT().SetValues([]string{"1.2.3.4"})

			b.newDNSComponentsTargetingAPIServerAddress()
		})
	})
})
