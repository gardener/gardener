// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apiserver/registry/seedmanagement/utils"
)

var _ = Describe("Utils", func() {
	Describe("#SyncSeedDNSProviderCredentials", func() {
		It("should do nothing when dns is nil", func() {
			Expect(func() {
				utils.SyncSeedDNSProviderCredentials(nil)
			}).ToNot(Panic())
		})

		It("should sync credentialsRef when only secretRef is set", func() {
			dns := &gardencorev1beta1.SeedDNSProvider{
				SecretRef: corev1.SecretReference{
					Name:      "dns-secret",
					Namespace: "garden",
				},
			}

			utils.SyncSeedDNSProviderCredentials(dns)

			Expect(dns.CredentialsRef).To(Equal(corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  "garden",
				Name:       "dns-secret",
			}))
			Expect(dns.SecretRef).To(Equal(corev1.SecretReference{
				Name:      "dns-secret",
				Namespace: "garden",
			}))
		})

		It("should sync secretRef when only credentialsRef is set with a Secret", func() {
			dns := &gardencorev1beta1.SeedDNSProvider{
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "garden",
					Name:       "dns-secret",
				},
			}

			utils.SyncSeedDNSProviderCredentials(dns)

			Expect(dns.SecretRef).To(Equal(corev1.SecretReference{
				Name:      "dns-secret",
				Namespace: "garden",
			}))
			Expect(dns.CredentialsRef).To(Equal(corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  "garden",
				Name:       "dns-secret",
			}))
		})

		It("should not sync when credentialsRef refers to WorkloadIdentity", func() {
			dns := &gardencorev1beta1.SeedDNSProvider{
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "security.gardener.cloud/v1alpha1",
					Kind:       "WorkloadIdentity",
					Namespace:  "garden",
					Name:       "wi-dns",
				},
			}

			utils.SyncSeedDNSProviderCredentials(dns)

			Expect(dns.SecretRef).To(Equal(corev1.SecretReference{}))
			Expect(dns.CredentialsRef).To(Equal(corev1.ObjectReference{
				APIVersion: "security.gardener.cloud/v1alpha1",
				Kind:       "WorkloadIdentity",
				Namespace:  "garden",
				Name:       "wi-dns",
			}))
		})

		It("should not sync when both fields are empty", func() {
			dns := &gardencorev1beta1.SeedDNSProvider{}

			utils.SyncSeedDNSProviderCredentials(dns)

			Expect(dns.SecretRef).To(Equal(corev1.SecretReference{}))
			Expect(dns.CredentialsRef).To(Equal(corev1.ObjectReference{}))
		})

		It("should not sync when both fields are set", func() {
			dns := &gardencorev1beta1.SeedDNSProvider{
				SecretRef: corev1.SecretReference{
					Name:      "secret-name",
					Namespace: "secret-namespace",
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "ref-namespace",
					Name:       "ref-name",
				},
			}

			original := dns.DeepCopy()
			utils.SyncSeedDNSProviderCredentials(dns)

			Expect(dns.SecretRef).To(Equal(original.SecretRef))
			Expect(dns.CredentialsRef).To(Equal(original.CredentialsRef))
		})

		It("should not sync when credentialsRef kind is not Secret", func() {
			dns := &gardencorev1beta1.SeedDNSProvider{
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  "garden",
					Name:       "dns-config",
				},
			}

			utils.SyncSeedDNSProviderCredentials(dns)

			// secretRef should remain empty
			Expect(dns.SecretRef).To(Equal(corev1.SecretReference{}))
		})

		It("should not sync when credentialsRef APIVersion is not v1", func() {
			dns := &gardencorev1beta1.SeedDNSProvider{
				CredentialsRef: corev1.ObjectReference{
					APIVersion: "v2",
					Kind:       "Secret",
					Namespace:  "garden",
					Name:       "dns-secret",
				},
			}

			utils.SyncSeedDNSProviderCredentials(dns)

			// secretRef should remain empty
			Expect(dns.SecretRef).To(Equal(corev1.SecretReference{}))
		})
	})
})
