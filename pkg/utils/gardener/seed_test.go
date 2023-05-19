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

package gardener_test

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
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

	Describe("#ComputeNginxIngressClassForSeed", func() {
		var (
			seed              *gardencorev1beta1.Seed
			kubernetesVersion *string
		)

		BeforeEach(func() {
			seed = &gardencorev1beta1.Seed{}
			kubernetesVersion = pointer.String("1.20.3")
		})

		It("should return an error because kubernetes version is nil", func() {
			class, err := ComputeNginxIngressClassForSeed(seed, nil)
			Expect(class).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("kubernetes version is missing for seed")))
		})

		It("should return an error because kubernetes version cannot be parsed", func() {
			class, err := ComputeNginxIngressClassForSeed(seed, pointer.String("foo"))
			Expect(class).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("Invalid Semantic Version")))
		})

		Context("when seed does not want managed ingress", func() {
			It("should return 'nginx'", func() {
				class, err := ComputeNginxIngressClassForSeed(seed, kubernetesVersion)
				Expect(class).To(Equal("nginx"))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when seed wants managed ingress", func() {
			BeforeEach(func() {
				seed.Spec.DNS.Provider = &gardencorev1beta1.SeedDNSProvider{}
				seed.Spec.Ingress = &gardencorev1beta1.Ingress{Controller: gardencorev1beta1.IngressController{Kind: v1beta1constants.IngressKindNginx}}
			})

			It("should return 'nginx-gardener' when kubernetes version < 1.22", func() {
				class, err := ComputeNginxIngressClassForSeed(seed, kubernetesVersion)
				Expect(class).To(Equal("nginx-gardener"))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return 'nginx-ingress-gardener' when kubernetes version >= 1.22", func() {
				kubernetesVersion = pointer.String("1.22.0")

				class, err := ComputeNginxIngressClassForSeed(seed, kubernetesVersion)
				Expect(class).To(Equal("nginx-ingress-gardener"))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("#GetWilcardCertificate", func() {
		var (
			ctx        = context.TODO()
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
			Expect(err).To(MatchError(ContainSubstring("misconfigured seed cluster: not possible to provide more than one secret with annotation")))
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

	Describe("#SeedIsGarden", func() {
		var (
			ctx        context.Context
			mockReader *mockclient.MockReader
			ctrl       *gomock.Controller
		)

		BeforeEach(func() {
			ctx = context.Background()
			ctrl = gomock.NewController(GinkgoT())
			mockReader = mockclient.NewMockReader(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return that seed is a garden cluster", func() {
			mockReader.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1)).DoAndReturn(
				func(_ context.Context, list *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					list.Items = []metav1.PartialObjectMetadata{{}}
					return nil
				})
			Expect(SeedIsGarden(ctx, mockReader)).To(BeTrue())
		})

		It("should return that seed is a not a garden cluster because no garden object found", func() {
			mockReader.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1))
			Expect(SeedIsGarden(ctx, mockReader)).To(BeFalse())
		})

		It("should return that seed is a not a garden cluster because of a no match error", func() {
			mockReader.EXPECT().List(ctx, gomock.AssignableToTypeOf(&metav1.PartialObjectMetadataList{}), client.Limit(1)).DoAndReturn(
				func(_ context.Context, list *metav1.PartialObjectMetadataList, _ ...client.ListOption) error {
					return &meta.NoResourceMatchError{}
				})
			Expect(SeedIsGarden(ctx, mockReader)).To(BeFalse())
		})
	})

	Describe("#ComputeRequiredExtensionsForSeed", func() {
		var seed *gardencorev1beta1.Seed

		BeforeEach(func() {
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{Type: "providerA"},
				},
			}
		})

		It("should return the required types for seed", func() {
			seed.Spec.DNS = gardencorev1beta1.SeedDNS{
				Provider: &gardencorev1beta1.SeedDNSProvider{Type: "providerB"},
			}

			Expect(ComputeRequiredExtensionsForSeed(seed).UnsortedList()).To(ConsistOf(
				"DNSRecord/providerB",
				"ControlPlane/providerA",
				"Infrastructure/providerA",
				"Worker/providerA",
			))
		})

		It("should return the required types for seed w/o DNS provider", func() {
			Expect(ComputeRequiredExtensionsForSeed(seed).UnsortedList()).To(ConsistOf(
				"ControlPlane/providerA",
				"Infrastructure/providerA",
				"Worker/providerA",
			))
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
})
