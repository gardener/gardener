// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("SelfHostedShootIngressDNSRecord", func() {
	const (
		shootName       = "foo"
		domain          = "foo.example.com"
		provider        = "some-provider"
		zone            = "some-zone"
		ttl       int64 = 300
	)

	var (
		b              *Botanist
		shootInfo      *gardencorev1beta1.Shoot
		externalDomain *gardenerutils.Domain
	)

	newShootInfo := func(selfHosted bool, withDomain bool) *gardencorev1beta1.Shoot {
		s := &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: shootName},
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{Version: "1.31.1"},
			},
		}
		if selfHosted {
			s.Spec.Provider.Workers = []gardencorev1beta1.Worker{{
				Name:         "cp",
				ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
			}}
		}
		if withDomain {
			s.Spec.DNS = &gardencorev1beta1.DNS{Domain: new(domain)}
		}
		return s
	}

	JustBeforeEach(func() {
		b = &Botanist{Operation: &operation.Operation{
			Config: &gardenletconfigv1alpha1.GardenletConfiguration{
				Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
					Shoot: &gardenletconfigv1alpha1.ShootControllerConfiguration{
						DNSEntryTTLSeconds: new(ttl),
					},
				},
			},
			Shoot: &shoot.Shoot{
				ControlPlaneNamespace: v1beta1constants.GardenNamespace,
				ExternalClusterDomain: new(domain),
				ExternalDomain:        externalDomain,
			},
		}}
		b.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeclient.NewClientBuilder().Build()).Build()
		b.Shoot.SetInfo(shootInfo)
	})

	Describe("#DefaultSelfHostedShootIngressDNSRecord", func() {
		When("the shoot is not self-hosted", func() {
			BeforeEach(func() {
				shootInfo = newShootInfo(false, true)
				externalDomain = &gardenerutils.Domain{Domain: domain, Provider: provider}
			})

			It("should return nil", func() {
				Expect(b.DefaultSelfHostedShootIngressDNSRecord()).To(BeNil())
			})
		})

		When("the self-hosted shoot does not need external DNS", func() {
			BeforeEach(func() {
				// No DNS domain => NeedsExternalDNS() is false.
				shootInfo = newShootInfo(true, false)
				externalDomain = nil
			})

			It("should return nil", func() {
				Expect(b.DefaultSelfHostedShootIngressDNSRecord()).To(BeNil())
			})
		})

		When("the self-hosted shoot uses an unmanaged DNS provider", func() {
			BeforeEach(func() {
				shootInfo = newShootInfo(true, true)
				externalDomain = &gardenerutils.Domain{Domain: domain, Provider: "unmanaged"}
			})

			It("should return nil", func() {
				Expect(b.DefaultSelfHostedShootIngressDNSRecord()).To(BeNil())
			})
		})

		When("the self-hosted shoot needs external DNS", func() {
			BeforeEach(func() {
				shootInfo = newShootInfo(true, true)
				externalDomain = &gardenerutils.Domain{
					Domain:   domain,
					Provider: provider,
					Zone:     zone,
					Credentials: &corev1.Secret{
						Data: map[string][]byte{"foo": []byte("bar")},
					},
				}
			})

			It("should return a correctly-configured deployer", func() {
				deployer := b.DefaultSelfHostedShootIngressDNSRecord()
				Expect(deployer).NotTo(BeNil())

				values := deployer.GetValues()
				Expect(values.Name).To(Equal(shootName + "-" + v1beta1constants.LabelDNSRecordIngress))
				Expect(values.SecretName).To(Equal("dnsrecord-" + shootName + "-" + v1beta1constants.LabelDNSRecordIngress))
				Expect(values.Namespace).To(Equal(v1beta1constants.GardenNamespace))
				Expect(values.DNSName).To(Equal("*.ingress." + domain))
				Expect(values.Type).To(Equal(provider))
				Expect(values.Zone).To(HaveValue(Equal(zone)))
				Expect(values.TTL).To(HaveValue(Equal(ttl)))
				Expect(values.ReconcileOnlyOnChangeOrError).To(BeTrue())
				Expect(values.Labels).To(HaveKeyWithValue(v1beta1constants.LabelRole, v1beta1constants.LabelDNSRecordIngress))
				Expect(values.Labels).To(HaveKeyWithValue(v1beta1constants.GardenRole, v1beta1constants.GardenRoleControlPlane))
			})
		})

		When("the external domain has no zone", func() {
			BeforeEach(func() {
				shootInfo = newShootInfo(true, true)
				externalDomain = &gardenerutils.Domain{Domain: domain, Provider: provider}
			})

			It("should leave the Zone unset", func() {
				deployer := b.DefaultSelfHostedShootIngressDNSRecord()
				Expect(deployer).NotTo(BeNil())
				Expect(deployer.GetValues().Zone).To(BeNil())
			})
		})
	})
})
