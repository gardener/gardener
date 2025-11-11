// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Seed Validation Tests", func() {
	var (
		seed         *core.Seed
		seedTemplate *core.SeedTemplate
		backup       *core.Backup
	)

	BeforeEach(func() {
		region := "some-region"
		pods := "10.240.0.0/16"
		services := "10.241.0.0/16"
		nodesCIDR := "10.250.0.0/16"
		seed = &core.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "seed-1",
			},
			Spec: core.SeedSpec{
				Provider: core.SeedProvider{
					Type:   "foo",
					Region: "eu-west-1",
				},
				DNS: core.SeedDNS{
					Provider: &core.SeedDNSProvider{
						Type: "foo",
						SecretRef: corev1.SecretReference{
							Name:      "some-secret",
							Namespace: "some-namespace",
						},
					},
				},
				Ingress: &core.Ingress{
					Domain: "some-domain.example.com",
					Controller: core.IngressController{
						Kind: "nginx",
					},
				},
				Taints: []core.SeedTaint{
					{Key: "foo"},
				},
				Networks: core.SeedNetworks{
					Nodes:    &nodesCIDR,
					Pods:     "100.96.0.0/11",
					Services: "100.64.0.0/13",
					ShootDefaults: &core.ShootNetworks{
						Pods:     &pods,
						Services: &services,
					},
				},
				Backup: &core.Backup{
					Provider: "foo",
					Region:   &region,
					CredentialsRef: &corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "backup-foo",
						Namespace:  "garden",
					},
				},
				Settings: &core.SeedSettings{
					VerticalPodAutoscaler: &core.SeedSettingVerticalPodAutoscaler{},
				},
			},
		}
		seedTemplate = &core.SeedTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"foo": "bar",
				},
			},
			Spec: seed.Spec,
		}
	})

	Describe("#ValidateSeed, #ValidateSeedUpdate", func() {
		It("should not return any errors", func() {
			Expect(ValidateSeed(seed)).To(BeEmpty())
		})

		DescribeTable("Seed metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				seed.ObjectMeta = objectMeta

				Expect(ValidateSeed(seed)).To(matcher)
			},

			Entry("should forbid Seed with empty metadata",
				metav1.ObjectMeta{},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Seed with empty name",
				metav1.ObjectMeta{Name: ""},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Seed with '.' in the name (not a DNS-1123 label compliant name)",
				metav1.ObjectMeta{Name: "seed.test"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Seed with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "seed_test"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		Context("operation annotation", func() {
			It("should do nothing if the operation annotation is not set", func() {
				Expect(ValidateSeed(seed)).To(BeEmpty())
			})

			It("should return an error if the operation annotation is invalid", func() {
				metav1.SetMetaDataAnnotation(&seed.ObjectMeta, "gardener.cloud/operation", "foo-bar")

				Expect(ValidateSeed(seed)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
				}))))
			})

			DescribeTable("should return nothing if the operation annotations is valid", func(operation string) {
				metav1.SetMetaDataAnnotation(&seed.ObjectMeta, "gardener.cloud/operation", operation)

				Expect(ValidateSeed(seed)).To(BeEmpty())
			},
				Entry("reconcile", "reconcile"),
				Entry("renew-garden-access-secrets", "renew-garden-access-secrets"),
				Entry("renew-kubeconfig", "renew-kubeconfig"),
				Entry("renew-workload-identity-tokens", "renew-workload-identity-tokens"),
			)

			DescribeTable("should do nothing if a valid operation annotation is added", func(operation string) {
				newSeed := prepareSeedForUpdate(seed)
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", operation)

				Expect(ValidateSeedUpdate(newSeed, seed)).To(BeEmpty())
			},
				Entry("reconcile", "reconcile"),
				Entry("renew-garden-access-secrets", "renew-garden-access-secrets"),
				Entry("renew-kubeconfig", "renew-kubeconfig"),
				Entry("renew-workload-identity-tokens", "renew-workload-identity-tokens"),
			)

			DescribeTable("should do nothing if a valid operation annotation is removed", func(operation string) {
				metav1.SetMetaDataAnnotation(&seed.ObjectMeta, "gardener.cloud/operation", operation)
				newSeed := prepareSeedForUpdate(seed)
				delete(newSeed.Annotations, "gardener.cloud/operation")

				Expect(ValidateSeedUpdate(newSeed, seed)).To(BeEmpty())
			},
				Entry("reconcile", "reconcile"),
				Entry("renew-garden-access-secrets", "renew-garden-access-secrets"),
				Entry("renew-kubeconfig", "renew-kubeconfig"),
				Entry("renew-workload-identity-tokens", "renew-workload-identity-tokens"),
			)

			DescribeTable("should do nothing if a valid operation annotation does not change during an update", func(operation string) {
				metav1.SetMetaDataAnnotation(&seed.ObjectMeta, "gardener.cloud/operation", operation)
				newSeed := prepareSeedForUpdate(seed)

				Expect(ValidateSeedUpdate(newSeed, seed)).To(BeEmpty())
			},
				Entry("reconcile", "reconcile"),
				Entry("renew-garden-access-secrets", "renew-garden-access-secrets"),
				Entry("renew-kubeconfig", "renew-kubeconfig"),
				Entry("renew-workload-identity-tokens", "renew-workload-identity-tokens"),
			)

			It("should return an error if a valid operation should be overwritten with a different valid operation", func() {
				metav1.SetMetaDataAnnotation(&seed.ObjectMeta, "gardener.cloud/operation", "renew-garden-access-secrets")
				newSeed := prepareSeedForUpdate(seed)
				metav1.SetMetaDataAnnotation(&newSeed.ObjectMeta, "gardener.cloud/operation", "renew-kubeconfig")

				Expect(ValidateSeedUpdate(newSeed, seed)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": Equal("must not overwrite operation \"renew-garden-access-secrets\" with \"renew-kubeconfig\""),
					}))))
			})
		})

		It("should forbid Seed specification with empty or invalid keys", func() {
			invalidCIDR := "invalid-cidr"
			seed.Spec.Provider = core.SeedProvider{
				Zones: []string{"a", "a"},
			}
			seed.Spec.Networks = core.SeedNetworks{
				Nodes:    &invalidCIDR,
				Pods:     "300.300.300.300/300",
				Services: invalidCIDR,
				ShootDefaults: &core.ShootNetworks{
					Pods:     &invalidCIDR,
					Services: &invalidCIDR,
				},
			}
			seed.Spec.Taints = []core.SeedTaint{
				{Key: "foo"},
				{Key: "foo"},
				{Key: ""},
			}
			seed.Spec.Backup.CredentialsRef = nil
			seed.Spec.Backup.Provider = ""
			minSize := resource.MustParse("-1")
			seed.Spec.Volume = &core.SeedVolume{
				MinimumSize: &minSize,
				Providers: []core.SeedVolumeProvider{
					{
						Purpose: "",
						Name:    "",
					},
					{
						Purpose: "duplicate",
						Name:    "value1",
					},
					{
						Purpose: "duplicate",
						Name:    "value2",
					},
				},
			}

			errorList := ValidateSeed(seed)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.backup.provider"),
					"Detail": Equal(`must provide a backup cloud provider name`),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.backup.credentialsRef"),
					"Detail": Equal(`must be set to refer a Secret or WorkloadIdentity`),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.type"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.provider.zones[1]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.taints[1]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.taints[2].key"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.nodes"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.pods"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.services"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.shootDefaults.pods"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.shootDefaults.services"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.volume.minimumSize"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.volume.providers[0].purpose"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.volume.providers[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.volume.providers[2].purpose"),
				})),
			))
		})

		Context("internal DNS", func() {
			It("should require valid fields if internal DNS is set", func() {
				seed.Spec.DNS.Internal = &core.SeedDNSProviderConfig{}
				errorList := ValidateSeed(seed)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.internal.type"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.internal.domain"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.internal.credentialsRef"),
						"Detail": Equal("credentialsRef must reference a Secret"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.internal.credentialsRef.apiVersion"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.internal.credentialsRef.kind"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.internal.credentialsRef.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.internal.credentialsRef.name"),
						"Detail": ContainSubstring("a lowercase RFC 1123 subdomain"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.internal.credentialsRef.namespace"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.internal.credentialsRef.namespace"),
						"Detail": ContainSubstring("a lowercase RFC 1123 subdomain"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  Equal("spec.dns.internal.credentialsRef"),
						"Detail": ContainSubstring("supported values: \"/v1, Kind=Secret\""),
					})),
				))
			})

			It("should return error if the internal DNS configures a malformed domain", func() {
				seed.Spec.DNS.Internal = &core.SeedDNSProviderConfig{
					Type:   "foo",
					Domain: "invalid_dns1123-subdomain",
				}
				errorList := ValidateSeed(seed)
				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.dns.internal.domain"),
					"Detail": ContainSubstring("a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters"),
				}))))
			})

			It("should not return an error if the dns is configured correctly", func() {
				seed.Spec.DNS.Internal = &core.SeedDNSProviderConfig{
					Type:   "foo",
					Domain: "foo.example.com",
					Zone:   ptr.To("zone-1"),
					CredentialsRef: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "internal-domain",
						Namespace:  "garden",
					},
				}
				errorList := ValidateSeed(seed)
				Expect(errorList).To(BeEmpty())
			})

			It("should return an error if old seed has dns configured, but new one does not", func() {
				seed.Spec.DNS.Internal = &core.SeedDNSProviderConfig{
					Type:   "foo",
					Domain: "foo.example.com",
					Zone:   ptr.To("zone-1"),
					CredentialsRef: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "internal-domain",
						Namespace:  "garden",
					},
				}
				newSeed := seed.DeepCopy()
				newSeed.Spec.DNS.Internal = nil

				errorList := ValidateSeedUpdate(newSeed, seed)
				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.dns.internal"),
					"Detail": ContainSubstring("removing internal DNS configuration is not allowed"),
				}))))
			})
		})

		Context("defaults DNS", func() {
			It("should require valid fields if defaults DNS is set", func() {
				seed.Spec.DNS.Defaults = []core.SeedDNSProviderConfig{{}}
				errorList := ValidateSeed(seed)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[0].type"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[0].domain"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.defaults[0].credentialsRef"),
						"Detail": Equal("credentialsRef must reference a Secret"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[0].credentialsRef.apiVersion"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[0].credentialsRef.kind"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[0].credentialsRef.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.defaults[0].credentialsRef.name"),
						"Detail": ContainSubstring("a lowercase RFC 1123 subdomain"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[0].credentialsRef.namespace"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.defaults[0].credentialsRef.namespace"),
						"Detail": ContainSubstring("a lowercase RFC 1123 subdomain"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  Equal("spec.dns.defaults[0].credentialsRef"),
						"Detail": ContainSubstring("supported values: \"/v1, Kind=Secret\""),
					})),
				))
			})

			It("should return error if the defaults DNS configures a malformed domain", func() {
				seed.Spec.DNS.Defaults = []core.SeedDNSProviderConfig{{
					Type:   "foo",
					Domain: "invalid_dns1123-subdomain",
				}}
				errorList := ValidateSeed(seed)
				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.dns.defaults[0].domain"),
					"Detail": ContainSubstring("a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters"),
				}))))
			})

			It("should succeed if the defaults DNS is configured correctly", func() {
				seed.Spec.DNS.Defaults = []core.SeedDNSProviderConfig{{
					Type:   "foo",
					Domain: "foo.example.com",
					Zone:   ptr.To("zone-1"),
					CredentialsRef: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "Secret",
						Name:       "default-domain",
						Namespace:  "garden",
					},
				}}
				errorList := ValidateSeed(seed)
				Expect(errorList).To(BeEmpty())
			})

			It("should validate multiple defaults DNS configurations", func() {
				seed.Spec.DNS.Defaults = []core.SeedDNSProviderConfig{
					{
						Type:   "foo",
						Domain: "foo.example.com",
						Zone:   ptr.To("zone-1"),
						CredentialsRef: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       "default-domain-1",
							Namespace:  "garden",
						},
					},
					{
						Type:   "bar",
						Domain: "bar.example.com",
						Zone:   ptr.To("zone-2"),
						CredentialsRef: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       "default-domain-2",
							Namespace:  "garden",
						},
					},
				}
				errorList := ValidateSeed(seed)
				Expect(errorList).To(BeEmpty())
			})

			It("should return error for duplicate domains in defaults DNS configurations", func() {
				seed.Spec.DNS.Defaults = []core.SeedDNSProviderConfig{
					{
						Type:   "foo",
						Domain: "example.com",
						Zone:   ptr.To("zone-1"),
						CredentialsRef: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       "default-domain-1",
							Namespace:  "garden",
						},
					},
					{
						Type:   "bar",
						Domain: "example.com",
						Zone:   ptr.To("zone-2"),
						CredentialsRef: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       "default-domain-2",
							Namespace:  "garden",
						},
					},
				}
				errorList := ValidateSeed(seed)
				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.dns.defaults[1].domain"),
				}))))
			})

			It("should return errors for invalid entries in defaults DNS configurations", func() {
				seed.Spec.DNS.Defaults = []core.SeedDNSProviderConfig{
					{
						Type:   "foo",
						Domain: "foo.example.com",
						Zone:   ptr.To("zone-1"),
						CredentialsRef: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       "default-domain-1",
							Namespace:  "garden",
						},
					},
					{},
				}
				errorList := ValidateSeed(seed)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{ // required domain
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[1].domain"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{ // required type
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[1].type"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{ // invalid credentialsRef (must reference a Secret)
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.defaults[1].credentialsRef"),
						"Detail": Equal("credentialsRef must reference a Secret"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{ // required apiVersion
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[1].credentialsRef.apiVersion"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{ // required kind
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[1].credentialsRef.kind"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{ // required name
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[1].credentialsRef.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{ // invalid name format
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.defaults[1].credentialsRef.name"),
						"Detail": ContainSubstring("a lowercase RFC 1123 subdomain"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{ // required namespace
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dns.defaults[1].credentialsRef.namespace"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{ // invalid namespace format
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.dns.defaults[1].credentialsRef.namespace"),
						"Detail": ContainSubstring("a lowercase RFC 1123 subdomain"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{ // not supported credentialsRef
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  Equal("spec.dns.defaults[1].credentialsRef"),
						"Detail": ContainSubstring("supported values: \"/v1, Kind=Secret\""),
					})),
				))
			})

			It("should return an error if old seed has dns configured, but new one does not", func() {
				seed.Spec.DNS.Defaults = []core.SeedDNSProviderConfig{
					{
						Type:   "foo",
						Domain: "foo.example.com",
						Zone:   ptr.To("zone-1"),
						CredentialsRef: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       "internal-domain",
							Namespace:  "garden",
						},
					},
				}
				newSeed := seed.DeepCopy()
				newSeed.Spec.DNS.Defaults = nil

				errorList := ValidateSeedUpdate(newSeed, seed)
				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.dns.defaults"),
					"Detail": ContainSubstring("removing defaults DNS configuration is not allowed"),
				}))))
			})
		})

		Context("backup credentialsRef and secretRef", func() {
			It("should require credentialsRef to be set", func() {
				seed.Spec.Backup.CredentialsRef = nil

				Expect(ValidateSeed(seed)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.backup.credentialsRef"),
						"Detail": Equal("must be set to refer a Secret or WorkloadIdentity"),
					})),
				))
			})

			It("should allow credentialsRef to refer a WorkloadIdentity", func() {
				seed.Spec.Backup.CredentialsRef = &corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Namespace: "garden", Name: "backup"}

				Expect(ValidateSeed(seed)).To(BeEmpty())
			})

			It("should allow credentialsRef to refer a Secret", func() {
				seed.Spec.Backup.CredentialsRef = &corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "garden", Name: "backup"}

				Expect(ValidateSeed(seed)).To(BeEmpty())
			})

			It("should forbid invalid values objectReference fields", func() {
				seed.Spec.Backup.CredentialsRef = &corev1.ObjectReference{APIVersion: "", Kind: "", Namespace: "", Name: ""}

				Expect(ValidateSeed(seed)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.backup.credentialsRef.apiVersion"),
						"Detail": Equal("must provide an apiVersion"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.backup.credentialsRef.kind"),
						"Detail": Equal("must provide a kind"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.backup.credentialsRef.name"),
						"Detail": Equal("must provide a name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.backup.credentialsRef.name"),
						"Detail": ContainSubstring("a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.backup.credentialsRef.namespace"),
						"Detail": Equal("must provide a namespace"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.backup.credentialsRef.namespace"),
						"Detail": ContainSubstring("a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  Equal("spec.backup.credentialsRef"),
						"Detail": Equal(`supported values: "/v1, Kind=Secret", "security.gardener.cloud/v1alpha1, Kind=WorkloadIdentity"`),
					})),
				))
			})
		})

		Context("networks", func() {
			It("should forbid specifying unsupported IP family", func() {
				seed.Spec.Networks.IPFamilies = []core.IPFamily{"IPv5"}

				errorList := ValidateSeed(seed)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.networks.ipFamilies[0]"),
				}))))
			})

			Context("IPv4", func() {
				It("should allow valid networking configuration", func() {
					seed.Spec.Networks.Nodes = ptr.To("10.1.0.0/16")
					seed.Spec.Networks.Pods = "10.2.0.0/16"
					seed.Spec.Networks.Services = "10.3.0.0/16"
					seed.Spec.Networks.ShootDefaults.Pods = ptr.To("10.4.0.0/16")
					seed.Spec.Networks.ShootDefaults.Services = ptr.To("10.5.0.0/16")

					errorList := ValidateSeed(seed)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid invalid network CIDRs", func() {
					invalidCIDR := "invalid-cidr"

					seed.Spec.Networks.Nodes = &invalidCIDR
					seed.Spec.Networks.Pods = invalidCIDR
					seed.Spec.Networks.Services = invalidCIDR
					seed.Spec.Networks.ShootDefaults.Pods = &invalidCIDR
					seed.Spec.Networks.ShootDefaults.Services = &invalidCIDR

					errorList := ValidateSeed(seed)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.nodes"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.pods"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.services"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.services"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.pods"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}))
				})

				It("should forbid IPv6 CIDRs with IPv4 IP family", func() {
					seed.Spec.Networks.Nodes = ptr.To("2001:db8:11::/48")
					seed.Spec.Networks.Pods = "2001:db8:12::/48"
					seed.Spec.Networks.Services = "2001:db8:13::/48"
					seed.Spec.Networks.ShootDefaults.Pods = ptr.To("2001:db8:1::/48")
					seed.Spec.Networks.ShootDefaults.Services = ptr.To("2001:db8:3::/48")

					errorList := ValidateSeed(seed)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.nodes"),
						"Detail": ContainSubstring("must be a valid IPv4 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.pods"),
						"Detail": ContainSubstring("must be a valid IPv4 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.services"),
						"Detail": ContainSubstring("must be a valid IPv4 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.services"),
						"Detail": ContainSubstring("must be a valid IPv4 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.pods"),
						"Detail": ContainSubstring("must be a valid IPv4 address"),
					}))
				})

				It("should forbid Seed with overlapping networks but allow overlap with shoot defaults", func() {
					shootDefaultPodCIDR := "10.0.1.128/28"     // 10.0.1.128 -> 10.0.1.143
					shootDefaultServiceCIDR := "10.0.1.144/30" // 10.0.1.144 -> 10.0.1.147

					nodesCIDR := "10.0.0.0/8" // 10.0.0.0 -> 10.255.255.255
					// Pods CIDR overlaps with Nodes network
					// Services CIDR overlaps with Nodes and Pods
					// Shoot default pod CIDR overlaps with services
					// Shoot default pod CIDR overlaps with shoot default pod CIDR
					seed.Spec.Networks = core.SeedNetworks{
						Nodes:    &nodesCIDR,     // 10.0.0.0 -> 10.255.255.25
						Pods:     "10.0.1.0/24",  // 10.0.1.0 -> 10.0.1.255
						Services: "10.0.1.64/26", // 10.0.1.64 -> 10.0.1.127
						ShootDefaults: &core.ShootNetworks{
							Pods:     &shootDefaultPodCIDR,
							Services: &shootDefaultServiceCIDR,
						},
					}

					errorList := ValidateSeed(seed)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":     Equal(field.ErrorTypeInvalid),
						"Field":    Equal("spec.networks.nodes"),
						"BadValue": Equal("10.0.0.0/8"),
						"Detail":   Equal("must not overlap with \"spec.networks.pods\" (\"10.0.1.0/24\")"),
					}, Fields{
						"Type":     Equal(field.ErrorTypeInvalid),
						"Field":    Equal("spec.networks.nodes"),
						"BadValue": Equal("10.0.0.0/8"),
						"Detail":   Equal("must not overlap with \"spec.networks.services\" (\"10.0.1.64/26\")"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.services"),
						"Detail": Equal(`must not overlap with "spec.networks.pods" ("10.0.1.0/24")`),
					}))

					Expect(errorList).To(Not(ContainElement(HaveFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.pods"),
						"Detail": Equal(`must not overlap with "spec.networks.nodes" ("10.0.0.0/8")`),
					}))))

					Expect(errorList).To(Not(ContainElement(HaveFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.services"),
						"Detail": Equal(`must not overlap with "spec.networks.nodes" ("10.0.0.0/8")`),
					}))))

					Expect(errorList).To(Not(ContainElement(HaveFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.pods"),
						"Detail": Equal(`must not overlap with "spec.networks.pods" ("10.0.1.0/24")`),
					}))))

					Expect(errorList).To(Not(ContainElement(HaveFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.services"),
						"Detail": Equal(`must not overlap with "spec.networks.pods" ("10.0.1.0/24")`),
					}))))
				})

				It("should forbid Seed with overlap to reserved ranges", func() {
					// Service CIDR overlaps with reserved range
					seed.Spec.Networks.Pods = "240.0.0.0/16" // 240.0.0.0 -> 240.0.255.255
					seed.Spec.Networks.Services = "243.1.0.0/16"
					seed.Spec.Networks.Nodes = ptr.To("242.2.0.0/16")
					seed.Spec.Networks.ShootDefaults.Pods = ptr.To("244.3.0.0/16")
					seed.Spec.Networks.ShootDefaults.Services = ptr.To("244.4.0.0/16")

					Expect(ValidateSeed(seed)).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.pods"),
						"Detail": Equal(`pod network intersects with reserved kube-apiserver mapping range (240.0.0.0/8)`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.services"),
						"Detail": Equal(`service network intersects with reserved shoot service network mapping range (243.0.0.0/8)`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.nodes"),
						"Detail": Equal(`node network intersects with reserved shoot node network mapping range (242.0.0.0/8)`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.pods"),
						"Detail": Equal(`pod network intersects with reserved shoot pod network mapping range (244.0.0.0/8)`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.services"),
						"Detail": Equal(`service network intersects with reserved shoot pod network mapping range (244.0.0.0/8)`),
					}))
				})

				It("should forbid Seed with too large service range", func() {
					// Service CIDR too large
					seed.Spec.Networks.Services = "90.0.0.0/7" // 90.0.0.0 -> 91.255.255.255

					Expect(ValidateSeed(seed)).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.services"),
						"Detail": Equal(`cannot be larger than /8`),
					}))
				})
			})

			Context("IPv6", func() {
				BeforeEach(func() {
					seed.Spec.Networks.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}
				})

				It("should allow valid networking configuration", func() {
					seed.Spec.Networks.Nodes = ptr.To("2001:db8:11::/48")
					seed.Spec.Networks.Pods = "2001:db8:12::/48"
					seed.Spec.Networks.Services = "2001:db8:13::/48"
					seed.Spec.Networks.ShootDefaults.Pods = ptr.To("2001:db8:1::/48")
					seed.Spec.Networks.ShootDefaults.Services = ptr.To("2001:db8:3::/48")

					Expect(ValidateSeed(seed)).To(BeEmpty())
				})

				It("should forbid invalid network CIDRs", func() {
					invalidCIDR := "invalid-cidr"

					seed.Spec.Networks.Nodes = &invalidCIDR
					seed.Spec.Networks.Pods = invalidCIDR
					seed.Spec.Networks.Services = invalidCIDR
					seed.Spec.Networks.ShootDefaults.Pods = &invalidCIDR
					seed.Spec.Networks.ShootDefaults.Services = &invalidCIDR

					Expect(ValidateSeed(seed)).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.nodes"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.pods"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.services"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.services"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.pods"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}))
				})

				It("should forbid Seed with overlapping networks", func() {
					seed.Spec.Networks.Nodes = ptr.To("2001:db8:11::/48")
					seed.Spec.Networks.Pods = "2001:db8:11:1::/49"
					seed.Spec.Networks.Services = "2001:db8:11:2::/49"
					seed.Spec.Networks.ShootDefaults.Pods = ptr.To("2001:db8:11:a::/64")
					seed.Spec.Networks.ShootDefaults.Services = ptr.To("2001:db8:11:b::/64")

					errorList := ValidateSeed(seed)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":     Equal(field.ErrorTypeInvalid),
						"Field":    Equal("spec.networks.nodes"),
						"BadValue": Equal("2001:db8:11::/48"),
						"Detail":   Equal(`must not overlap with "spec.networks.pods" ("2001:db8:11:1::/49")`),
					}, Fields{
						"Type":     Equal(field.ErrorTypeInvalid),
						"Field":    Equal("spec.networks.nodes"),
						"BadValue": Equal("2001:db8:11::/48"),
						"Detail":   Equal(`must not overlap with "spec.networks.services" ("2001:db8:11:2::/49")`),
					}, Fields{
						"Type":     Equal(field.ErrorTypeInvalid),
						"Field":    Equal("spec.networks.services"),
						"BadValue": Equal("2001:db8:11:2::/49"),
						"Detail":   Equal(`must not overlap with "spec.networks.pods" ("2001:db8:11:1::/49")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.pods"),
						"Detail": Equal(`must not overlap with "spec.networks.pods" ("2001:db8:11:1::/49")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.pods"),
						"Detail": Equal(`must not overlap with "spec.networks.nodes" ("2001:db8:11::/48")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.pods"),
						"Detail": Equal(`must not overlap with "spec.networks.services" ("2001:db8:11:2::/49")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.services"),
						"Detail": Equal(`must not overlap with "spec.networks.nodes" ("2001:db8:11::/48")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.services"),
						"Detail": Equal(`must not overlap with "spec.networks.pods" ("2001:db8:11:1::/49")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.services"),
						"Detail": Equal(`must not overlap with "spec.networks.services" ("2001:db8:11:2::/49")`),
					}))
				})

				It("should forbid IPv4 CIDRs with IPv6 IP family", func() {
					seed.Spec.Networks.Nodes = ptr.To("10.1.0.0/16")
					seed.Spec.Networks.Pods = "10.2.0.0/16"
					seed.Spec.Networks.Services = "10.3.0.0/16"
					seed.Spec.Networks.ShootDefaults.Pods = ptr.To("10.4.0.0/16")
					seed.Spec.Networks.ShootDefaults.Services = ptr.To("10.5.0.0/16")

					Expect(ValidateSeed(seed)).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.nodes"),
						"Detail": ContainSubstring("must be a valid IPv6 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.pods"),
						"Detail": ContainSubstring("must be a valid IPv6 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.services"),
						"Detail": ContainSubstring("must be a valid IPv6 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.services"),
						"Detail": ContainSubstring("must be a valid IPv6 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networks.shootDefaults.pods"),
						"Detail": ContainSubstring("must be a valid IPv6 address"),
					}))
				})

				It("should allow Seed with very large service range because the range limit only applies to ipv4", func() {
					seed.Spec.Networks.Nodes = ptr.To("2001:db8:11::/48")
					seed.Spec.Networks.Pods = "2001:db8:12::/48"
					seed.Spec.Networks.Services = "3001::/7" // larger than /8 ipv4 limit
					seed.Spec.Networks.ShootDefaults.Pods = ptr.To("2001:db8:1::/48")
					seed.Spec.Networks.ShootDefaults.Services = ptr.To("2001:db8:3::/48")

					Expect(ValidateSeed(seed)).To(BeEmpty())
				})
			})
		})

		Context("networks update", func() {
			var oldSeed *core.Seed

			BeforeEach(func() {
				oldSeed = seed.DeepCopy()
			})

			It("should fail updating immutable fields", func() {
				oldSeed.Spec.Networks.IPFamilies = []core.IPFamily{core.IPFamilyIPv4}

				newSeed := prepareSeedForUpdate(oldSeed)
				newSeed.Spec.Networks.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}

				Expect(ValidateSeedUpdate(newSeed, oldSeed)).To(ContainElement(HaveFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.networks.ipFamilies"),
					"Detail": ContainSubstring(`field is immutable`),
				})))
			})

			It("should allow adding a nodes CIDR", func() {
				oldSeed.Spec.Networks.Nodes = nil

				nodesCIDR := "10.1.0.0/16"
				newSeed := prepareSeedForUpdate(oldSeed)
				newSeed.Spec.Networks.Nodes = &nodesCIDR

				Expect(ValidateSeedUpdate(newSeed, oldSeed)).To(BeEmpty())
			})

			It("should forbid removing the nodes CIDR", func() {
				newSeed := prepareSeedForUpdate(oldSeed)
				newSeed.Spec.Networks.Nodes = nil

				Expect(ValidateSeedUpdate(newSeed, oldSeed)).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.networks.nodes"),
					"Detail": Equal(`field is immutable`),
				}))
			})

			It("should forbid changing the nodes CIDR", func() {
				newSeed := prepareSeedForUpdate(oldSeed)

				differentNodesCIDR := "12.1.0.0/16"
				newSeed.Spec.Networks.Nodes = &differentNodesCIDR

				Expect(ValidateSeedUpdate(newSeed, oldSeed)).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.networks.nodes"),
					"Detail": Equal(`field is immutable`),
				}))
			})
		})

		Context("settings", func() {
			Context("excessCapacityReservation", func() {
				It("should allow valid excessCapacityReservation configurations", func() {
					seed.Spec.Settings = &core.SeedSettings{
						ExcessCapacityReservation: &core.SeedSettingExcessCapacityReservation{
							Configs: []core.SeedSettingExcessCapacityReservationConfig{
								{
									Resources: corev1.ResourceList{
										"cpu":    resource.MustParse("2"),
										"memory": resource.MustParse("10Gi"),
									},
								},
								{
									Resources: corev1.ResourceList{
										"cpu":    resource.MustParse("5"),
										"memory": resource.MustParse("100Gi"),
									},
									NodeSelector: map[string]string{"foo": "bar"},
									Tolerations: []corev1.Toleration{
										{
											Key:      "foo",
											Operator: "Equal",
											Value:    "bar",
											Effect:   "NoExecute",
										},
										{
											Key:      "bar",
											Operator: "Equal",
											Value:    "foo",
											Effect:   "NoSchedule",
										},
									},
								},
							},
						},
					}

					Expect(ValidateSeed(seed)).To(BeEmpty())
				})

				It("should not allow configs with no resources", func() {
					seed.Spec.Settings = &core.SeedSettings{
						ExcessCapacityReservation: &core.SeedSettingExcessCapacityReservation{
							Configs: []core.SeedSettingExcessCapacityReservationConfig{{NodeSelector: map[string]string{"foo": "bar"}}},
						},
					}

					Expect(ValidateSeed(seed)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("spec.settings.excessCapacityReservation.configs[0].resources"),
						})),
					))
				})

				It("should not allow configs invalid resources", func() {
					seed.Spec.Settings = &core.SeedSettings{
						ExcessCapacityReservation: &core.SeedSettingExcessCapacityReservation{
							Configs: []core.SeedSettingExcessCapacityReservationConfig{{Resources: corev1.ResourceList{"cpu": resource.MustParse("-1")}}},
						},
					}

					Expect(ValidateSeed(seed)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.settings.excessCapacityReservation.configs[0].resources.cpu"),
						})),
					))
				})

				It("should not allow configs invalid tolerations", func() {
					seed.Spec.Settings = &core.SeedSettings{
						ExcessCapacityReservation: &core.SeedSettingExcessCapacityReservation{
							Configs: []core.SeedSettingExcessCapacityReservationConfig{
								{
									Resources:   corev1.ResourceList{"cpu": resource.MustParse("1")},
									Tolerations: []corev1.Toleration{{Key: "foo", Value: "bar", Operator: "Equal", Effect: "foobar"}},
								},
								{
									Resources:   corev1.ResourceList{"cpu": resource.MustParse("1")},
									Tolerations: []corev1.Toleration{{Key: "foo", Operator: "Exists", Value: "bar", Effect: "NoSchedule"}},
								},
							},
						},
					}

					Expect(ValidateSeed(seed)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotSupported),
							"Field": Equal("spec.settings.excessCapacityReservation.configs[0].tolerations[0].effect"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.settings.excessCapacityReservation.configs[1].tolerations[0].operator"),
						})),
					))
				})
			})

			Context("loadbalancer", func() {
				It("should allow valid load balancer service annotations", func() {
					seed.Spec.Settings = &core.SeedSettings{
						LoadBalancerServices: &core.SeedSettingLoadBalancerServices{
							Annotations: map[string]string{
								"simple":                          "bar",
								"now-with-dashes":                 "bar",
								"1-starts-with-num":               "bar",
								"1234":                            "bar",
								"simple/simple":                   "bar",
								"now-with-dashes/simple":          "bar",
								"now-with-dashes/now-with-dashes": "bar",
								"now.with.dots/simple":            "bar",
								"now-with.dashes-and.dots/simple": "bar",
								"1-num.2-num/3-num":               "bar",
								"1234/5678":                       "bar",
								"1.2.3.4/5678":                    "bar",
								"UpperCase123":                    "bar",
							},
						},
					}

					Expect(ValidateSeed(seed)).To(BeEmpty())
				})

				It("should prevent invalid load balancer service annotations", func() {
					seed.Spec.Settings = &core.SeedSettings{
						LoadBalancerServices: &core.SeedSettingLoadBalancerServices{
							Annotations: map[string]string{
								"nospecialchars^=@":      "bar",
								"cantendwithadash-":      "bar",
								"only/one/slash":         "bar",
								strings.Repeat("a", 254): "bar",
							},
						},
					}

					Expect(ValidateSeed(seed)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.settings.loadBalancerServices.annotations"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.settings.loadBalancerServices.annotations"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.settings.loadBalancerServices.annotations"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.settings.loadBalancerServices.annotations"),
						})),
					))
				})

				It("should allow valid load balancer service traffic policy", func() {
					for _, p := range []string{"Cluster", "Local"} {
						policy := corev1.ServiceExternalTrafficPolicy(p)
						seed.Spec.Settings = &core.SeedSettings{
							LoadBalancerServices: &core.SeedSettingLoadBalancerServices{
								ExternalTrafficPolicy: &policy,
							},
						}

						Expect(ValidateSeed(seed)).To(BeEmpty(), fmt.Sprintf("seed validation should succeed with load balancer service traffic policy '%s' and have no errors", p))
					}
				})

				It("should prevent invalid load balancer service traffic policy", func() {
					policy := corev1.ServiceExternalTrafficPolicy("foobar")
					seed.Spec.Settings = &core.SeedSettings{
						LoadBalancerServices: &core.SeedSettingLoadBalancerServices{
							ExternalTrafficPolicy: &policy,
						},
					}

					Expect(ValidateSeed(seed)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotSupported),
							"Field": Equal("spec.settings.loadBalancerServices.externalTrafficPolicy"),
						})),
					))
				})

				It("should allow valid zonal load balancer service annotations and traffic policy", func() {
					for _, p := range []string{"Cluster", "Local"} {
						policy := corev1.ServiceExternalTrafficPolicy(p)
						zoneName := "a"
						seed.Spec.Provider.Zones = []string{zoneName, "b"}
						seed.Spec.Settings = &core.SeedSettings{
							LoadBalancerServices: &core.SeedSettingLoadBalancerServices{
								Zones: []core.SeedSettingLoadBalancerServicesZones{{
									Name: zoneName,
									Annotations: map[string]string{
										"simple":                          "bar",
										"now-with-dashes":                 "bar",
										"1-starts-with-num":               "bar",
										"1234":                            "bar",
										"simple/simple":                   "bar",
										"now-with-dashes/simple":          "bar",
										"now-with-dashes/now-with-dashes": "bar",
										"now.with.dots/simple":            "bar",
										"now-with.dashes-and.dots/simple": "bar",
										"1-num.2-num/3-num":               "bar",
										"1234/5678":                       "bar",
										"1.2.3.4/5678":                    "bar",
										"UpperCase123":                    "bar",
									},
									ExternalTrafficPolicy: &policy,
								}},
							},
						}

						Expect(ValidateSeed(seed)).To(BeEmpty(), fmt.Sprintf("seed validation should succeed with valid zonal load balancer traffic policy '%s' and have no errors", p))
					}
				})

				It("should prevent invalid zonal load balancer service annotations, traffic policy and duplicate zones", func() {
					policy := corev1.ServiceExternalTrafficPolicy("foobar")
					zoneName := "a"
					incorrectZoneName := "b"
					seed.Spec.Provider.Zones = []string{zoneName}
					seed.Spec.Settings = &core.SeedSettings{
						LoadBalancerServices: &core.SeedSettingLoadBalancerServices{
							Zones: []core.SeedSettingLoadBalancerServicesZones{
								{
									Name: incorrectZoneName,
									Annotations: map[string]string{
										"nospecialchars^=@":      "bar",
										"cantendwithadash-":      "bar",
										"only/one/slash":         "bar",
										strings.Repeat("a", 254): "bar",
									},
									ExternalTrafficPolicy: &policy,
								},
								{
									Name: incorrectZoneName,
								},
							},
						},
					}

					Expect(ValidateSeed(seed)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("spec.settings.loadBalancerServices.zones"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.settings.loadBalancerServices.zones[0].annotations"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.settings.loadBalancerServices.zones[0].annotations"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.settings.loadBalancerServices.zones[0].annotations"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.settings.loadBalancerServices.zones[0].annotations"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotSupported),
							"Field": Equal("spec.settings.loadBalancerServices.zones[0].externalTrafficPolicy"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotFound),
							"Field": Equal("spec.settings.loadBalancerServices.zones[0].name"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotFound),
							"Field": Equal("spec.settings.loadBalancerServices.zones[1].name"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal("spec.settings.loadBalancerServices.zones[1].name"),
						})),
					))
				})

				Describe("LoadBalancer class", func() {
					BeforeEach(func() {
						seed.Spec.Settings = &core.SeedSettings{
							LoadBalancerServices: &core.SeedSettingLoadBalancerServices{},
						}
					})

					It("should allow omitting the class", func() {
						seed.Spec.Settings.LoadBalancerServices.Class = nil

						Expect(ValidateSeed(seed)).To(BeEmpty())
					})

					It("should allow specifying a non-empty class", func() {
						seed.Spec.Settings.LoadBalancerServices.Class = ptr.To("non-default")

						Expect(ValidateSeed(seed)).To(BeEmpty())
					})

					It("should allow specifying a qualified class", func() {
						seed.Spec.Settings.LoadBalancerServices.Class = ptr.To("stackit.cloud/yawol")

						Expect(ValidateSeed(seed)).To(BeEmpty())
					})

					It("should deny specifying an empty class", func() {
						seed.Spec.Settings.LoadBalancerServices.Class = ptr.To("")

						Expect(ValidateSeed(seed)).To(ContainElement(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.settings.loadBalancerServices.class"),
								"Origin": Equal("format=qualified-name"),
							})),
						))
					})

					It("should deny specifying a class that Kubernetes does not accept", func() {
						seed.Spec.Settings.LoadBalancerServices.Class = ptr.To(".invalid-")

						Expect(ValidateSeed(seed)).To(ContainElement(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.settings.loadBalancerServices.class"),
								"Origin": Equal("format=qualified-name"),
							})),
						))
					})
				})
			})

			Context("vertical pod autoscaler", func() {
				It("should not allow maxAllowed with unsupported resource", func() {
					seed.Spec.Settings = &core.SeedSettings{
						VerticalPodAutoscaler: &core.SeedSettingVerticalPodAutoscaler{
							MaxAllowed: corev1.ResourceList{
								"storage": {},
							},
						},
					}

					errorList := ValidateSeed(seed)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeNotSupported),
							"Field":    Equal("spec.settings.verticalPodAutoscaler.maxAllowed.storage"),
							"BadValue": Equal(corev1.ResourceName("storage")),
							"Detail":   Equal(`supported values: "cpu", "memory"`),
						})),
					))
				})

				It("should not allow maxAllowed with invalid resources", func() {
					seed.Spec.Settings = &core.SeedSettings{
						VerticalPodAutoscaler: &core.SeedSettingVerticalPodAutoscaler{
							MaxAllowed: corev1.ResourceList{
								"cpu":    resource.MustParse("-100m"),
								"memory": resource.MustParse("-100Mi"),
							},
						},
					}

					errorList := ValidateSeed(seed)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("spec.settings.verticalPodAutoscaler.maxAllowed.cpu"),
							"BadValue": Equal("-100m"),
							"Detail":   Equal("must be greater than or equal to 0"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("spec.settings.verticalPodAutoscaler.maxAllowed.memory"),
							"BadValue": Equal("-100Mi"),
							"Detail":   Equal("must be greater than or equal to 0"),
						})),
					))
				})

				It("should allow maxAllowed with valid resources", func() {
					seed.Spec.Settings = &core.SeedSettings{
						VerticalPodAutoscaler: &core.SeedSettingVerticalPodAutoscaler{
							MaxAllowed: corev1.ResourceList{
								"cpu":    resource.MustParse("8"),
								"memory": resource.MustParse("32Gi"),
							},
						},
					}

					Expect(ValidateSeed(seed)).To(BeEmpty())
				})
			})

			Context("topology-aware routing", func() {
				It("should prevent enabling topology-aware routing on single-zone Seed cluster", func() {
					seed.Spec.Provider.Zones = []string{"a"}
					seed.Spec.Settings = &core.SeedSettings{
						TopologyAwareRouting: &core.SeedSettingTopologyAwareRouting{
							Enabled: true,
						},
					}

					Expect(ValidateSeed(seed)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.settings.topologyAwareRouting.enabled"),
							"Detail": Equal("topology-aware routing can only be enabled on multi-zone Seed clusters (with at least two zones in spec.provider.zones)"),
						})),
					))
				})
			})

			It("should allow enabling topology-aware routing on multi-zone Seed cluster", func() {
				seed.Spec.Provider.Zones = []string{"a", "b"}
				seed.Spec.Settings = &core.SeedSettings{
					TopologyAwareRouting: &core.SeedSettingTopologyAwareRouting{
						Enabled: true,
					},
				}

				Expect(ValidateSeed(seed)).To(BeEmpty())
			})

			Context("verticalPodAutoscaler", func() {
				It("should not allow unknown feature gates", func() {
					seed.Spec.Settings.VerticalPodAutoscaler.FeatureGates = map[string]bool{
						"Foo": true,
					}
					Expect(ValidateSeed(seed)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.settings.verticalPodAutoscaler.featureGates.Foo"),
							"Detail": Equal("unknown feature gate"),
						})),
					))
				})

				It("should allow supported feature gates", func() {
					seed.Spec.Settings.VerticalPodAutoscaler.FeatureGates = map[string]bool{
						"InPlaceOrRecreate": true,
					}
					Expect(ValidateSeed(seed)).To(BeEmpty())
				})
			})
		})

		It("should fail updating immutable fields", func() {
			nodesCIDR := "10.1.0.0/16"

			newSeed := prepareSeedForUpdate(seed)
			newSeed.Spec.Networks = core.SeedNetworks{
				Nodes:    &nodesCIDR,
				Pods:     "10.2.0.0/16",
				Services: "10.3.1.64/26",
			}
			otherRegion := "other-region"
			newSeed.Spec.Backup.Provider = "other-provider"
			newSeed.Spec.Backup.Region = &otherRegion

			Expect(ValidateSeedUpdate(newSeed, seed)).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.pods"),
				"Detail": Equal(`field is immutable`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.services"),
				"Detail": Equal(`field is immutable`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.networks.nodes"),
				"Detail": Equal(`field is immutable`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.backup.region"),
				"Detail": Equal(`field is immutable`),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.backup.provider"),
				"Detail": Equal(`field is immutable`),
			}))
		})

		Context("#validateSeedBackupUpdate", func() {
			It("should allow adding backup profile", func() {
				seed.Spec.Backup = nil
				newSeed := prepareSeedForUpdate(seed)
				newSeed.Spec.Backup = backup

				Expect(ValidateSeedUpdate(newSeed, seed)).To(BeEmpty())
			})

			It("should forbid removing backup profile", func() {
				newSeed := prepareSeedForUpdate(seed)
				newSeed.Spec.Backup = nil

				Expect(ValidateSeedUpdate(newSeed, seed)).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.backup"),
					"Detail": Equal(`field is immutable`),
				}))
			})
		})

		Context("ingress config", func() {
			BeforeEach(func() {
				seed.Spec.Ingress = &core.Ingress{
					Domain: "foo.bar.com",
					Controller: core.IngressController{
						Kind: "nginx",
					},
				}
				seed.Spec.DNS.Provider = &core.SeedDNSProvider{
					Type: "some-type",
					SecretRef: corev1.SecretReference{
						Name:      "foo",
						Namespace: "bar",
					},
				}
			})

			It("should fail if immutable spec.ingress.domain gets changed", func() {
				newSeed := prepareSeedForUpdate(seed)
				newSeed.Spec.Ingress.Domain = "other-domain"

				Expect(ValidateSeedUpdate(newSeed, seed)).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.ingress.domain"),
					"Detail": Equal(`field is immutable`),
				}))
			})

			It("should succeed if ingress config is correct", func() {
				Expect(ValidateSeed(seed)).To(BeEmpty())
			})

			It("ingress is nil", func() {
				seed.Spec.Ingress = nil

				Expect(ValidateSeed(seed)).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.ingress"),
					"Detail": ContainSubstring("cannot be empty"),
				}))
			})

			It("requires to specify a DNS provider if ingress is specified", func() {
				seed.Spec.DNS.Provider = nil

				Expect(ValidateSeed(seed)).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.dns.provider"),
				}))
			})

			It("should fail if kind is different to nginx", func() {
				seed.Spec.Ingress.Controller.Kind = "new-kind"

				Expect(ValidateSeed(seed)).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.ingress.controller.kind"),
				}))
			})

			It("should fail if the ingress domain is invalid", func() {
				seed.Spec.Ingress.Domain = "invalid_dns1123-subdomain"

				Expect(ValidateSeed(seed)).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.ingress.domain"),
				}))
			})

			It("should fail if the ingress domain is empty", func() {
				seed.Spec.Ingress.Domain = ""

				Expect(ValidateSeed(seed)).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.ingress.domain"),
					"Detail": ContainSubstring("cannot be empty"),
				}))
			})

			It("should fail if DNS provider config is invalid", func() {
				seed.Spec.DNS.Provider = &core.SeedDNSProvider{}

				Expect(ValidateSeed(seed)).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.dns.provider.type"),
				}, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.dns.provider.secretRef.name"),
				}, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.dns.provider.secretRef.namespace"),
				}))
			})
		})

		Context("Extensions validation", func() {
			It("should forbid passing an extension w/o type information", func() {
				extension := core.Extension{}
				seed.Spec.Extensions = append(seed.Spec.Extensions, extension)

				Expect(ValidateSeed(seed)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.extensions[0].type"),
					}))))
			})

			It("should allow passing an extension w/ type information", func() {
				extension := core.Extension{
					Type: "arbitrary",
				}
				seed.Spec.Extensions = append(seed.Spec.Extensions, extension)

				Expect(ValidateSeed(seed)).To(BeEmpty())
			})

			It("should forbid passing an extension of same type more than once", func() {
				extension := core.Extension{
					Type: "arbitrary",
				}
				seed.Spec.Extensions = append(seed.Spec.Extensions, extension, extension)

				Expect(ValidateSeed(seed)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.extensions[1].type"),
					}))))
			})

			It("should allow passing more than one extension of different type", func() {
				extension := core.Extension{
					Type: "arbitrary",
				}
				seed.Spec.Extensions = append(seed.Spec.Extensions, extension, extension)
				seed.Spec.Extensions[1].Type = "arbitrary-2"

				Expect(ValidateSeed(seed)).To(BeEmpty())
			})
		})

		Context("Resources validation", func() {
			It("should forbid resources w/o names or w/ invalid references", func() {
				ref := core.NamedResourceReference{}
				seed.Spec.Resources = append(seed.Spec.Resources, ref)

				Expect(ValidateSeed(seed)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.resources[0].name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.resources[0].resourceRef.kind"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.resources[0].resourceRef.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.resources[0].resourceRef.apiVersion"),
					})),
				))
			})

			It("should forbid resources of kind other than Secret/ConfigMap", func() {
				ref := core.NamedResourceReference{
					Name: "test",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind:       "ServiceAccount",
						Name:       "test-sa",
						APIVersion: "v1",
					},
				}
				seed.Spec.Resources = append(seed.Spec.Resources, ref)

				Expect(ValidateSeed(seed)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeNotSupported),
						"Field":    Equal("spec.resources[0].resourceRef.kind"),
						"BadValue": Equal("ServiceAccount"),
					})),
				))
			})

			It("should forbid resources with non-unique names", func() {
				ref := core.NamedResourceReference{
					Name: "test",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind:       "Secret",
						Name:       "test-secret",
						APIVersion: "v1",
					},
				}
				seed.Spec.Resources = append(seed.Spec.Resources, ref, ref)

				Expect(ValidateSeed(seed)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.resources[1].name"),
					})),
				))
			})

			It("should allow resources w/ names and valid references", func() {
				ref := core.NamedResourceReference{
					Name: "test",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind:       "Secret",
						Name:       "test-secret",
						APIVersion: "v1",
					},
				}

				ref2 := core.NamedResourceReference{
					Name: "test-cm",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind:       "ConfigMap",
						Name:       "test-cm",
						APIVersion: "v1",
					},
				}

				seed.Spec.Resources = append(seed.Spec.Resources, ref, ref2)

				Expect(ValidateSeed(seed)).To(BeEmpty())
			})
		})

	})

	Describe("#ValidateSeedStatusUpdate", func() {
		Context("validate .status.clusterIdentity updates", func() {
			newSeed := &core.Seed{
				Status: core.SeedStatus{
					ClusterIdentity: ptr.To("newClusterIdentity"),
				},
			}

			It("should fail to update seed status cluster identity if it already exists", func() {
				oldSeed := &core.Seed{Status: core.SeedStatus{
					ClusterIdentity: ptr.To("clusterIdentityExists"),
				}}

				Expect(ValidateSeedStatusUpdate(newSeed, oldSeed)).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.clusterIdentity"),
					"Detail": Equal(`field is immutable`),
				}))
			})

			It("should not fail to update seed status cluster identity if it is missing", func() {
				oldSeed := &core.Seed{}

				Expect(ValidateSeedStatusUpdate(newSeed, oldSeed)).To(BeEmpty())
			})
		})
	})

	Describe("#ValidateSeedTemplate", func() {
		It("should allow valid resources", func() {
			errorList := ValidateSeedTemplate(seedTemplate, nil)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid invalid metadata or spec fields", func() {
			seedTemplate.Labels = map[string]string{"foo!": "bar"}
			seedTemplate.Annotations = map[string]string{"foo!": "bar"}
			seedTemplate.Spec.Networks.Nodes = ptr.To("")

			Expect(ValidateSeedTemplate(seedTemplate, nil)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.labels"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.annotations"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networks.nodes"),
				})),
			))
		})
	})

	Describe("#ValidateSeedTemplateUpdate", func() {
		It("should allow valid updates", func() {
			Expect(ValidateSeedTemplateUpdate(seedTemplate, seedTemplate, nil)).To(BeEmpty())
		})

		It("should forbid changes to immutable fields in spec", func() {
			newSeedTemplate := *seedTemplate
			newSeedTemplate.Spec.Networks.Pods = "100.97.0.0/11"

			Expect(ValidateSeedTemplateUpdate(&newSeedTemplate, seedTemplate, nil)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.networks.pods"),
					"Detail": Equal("field is immutable"),
				})),
			))
		})
	})
})

func prepareSeedForUpdate(seed *core.Seed) *core.Seed {
	s := seed.DeepCopy()
	s.ResourceVersion = "1"
	return s
}
