// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/rotation"
)

// ExternalDomainVerifier verifies the migration of the external domain which runs as part of the certificate
// authorities rotation.
type ExternalDomainVerifier struct {
	*ShootContext

	// NewDomain is the domain the shoot is migrated to. It is set by the test before the rotation is started.
	NewDomain string

	priorDomain string
}

// Before is called before the rotation is started.
func (v *ExternalDomainVerifier) Before(_ context.Context) {
	It("Remember the external domain before the migration", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.GardenClient.Get(ctx, client.ObjectKeyFromObject(v.Shoot), v.Shoot)).To(Succeed())
			g.Expect(v.Shoot.Spec.DNS).NotTo(BeNil())
			g.Expect(v.Shoot.Spec.DNS.Domain).NotTo(BeNil())
			v.priorDomain = *v.Shoot.Spec.DNS.Domain
			g.Expect(v.priorDomain).NotTo(Equal(v.NewDomain))

			g.Expect(v.advertisedAddress(v1beta1constants.AdvertisedAddressExternal)).To(Equal(apiServerURL(v.priorDomain)))
			g.Expect(v.advertisedAddress(v1beta1constants.AdvertisedAddressPriorExternal)).To(BeEmpty())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *ExternalDomainVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(v.Shoot.Spec.DNS.Domain).To(HaveValue(Equal(v.NewDomain)))
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *ExternalDomainVerifier) ExpectPreparingWithoutWorkersRolloutStatus(g Gomega) {
	g.Expect(v.Shoot.Spec.DNS.Domain).To(HaveValue(Equal(v.NewDomain)))
}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *ExternalDomainVerifier) ExpectWaitingForWorkersRolloutStatus(g Gomega) {
	g.Expect(v.Shoot.Spec.DNS.Domain).To(HaveValue(Equal(v.NewDomain)))
}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *ExternalDomainVerifier) AfterPrepared(_ context.Context) {
	It("Verify that both external domains are advertised", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.GardenClient.Get(ctx, client.ObjectKeyFromObject(v.Shoot), v.Shoot)).To(Succeed())
			g.Expect(v.advertisedAddress(v1beta1constants.AdvertisedAddressExternal)).To(Equal(apiServerURL(v.NewDomain)))
			g.Expect(v.advertisedAddress(v1beta1constants.AdvertisedAddressPriorExternal)).To(Equal(apiServerURL(v.priorDomain)))
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Verify that both external DNSRecords exist", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.dnsRecordName(ctx, g, v1beta1constants.LabelDNSRecordExternal)).To(Equal(v1beta1helper.GetAPIServerDomain(v.NewDomain)))
			g.Expect(v.dnsRecordName(ctx, g, v1beta1constants.LabelDNSRecordPriorExternal)).To(Equal(v1beta1helper.GetAPIServerDomain(v.priorDomain)))
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Verify that both external domains resolve to the same address", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			newAddresses, err := net.DefaultResolver.LookupHost(ctx, v1beta1helper.GetAPIServerDomain(v.NewDomain))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(newAddresses).NotTo(BeEmpty())

			priorAddresses, err := net.DefaultResolver.LookupHost(ctx, v1beta1helper.GetAPIServerDomain(v.priorDomain))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(priorAddresses).To(ConsistOf(newAddresses))
		}).Should(Succeed())
	}, SpecTimeout(2*time.Minute))

	It("Verify that the server certificate contains both external domains", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			dnsNames := v.serverCertificateDNSNames(ctx, g)
			g.Expect(dnsNames).To(ContainElements(v.NewDomain, v1beta1helper.GetAPIServerDomain(v.NewDomain)))
			g.Expect(dnsNames).To(ContainElements(v.priorDomain, v1beta1helper.GetAPIServerDomain(v.priorDomain)))
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *ExternalDomainVerifier) ExpectCompletingStatus(g Gomega) {
	g.Expect(v.Shoot.Spec.DNS.Domain).To(HaveValue(Equal(v.NewDomain)))
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *ExternalDomainVerifier) AfterCompleted(_ context.Context) {
	It("Verify that only the new external domain resolves", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			_, err := net.DefaultResolver.LookupHost(ctx, v1beta1helper.GetAPIServerDomain(v.NewDomain))
			g.Expect(err).NotTo(HaveOccurred())

			_, err = net.DefaultResolver.LookupHost(ctx, v1beta1helper.GetAPIServerDomain(v.priorDomain))
			g.Expect(err).To(HaveOccurred(), "the prior domain must not resolve any more")
		}).WithPolling(10 * time.Second).Should(Succeed())
	}, SpecTimeout(5*time.Minute))

	It("Verify that only the new external domain is left", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.GardenClient.Get(ctx, client.ObjectKeyFromObject(v.Shoot), v.Shoot)).To(Succeed())
			g.Expect(v.advertisedAddress(v1beta1constants.AdvertisedAddressExternal)).To(Equal(apiServerURL(v.NewDomain)))
			g.Expect(v.advertisedAddress(v1beta1constants.AdvertisedAddressPriorExternal)).To(BeEmpty())

			g.Expect(v.dnsRecordName(ctx, g, v1beta1constants.LabelDNSRecordExternal)).To(Equal(v1beta1helper.GetAPIServerDomain(v.NewDomain)))
			g.Expect(v.dnsRecordName(ctx, g, v1beta1constants.LabelDNSRecordPriorExternal)).To(BeEmpty())

			dnsNames := v.serverCertificateDNSNames(ctx, g)
			g.Expect(dnsNames).To(ContainElements(v.NewDomain, v1beta1helper.GetAPIServerDomain(v.NewDomain)))
			g.Expect(dnsNames).NotTo(ContainElement(v1beta1helper.GetAPIServerDomain(v.priorDomain)))
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

func (v *ExternalDomainVerifier) advertisedAddress(name string) string {
	for _, address := range v.Shoot.Status.AdvertisedAddresses {
		if address.Name == name {
			return address.URL
		}
	}

	return ""
}

func (v *ExternalDomainVerifier) dnsRecordName(ctx context.Context, g Gomega, role string) string {
	dnsRecordList := &extensionsv1alpha1.DNSRecordList{}
	g.Expect(v.SeedClient.List(ctx, dnsRecordList,
		client.InNamespace(v.Shoot.Status.TechnicalID),
		client.MatchingLabels{v1beta1constants.LabelRole: role},
	)).To(Succeed())
	g.Expect(len(dnsRecordList.Items)).To(BeNumerically("<=", 1), "there must be at most one DNSRecord per role")

	if len(dnsRecordList.Items) == 0 {
		return ""
	}

	return dnsRecordList.Items[0].Spec.Name
}

func (v *ExternalDomainVerifier) serverCertificateDNSNames(ctx context.Context, g Gomega) []string {
	secretList := &corev1.SecretList{}
	g.Expect(v.SeedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), ManagedByGardenletSecretsManager)).To(Succeed())

	serverCertificates := rotation.GroupByName(secretList.Items)["kube-apiserver"]
	g.Expect(serverCertificates).NotTo(BeEmpty(), "the server certificate of the kube-apiserver must exist")
	secret := serverCertificates[len(serverCertificates)-1]

	block, _ := pem.Decode(secret.Data["tls.crt"])
	g.Expect(block).NotTo(BeNil(), "server certificate must be PEM encoded")

	certificate, err := x509.ParseCertificate(block.Bytes)
	g.Expect(err).NotTo(HaveOccurred())

	return certificate.DNSNames
}

func apiServerURL(domain string) string {
	return "https://" + v1beta1helper.GetAPIServerDomain(domain)
}
