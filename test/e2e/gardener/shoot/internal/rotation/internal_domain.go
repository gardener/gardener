// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package rotation

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// InternalDomainVerifier verifies the migration of the internal domain which runs as part of the certificate
// authorities rotation. The test changes the internal domain of the seed and confirms the migration before the rotation
// is started.
type InternalDomainVerifier struct {
	*ShootContext

	// NewDomain is the internal cluster domain the shoot is migrated to. It is empty if the seed disables the internal
	// domain. It must be set before the rotation is started.
	NewDomain string

	priorDomain    string
	externalDomain string
}

// Before is called before the rotation is started.
func (v *InternalDomainVerifier) Before(_ context.Context) {
	It("Remember the internal domain before the migration", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.GardenClient.Get(ctx, client.ObjectKeyFromObject(v.Shoot), v.Shoot)).To(Succeed())
			g.Expect(v.Shoot.Spec.DNS).NotTo(BeNil())
			g.Expect(v.Shoot.Spec.DNS.Domain).NotTo(BeNil())
			v.externalDomain = *v.Shoot.Spec.DNS.Domain
			v.priorDomain = strings.TrimPrefix(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressInternal), apiServerURL(""))
			g.Expect(v.priorDomain).NotTo(Equal(v.NewDomain))
			g.Expect(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressPriorInternal)).To(BeEmpty())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Wait until the shoot has to migrate its internal domain", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.GardenClient.Get(ctx, client.ObjectKeyFromObject(v.Shoot), v.Shoot)).To(Succeed())
			constraint := v1beta1helper.GetCondition(v.Shoot.Status.Constraints, gardencorev1beta1.ShootInternalDomainUpToDate)
			g.Expect(constraint).NotTo(BeNil())
			g.Expect(constraint.Status).To(Equal(gardencorev1beta1.ConditionFalse))
			g.Expect(constraint.Reason).To(Equal("InternalDomainMigrationPending"))
		}).WithPolling(10 * time.Second).Should(Succeed())
	}, SpecTimeout(5*time.Minute))
}

// ExpectPreparingStatus is called while waiting for the Preparing status.
func (v *InternalDomainVerifier) ExpectPreparingStatus(g Gomega) {
	g.Expect(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressInternal)).To(Equal(optionalAPIServerURL(v.NewDomain)))
}

// ExpectPreparingWithoutWorkersRolloutStatus is called while waiting for the PreparingWithoutWorkersRollout status.
func (v *InternalDomainVerifier) ExpectPreparingWithoutWorkersRolloutStatus(g Gomega) {
	v.ExpectPreparingStatus(g)
}

// ExpectWaitingForWorkersRolloutStatus is called while waiting for the WaitingForWorkersRollout status.
func (v *InternalDomainVerifier) ExpectWaitingForWorkersRolloutStatus(g Gomega) {
	v.ExpectPreparingStatus(g)
}

// AfterPrepared is called when the Shoot is in Prepared status.
func (v *InternalDomainVerifier) AfterPrepared(_ context.Context) {
	It("Verify that the new and the prior internal domain are advertised", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.GardenClient.Get(ctx, client.ObjectKeyFromObject(v.Shoot), v.Shoot)).To(Succeed())
			g.Expect(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressInternal)).To(Equal(optionalAPIServerURL(v.NewDomain)))
			g.Expect(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressPriorInternal)).To(Equal(optionalAPIServerURL(v.priorDomain)))
			g.Expect(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressServiceAccountIssuer)).To(Equal(v.newIssuer()))

			// Only a shoot without an internal domain records its issuer, because no domain gives it any more.
			priorIssuerAddress := ""
			if v.priorDomain == "" {
				priorIssuerAddress = v.priorIssuer()
			}
			g.Expect(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressPriorServiceAccountIssuer)).To(Equal(priorIssuerAddress))
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Verify the DNSRecords of the new and the prior internal domain", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(dnsRecordName(ctx, g, v.ShootContext, v1beta1constants.LabelDNSRecordInternal)).To(Equal(apiServerDomain(v.NewDomain)))
			g.Expect(dnsRecordName(ctx, g, v.ShootContext, v1beta1constants.LabelDNSRecordPriorInternal)).To(Equal(apiServerDomain(v.priorDomain)))
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Verify that the new and the prior internal domain resolve", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			for _, domain := range []string{v.NewDomain, v.priorDomain} {
				if domain == "" {
					continue
				}
				addresses, err := net.DefaultResolver.LookupHost(ctx, apiServerDomain(domain))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(addresses).NotTo(BeEmpty())
			}
		}).Should(Succeed())
	}, SpecTimeout(2*time.Minute))

	It("Verify that the server certificate contains the new and the prior internal domain", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			dnsNames := serverCertificateDNSNames(ctx, g, v.ShootContext)
			for _, domain := range []string{v.NewDomain, v.priorDomain} {
				if domain != "" {
					g.Expect(dnsNames).To(ContainElement(apiServerDomain(domain)))
				}
			}
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Verify that the kube-apiserver issues tokens for the new issuer and accepts the prior issuer", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.serviceAccountIssuers(ctx, g)).To(Equal([]string{v.newIssuer(), v.priorIssuer()}))
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	It("Verify that the shoot access tokens have the new issuer", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.accessTokenIssuers(ctx, g)).To(HaveEach(v.newIssuer()))
		}).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ExpectCompletingStatus is called while waiting for the Completing status.
func (v *InternalDomainVerifier) ExpectCompletingStatus(g Gomega) {
	v.ExpectPreparingStatus(g)
}

// AfterCompleted is called when the Shoot is in Completed status.
func (v *InternalDomainVerifier) AfterCompleted(_ context.Context) {
	It("Verify that the prior internal domain does not resolve any more", func(ctx SpecContext) {
		if v.priorDomain == "" {
			Skip("the shoot had no internal domain before the migration")
		}

		Eventually(ctx, func(g Gomega) {
			_, err := net.DefaultResolver.LookupHost(ctx, apiServerDomain(v.priorDomain))
			g.Expect(err).To(HaveOccurred(), "the prior internal domain must not resolve any more")
		}).WithPolling(10 * time.Second).Should(Succeed())
	}, SpecTimeout(5*time.Minute))

	It("Verify that only the new internal domain is left", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(v.GardenClient.Get(ctx, client.ObjectKeyFromObject(v.Shoot), v.Shoot)).To(Succeed())
			g.Expect(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressInternal)).To(Equal(optionalAPIServerURL(v.NewDomain)))
			g.Expect(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressPriorInternal)).To(BeEmpty())
			g.Expect(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressServiceAccountIssuer)).To(Equal(v.newIssuer()))
			g.Expect(advertisedAddress(v.Shoot, v1beta1constants.AdvertisedAddressPriorServiceAccountIssuer)).To(BeEmpty())

			constraint := v1beta1helper.GetCondition(v.Shoot.Status.Constraints, gardencorev1beta1.ShootInternalDomainUpToDate)
			g.Expect(constraint).To(Or(BeNil(), HaveField("Status", gardencorev1beta1.ConditionTrue)))

			g.Expect(dnsRecordName(ctx, g, v.ShootContext, v1beta1constants.LabelDNSRecordInternal)).To(Equal(apiServerDomain(v.NewDomain)))
			g.Expect(dnsRecordName(ctx, g, v.ShootContext, v1beta1constants.LabelDNSRecordPriorInternal)).To(BeEmpty())

			if v.priorDomain != "" {
				g.Expect(serverCertificateDNSNames(ctx, g, v.ShootContext)).NotTo(ContainElement(apiServerDomain(v.priorDomain)))
			}

			g.Expect(v.serviceAccountIssuers(ctx, g)).To(Equal([]string{v.newIssuer()}))
			g.Expect(v.accessTokenIssuers(ctx, g)).To(HaveEach(v.newIssuer()))
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// newIssuer returns the default service account issuer after the migration. Without an internal domain, it is the
// external domain.
func (v *InternalDomainVerifier) newIssuer() string {
	return apiServerURL(cmp.Or(v.NewDomain, v.externalDomain))
}

// priorIssuer returns the default service account issuer before the migration.
func (v *InternalDomainVerifier) priorIssuer() string {
	return apiServerURL(cmp.Or(v.priorDomain, v.externalDomain))
}

func optionalAPIServerURL(domain string) string {
	if domain == "" {
		return ""
	}
	return apiServerURL(domain)
}

func (v *InternalDomainVerifier) serviceAccountIssuers(ctx context.Context, g Gomega) []string {
	deployment := &appsv1.Deployment{}
	g.Expect(v.SeedClient.Get(ctx, client.ObjectKey{Namespace: v.Shoot.Status.TechnicalID, Name: v1beta1constants.DeploymentNameKubeAPIServer}, deployment)).To(Succeed())

	var issuers []string
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name != kubeapiserver.ContainerNameKubeAPIServer {
			continue
		}
		for _, arg := range container.Args {
			if issuer, ok := strings.CutPrefix(arg, "--service-account-issuer="); ok {
				issuers = append(issuers, issuer)
			}
		}
	}

	return issuers
}

func (v *InternalDomainVerifier) accessTokenIssuers(ctx context.Context, g Gomega) []string {
	secretList := &corev1.SecretList{}
	g.Expect(v.SeedClient.List(ctx, secretList, client.InNamespace(v.Shoot.Status.TechnicalID), client.MatchingLabels{
		resourcesv1alpha1.ResourceManagerPurpose: resourcesv1alpha1.LabelPurposeTokenRequest,
		resourcesv1alpha1.ResourceManagerClass:   resourcesv1alpha1.ResourceManagerClassShoot,
	})).To(Succeed())
	g.Expect(secretList.Items).NotTo(BeEmpty(), "the shoot access secrets must exist")

	var issuers []string
	for _, secret := range secretList.Items {
		if _, ok := secret.Annotations[resourcesv1alpha1.TokenRequestorTargetSecretName]; ok {
			continue
		}

		parts := strings.Split(accessToken(g, secret), ".")
		g.Expect(parts).To(HaveLen(3), "secret %s must contain a JWT", secret.Name)

		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		g.Expect(err).NotTo(HaveOccurred())

		var claims struct {
			Issuer string `json:"iss"`
		}
		g.Expect(json.Unmarshal(payload, &claims)).To(Succeed())
		issuers = append(issuers, claims.Issuer)
	}

	return issuers
}

func accessToken(g Gomega, secret corev1.Secret) string {
	kubeconfigData, ok := secret.Data[resourcesv1alpha1.DataKeyKubeconfig]
	if !ok {
		return string(secret.Data[resourcesv1alpha1.DataKeyToken])
	}

	kubeconfig, err := clientcmd.Load(kubeconfigData)
	g.Expect(err).NotTo(HaveOccurred())
	for _, authInfo := range kubeconfig.AuthInfos {
		if authInfo.Token != "" {
			return authInfo.Token
		}
	}

	return ""
}

func apiServerDomain(domain string) string {
	if domain == "" {
		return ""
	}
	return v1beta1helper.GetAPIServerDomain(domain)
}
