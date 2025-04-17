// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("utils", func() {
	DescribeTable("#ComputeGardenNamespace",
		func(name, expected string) {
			Expect(ComputeGardenNamespace(name)).To(Equal(expected))
		},

		Entry("empty name", "", "seed-"),
		Entry("garden", "garden", "seed-garden"),
		Entry("dash", "-", "seed--"),
		Entry("garden prefixed with dash", "-garden", "seed--garden"),
	)

	DescribeTable("#ComputeSeedName",
		func(name, expected string) {
			Expect(ComputeSeedName(name)).To(Equal(expected))
		},

		Entry("expect error with empty name", "", ""),
		Entry("expect error with garden name", "garden", ""),
		Entry("expect error with dash", "-", ""),
		Entry("expect success with empty name", "seed-", ""),
		Entry("expect success with dash name", "seed--", "-"),
		Entry("expect success with duplicated prefix", "seed-seed-", "seed-"),
		Entry("expect success with duplicated prefix", "seed-seed-a", "seed-a"),
		Entry("expect success with garden name", "seed-garden", "garden"),
	)

	DescribeTable("#IsSeedClientCert",
		func(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage, expectedStatus bool, expectedReason gomegatypes.GomegaMatcher) {
			status, reason := IsSeedClientCert(x509cr, usages)
			Expect(status).To(Equal(expectedStatus))
			Expect(reason).To(expectedReason)
		},

		Entry("org does not match", &x509.CertificateRequest{}, nil, false, ContainSubstring("organization")),
		Entry("dns names given", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}, DNSNames: []string{"foo"}}, nil, false, ContainSubstring("DNSNames")),
		Entry("email addresses given", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}, EmailAddresses: []string{"foo"}}, nil, false, ContainSubstring("EmailAddresses")),
		Entry("ip addresses given", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}, IPAddresses: []net.IP{{}}}, nil, false, ContainSubstring("IPAddresses")),
		Entry("key usages do not match", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}}, nil, false, ContainSubstring("key usages")),
		Entry("common name does not match", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, false, ContainSubstring("CommonName")),
		Entry("everything matches", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}, CommonName: "gardener.cloud:system:seed:foo"}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, true, Equal("")),
	)

	Describe("#GetWildcardCertificate", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client
			secret     *corev1.Secret
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    "garden",
					Labels:       map[string]string{"gardener.cloud/role": "controlplane-cert"},
				},
			}
		})

		It("should return an error because there are more than one wildcard certificates", func() {
			secret2 := secret.DeepCopy()
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

			result, err := GetWildcardCertificate(ctx, fakeClient)
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("misconfigured cluster: not possible to provide more than one secret with label")))
		})

		It("should return the wildcard certificate secret", func() {
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := GetWildcardCertificate(ctx, fakeClient)
			Expect(result).To(Equal(secret))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return nil because there is no wildcard certificate secret", func() {
			result, err := GetWildcardCertificate(ctx, fakeClient)
			Expect(result).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#ComputeRequiredExtensionsForSeed", func() {
		var (
			seed                       *gardencorev1beta1.Seed
			controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList
		)

		const (
			extensionType1 = "extension1"
			extensionType2 = "extension2"
			extensionType3 = "extension3"
		)

		BeforeEach(func() {
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{Type: "providerA"},
				},
			}

			controllerRegistrationList = &gardencorev1beta1.ControllerRegistrationList{
				Items: []gardencorev1beta1.ControllerRegistration{
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind:       extensionsv1alpha1.ExtensionResource,
									Type:       extensionType1,
									AutoEnable: []gardencorev1beta1.AutoEnableMode{"shoot", "seed"},
								},
							},
						},
					},
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind:       extensionsv1alpha1.ExtensionResource,
									Type:       extensionType2,
									AutoEnable: []gardencorev1beta1.AutoEnableMode{"shoot"},
								},
							},
						},
					},
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind: extensionsv1alpha1.ContainerRuntimeResource,
									Type: extensionType3,
								},
							},
						},
					},
					{
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind:       extensionsv1alpha1.ExtensionResource,
									Type:       extensionType3,
									AutoEnable: []gardencorev1beta1.AutoEnableMode{"seed"},
								},
							},
						},
					},
				},
			}
		})

		It("should return the required types for seed", func() {
			Expect(ComputeRequiredExtensionsForSeed(seed, controllerRegistrationList).UnsortedList()).To(ConsistOf(
				"Extension/extension1",
				"Extension/extension3",
				"ControlPlane/providerA",
				"Infrastructure/providerA",
				"Worker/providerA",
			))
		})

		When("seed has DNS provider", func() {
			BeforeEach(func() {
				seed.Spec.DNS = gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{Type: "providerB"},
				}
			})

			It("should return the required types for seed", func() {
				Expect(ComputeRequiredExtensionsForSeed(seed, controllerRegistrationList).UnsortedList()).To(ConsistOf(
					"Extension/extension1",
					"Extension/extension3",
					"DNSRecord/providerB",
					"ControlPlane/providerA",
					"Infrastructure/providerA",
					"Worker/providerA",
				))
			})
		})

		When("seed has extensions", func() {
			BeforeEach(func() {
				seed.Spec.Extensions = []gardencorev1beta1.Extension{
					{Type: "extensionA"},
				}
			})

			It("should return the required types for seed", func() {
				Expect(ComputeRequiredExtensionsForSeed(seed, controllerRegistrationList).UnsortedList()).To(ConsistOf(
					"Extension/extensionA",
					"Extension/extension1",
					"Extension/extension3",
					"ControlPlane/providerA",
					"Infrastructure/providerA",
					"Worker/providerA",
				))
			})

			It("should exclude disabled extensions", func() {
				seed.Spec.Extensions = append(seed.Spec.Extensions, gardencorev1beta1.Extension{Type: "extension3", Disabled: ptr.To(true)})

				Expect(ComputeRequiredExtensionsForSeed(seed, controllerRegistrationList).UnsortedList()).To(ConsistOf(
					"Extension/extensionA",
					"Extension/extension1",
					"ControlPlane/providerA",
					"Infrastructure/providerA",
					"Worker/providerA",
				))
			})
		})
	})

	Describe("#ExtensionKindAndTypeForID", func() {
		It("should return the expected kind and type", func() {
			extKind, extType, err := ExtensionKindAndTypeForID("extension/test")
			Expect(err).NotTo(HaveOccurred())
			Expect(extKind).To(Equal("extension"))
			Expect(extType).To(Equal("test"))
		})

		It("should return an error when separator is invalid", func() {
			extKind, extType, err := ExtensionKindAndTypeForID("extension-test")
			Expect(err).To(MatchError(ContainSubstring("unexpected required extension")))
			Expect(extKind).To(BeEmpty())
			Expect(extType).To(BeEmpty())
		})

		It("should return an error when format is invalid", func() {
			extKind, extType, err := ExtensionKindAndTypeForID("extension/backupbucket/test")
			Expect(err).To(MatchError(ContainSubstring("unexpected required extension")))
			Expect(extKind).To(BeEmpty())
			Expect(extType).To(BeEmpty())
		})
	})

	Describe("#RequiredExtensionsReady", func() {
		var (
			ctx        context.Context
			fakeClient client.Client

			controllerRegistrations []*gardencorev1beta1.ControllerRegistration
			controllerInstallations []*gardencorev1beta1.ControllerInstallation

			seedProvider       string
			dnsProvider        string
			seedName           string
			requiredExtensions sets.Set[string]
		)

		BeforeEach(func() {
			ctx = context.Background()
			fakeClient = fakeclient.
				NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithIndex(
					&gardencorev1beta1.ControllerInstallation{},
					core.SeedRefName,
					indexer.ControllerInstallationSeedRefNameIndexerFunc,
				).
				Build()

			seedProvider = "seedProvider"
			dnsProvider = "dnsProvider"

			seedName = "seed"

			requiredExtensions = sets.New(
				"DNSRecord/"+dnsProvider,
				"ControlPlane/"+seedProvider,
				"Infrastructure/"+seedProvider,
				"Worker/"+seedProvider,
			)
		})

		JustBeforeEach(func() {
			for _, controllerReg := range controllerRegistrations {
				Expect(fakeClient.Create(ctx, controllerReg)).To(Succeed())
			}
			for _, controllerInst := range controllerInstallations {
				Expect(fakeClient.Create(ctx, controllerInst)).To(Succeed())
			}
		})

		Context("when required ControllerInstallations are missing", func() {
			It("should fail checking all required extensions", func() {
				Expect(RequiredExtensionsReady(ctx, fakeClient, seedName, requiredExtensions)).To(MatchError("extension controllers missing or unready: map[ControlPlane/seedProvider:{} DNSRecord/dnsProvider:{} Infrastructure/seedProvider:{} Worker/seedProvider:{}]"))
			})
		})

		Context("when referenced ControllerRegistration is missing", func() {
			BeforeEach(func() {
				controllerInstallations = []*gardencorev1beta1.ControllerInstallation{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "seedProviderExtension",
						},
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							RegistrationRef: corev1.ObjectReference{
								Name: "foo",
							},
							SeedRef: corev1.ObjectReference{
								Name: seedName,
							},
						},
						Status: gardencorev1beta1.ControllerInstallationStatus{
							Conditions: []gardencorev1beta1.Condition{
								{Type: gardencorev1beta1.ControllerInstallationInstalled, Status: gardencorev1beta1.ConditionTrue},
								{Type: gardencorev1beta1.ControllerInstallationHealthy, Status: gardencorev1beta1.ConditionTrue},
								{Type: gardencorev1beta1.ControllerInstallationProgressing, Status: gardencorev1beta1.ConditionFalse},
							},
						},
					},
				}
			})

			It("should fail checking all required extensions", func() {
				Expect(RequiredExtensionsReady(ctx, fakeClient, seedName, requiredExtensions)).To(MatchError("controllerregistrations.core.gardener.cloud \"foo\" not found"))
			})
		})

		Context("when required ControllerRegistration and ControllerInstallations are registered", func() {
			BeforeEach(func() {
				controllerRegistrations = []*gardencorev1beta1.ControllerRegistration{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "seedProviderExtension",
						},
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{Kind: extensionsv1alpha1.ControlPlaneResource, Type: seedProvider},
								{Kind: extensionsv1alpha1.InfrastructureResource, Type: seedProvider},
								{Kind: extensionsv1alpha1.WorkerResource, Type: seedProvider},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dnsProviderExtension",
						},
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{Kind: extensionsv1alpha1.DNSRecordResource, Type: dnsProvider},
							},
						},
					},
				}
				controllerInstallations = []*gardencorev1beta1.ControllerInstallation{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "seedProviderExtension",
						},
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							RegistrationRef: corev1.ObjectReference{
								Name: controllerRegistrations[0].Name,
							},
							SeedRef: corev1.ObjectReference{
								Name: seedName,
							},
						},
						Status: gardencorev1beta1.ControllerInstallationStatus{
							Conditions: []gardencorev1beta1.Condition{
								{Type: gardencorev1beta1.ControllerInstallationInstalled, Status: gardencorev1beta1.ConditionTrue},
								{Type: gardencorev1beta1.ControllerInstallationHealthy, Status: gardencorev1beta1.ConditionTrue},
								{Type: gardencorev1beta1.ControllerInstallationProgressing, Status: gardencorev1beta1.ConditionFalse},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dnsProviderExtension",
						},
						Spec: gardencorev1beta1.ControllerInstallationSpec{
							RegistrationRef: corev1.ObjectReference{
								Name: controllerRegistrations[1].Name,
							},
							SeedRef: corev1.ObjectReference{
								Name: seedName,
							},
						},
					},
				}
			})

			Context("when all ControllerInstallations are ready", func() {
				BeforeEach(func() {
					for _, controllerInstallation := range controllerInstallations {
						controllerInstallation.Status = gardencorev1beta1.ControllerInstallationStatus{
							Conditions: []gardencorev1beta1.Condition{
								{Type: gardencorev1beta1.ControllerInstallationInstalled, Status: gardencorev1beta1.ConditionTrue},
								{Type: gardencorev1beta1.ControllerInstallationHealthy, Status: gardencorev1beta1.ConditionTrue},
								{Type: gardencorev1beta1.ControllerInstallationProgressing, Status: gardencorev1beta1.ConditionFalse},
							},
						}
					}
				})

				It("should succeed checking all required extensions", func() {
					Expect(RequiredExtensionsReady(ctx, fakeClient, seedName, requiredExtensions)).To(Succeed())
				})
			})

			Context("when a ControllerInstallation is not ready", func() {
				BeforeEach(func() {
					controllerInstallations[0].Status = gardencorev1beta1.ControllerInstallationStatus{
						Conditions: []gardencorev1beta1.Condition{
							{Type: gardencorev1beta1.ControllerInstallationInstalled, Status: gardencorev1beta1.ConditionTrue},
							{Type: gardencorev1beta1.ControllerInstallationHealthy, Status: gardencorev1beta1.ConditionTrue},
							{Type: gardencorev1beta1.ControllerInstallationProgressing, Status: gardencorev1beta1.ConditionFalse},
						},
					}
				})

				It("should fail checking all required extensions", func() {
					Expect(RequiredExtensionsReady(ctx, fakeClient, seedName, requiredExtensions)).To(MatchError("extension controllers missing or unready: map[DNSRecord/dnsProvider:{}]"))
				})
			})
		})
	})

	DescribeTable("#GetIPStackForSeed",
		func(seed *gardencorev1beta1.Seed, expectedResult string) {
			Expect(GetIPStackForSeed(seed)).To(Equal(expectedResult))
		},

		Entry("default seed", &gardencorev1beta1.Seed{}, "ipv4"),
		Entry("ipv4 seed", &gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}}}}, "ipv4"),
		Entry("ipv6 seed", &gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}}}}, "ipv6"),
		Entry("dual-stack seed (ipv4 preferred)", &gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6}}}}, "dual-stack"),
		Entry("dual-stack seed (ipv6 preferred)", &gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Networks: gardencorev1beta1.SeedNetworks{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6, gardencorev1beta1.IPFamilyIPv4}}}}, "dual-stack"),
	)
})
