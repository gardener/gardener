// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/validation"
	"github.com/gardener/gardener/pkg/features"
)

var _ = Describe("Validation Tests", func() {
	Describe("#ValidateGarden", func() {
		var garden *operatorv1alpha1.Garden

		BeforeEach(func() {
			garden = &operatorv1alpha1.Garden{
				ObjectMeta: metav1.ObjectMeta{
					Name: "garden",
				},
				Spec: operatorv1alpha1.GardenSpec{
					DNS: &operatorv1alpha1.DNSManagement{
						Providers: []operatorv1alpha1.DNSProvider{
							{
								Name: "primary",
								Type: "test",
								SecretRef: corev1.LocalObjectReference{
									Name: "test-secret",
								},
							},
						},
					},
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Ingress: operatorv1alpha1.Ingress{
							Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.bar.com", Provider: ptr.To("primary")}},
						},
						Networking: operatorv1alpha1.RuntimeNetworking{
							Pods:     "10.1.0.0/16",
							Services: "10.2.0.0/16",
						},
					},
					VirtualCluster: operatorv1alpha1.VirtualCluster{
						DNS: operatorv1alpha1.DNS{
							Domains: []operatorv1alpha1.DNSDomain{{Name: "foo.bar.com", Provider: ptr.To("primary")}},
						},
						Kubernetes: operatorv1alpha1.Kubernetes{
							Version: "1.26.3",
							KubeAPIServer: &operatorv1alpha1.KubeAPIServerConfig{
								KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{},
							},
						},
						Networking: operatorv1alpha1.Networking{
							Services: "10.4.0.0/16",
						},
					},
				},
			}
		})

		Context("operation annotation", func() {
			It("should do nothing if the operation annotation is not set", func() {
				Expect(ValidateGarden(garden)).To(BeEmpty())
			})

			DescribeTable("should not allow starting/completing rotation when garden has deletion timestamp",
				func(operation string) {
					garden.DeletionTimestamp = &metav1.Time{}
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", operation)

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
					}))))
				},

				Entry("start all rotations", "rotate-credentials-start"),
				Entry("complete all rotations", "rotate-credentials-complete"),
				Entry("start CA rotation", "rotate-ca-start"),
				Entry("complete CA rotation", "rotate-ca-complete"),
				Entry("start ServiceAccount key rotation", "rotate-serviceaccount-key-start"),
				Entry("complete ServiceAccount key rotation", "rotate-serviceaccount-key-complete"),
				Entry("start ETCD encryption key rotation", "rotate-etcd-encryption-key-start"),
				Entry("complete ETCD encryption key rotation", "rotate-etcd-encryption-key-complete"),
				Entry("start Observability key rotation", "rotate-observability-credentials"),
				Entry("start WorkloadIdentity key rotation", "rotate-workload-identity-key-start"),
				Entry("complete WorkloadIdentity key rotation", "rotate-workload-identity-key-complete"),
			)

			DescribeTable("starting rotation of all credentials",
				func(allowed bool, status operatorv1alpha1.GardenStatus, kubeAPIEncryptionConfig, gardenerEncryptionConfig *gardencorev1beta1.EncryptionConfig, extraMatchers ...gomegatypes.GomegaMatcher) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-start")
					garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = &operatorv1alpha1.KubeAPIServerConfig{
						KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{
							EncryptionConfig: kubeAPIEncryptionConfig,
						},
					}
					garden.Spec.VirtualCluster.Gardener.APIServer = &operatorv1alpha1.GardenerAPIServerConfig{
						EncryptionConfig: gardenerEncryptionConfig,
					}
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(And(matcher, SatisfyAll(extraMatchers...)))
				},

				Entry("ca rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}, nil, nil),
				Entry("sa rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}, nil, nil),
				Entry("etcd key rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}, nil, nil),
				Entry("workload identity key rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}, nil, nil),
				Entry("ca rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}, nil, nil),
				Entry("sa rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}, nil, nil),
				Entry("etcd key rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}, nil, nil),
				Entry("workload identity key rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}, nil, nil),
				Entry("ca rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}, nil, nil),
				Entry("sa rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}, nil, nil),
				Entry("etcd key rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}, nil, nil),
				Entry("workload identity key rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}, nil, nil),
				Entry("ca rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}, nil, nil),
				Entry("sa rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}, nil, nil),
				Entry("etcd key rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}, nil, nil),
				Entry("workload identity key rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}, nil, nil),
				Entry("when spec encrypted resources and status encrypted resources are not equal", false,
					operatorv1alpha1.GardenStatus{
						EncryptedResources: []string{"configmaps", "projects.core.gardener.cloud"},
					},
					&gardencorev1beta1.EncryptionConfig{Resources: []string{"deployments.apps"}},
					&gardencorev1beta1.EncryptionConfig{Resources: []string{"shoots.core.gardener.cloud"}},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": Equal("cannot start rotation of all credentials because a previous encryption configuration change is currently being rolled out"),
					}))),
				),
				Entry("when spec encrypted resources and status encrypted resources are equal", true,
					operatorv1alpha1.GardenStatus{
						EncryptedResources: []string{"configmaps", "daemonsets.apps", "projects.core.gardener.cloud", "shoots.core.gardener.cloud"},
					},
					&gardencorev1beta1.EncryptionConfig{Resources: []string{"daemonsets.apps", "configmaps"}},
					&gardencorev1beta1.EncryptionConfig{Resources: []string{"shoots.core.gardener.cloud", "projects.core.gardener.cloud"}},
				),
			)

			DescribeTable("completing rotation of all credentials",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-complete")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("ca rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("workload identity key rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("all rotation phases are prepared", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("workload identity key rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("workload identity key rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("starting CA rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-ca-start")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("ca rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("ca rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("completing CA rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-ca-complete")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("ca rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("ca rotation phase is prepared", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							CertificateAuthorities: &gardencorev1beta1.CARotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("starting service account key rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-serviceaccount-key-start")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},
				Entry("rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("completing service account key rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-serviceaccount-key-complete")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("starting ETCD encryption key rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus, kubeAPIEncryptionConfig, gardenerEncryptionConfig *gardencorev1beta1.EncryptionConfig, extraMatchers ...gomegatypes.GomegaMatcher) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-etcd-encryption-key-start")

					garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = &operatorv1alpha1.KubeAPIServerConfig{
						KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{
							EncryptionConfig: kubeAPIEncryptionConfig,
						},
					}
					garden.Spec.VirtualCluster.Gardener.APIServer = &operatorv1alpha1.GardenerAPIServerConfig{
						EncryptionConfig: gardenerEncryptionConfig,
					}

					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(And(matcher, SatisfyAll(extraMatchers...)))
				},

				Entry("rotation phase is prepare", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}, nil, nil),
				Entry("rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}, nil, nil),
				Entry("rotation phase is complete", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}, nil, nil),
				Entry("rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}, nil, nil),
				Entry("when spec encrypted resources and status encrypted resources are not equal", false,
					operatorv1alpha1.GardenStatus{
						EncryptedResources: []string{"configmaps", "projects.core.gardener.cloud"},
					},
					&gardencorev1beta1.EncryptionConfig{Resources: []string{"deployments.apps"}},
					&gardencorev1beta1.EncryptionConfig{Resources: []string{"shoots.core.gardener.cloud"}},
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": Equal("cannot start ETCD encryption key rotation because a previous encryption configuration change is currently being rolled out"),
					}))),
				),
				Entry("when spec encrypted resources and status encrypted resources are equal", true,
					operatorv1alpha1.GardenStatus{
						EncryptedResources: []string{"configmaps", "daemonsets.apps", "projects.core.gardener.cloud", "shoots.core.gardener.cloud"},
					},
					&gardencorev1beta1.EncryptionConfig{Resources: []string{"daemonsets.apps", "configmaps"}},
					&gardencorev1beta1.EncryptionConfig{Resources: []string{"shoots.core.gardener.cloud", "projects.core.gardener.cloud"}},
				),
			)

			DescribeTable("completing ETCD encryption key rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-etcd-encryption-key-complete")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("rotation phase is prepare", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is complete", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("starting workload identity key rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-workload-identity-key-start")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},
				Entry("rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("completing workload identity key rotation",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-workload-identity-key-complete")
					garden.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateGarden(garden)).To(matcher)
				},

				Entry("rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							WorkloadIdentityKey: &operatorv1alpha1.WorkloadIdentityKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
			)

		})

		Context("extensions", func() {
			BeforeEach(func() {
				garden.Spec.Extensions = []operatorv1alpha1.GardenExtension{
					{
						Type:           "extension-1",
						ProviderConfig: &runtime.RawExtension{Raw: []byte(`{}`)},
					},
					{
						Type: "extension-2",
					},
				}
			})

			It("should succeed if valid extensions are set", func() {
				Expect(ValidateGarden(garden)).To(BeEmpty())
			})

			It("should fail if the same extension type is set multiple times", func() {
				garden.Spec.Extensions = append(garden.Spec.Extensions, garden.Spec.Extensions[0])

				Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.extensions[2].type"),
				}))))
			})
		})

		Context("runtime cluster", func() {
			Context("networking", func() {
				It("should complain when pod network of runtime cluster intersects with service network of runtime cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Pods = garden.Spec.RuntimeCluster.Networking.Services

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.runtimeCluster.networking.services"),
					}))))
				})

				It("should complain when node network of runtime cluster intersects with pod network of runtime cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Nodes = &garden.Spec.RuntimeCluster.Networking.Pods

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.runtimeCluster.networking.nodes"),
					}))))
				})

				It("should complain when node network of runtime cluster intersects with service network of runtime cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Nodes = &garden.Spec.RuntimeCluster.Networking.Services

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.runtimeCluster.networking.nodes"),
					}))))
				})
			})

			Context("topology-aware routing field", func() {
				It("should prevent enabling topology-aware routing on single-zone cluster", func() {
					garden.Spec.RuntimeCluster.Provider.Zones = []string{"a"}
					garden.Spec.RuntimeCluster.Settings = &operatorv1alpha1.Settings{
						TopologyAwareRouting: &operatorv1alpha1.SettingTopologyAwareRouting{
							Enabled: true,
						},
					}
					garden.Spec.VirtualCluster.ControlPlane = &operatorv1alpha1.ControlPlane{HighAvailability: &operatorv1alpha1.HighAvailability{}}

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.runtimeCluster.settings.topologyAwareRouting.enabled"),
							"Detail": Equal("topology-aware routing can only be enabled on multi-zone garden runtime cluster (with at least two zones in spec.provider.zones)"),
						})),
					))
				})

				It("should prevent enabling topology-aware routing when control-plane is not HA", func() {
					garden.Spec.RuntimeCluster.Provider.Zones = []string{"a", "b", "c"}
					garden.Spec.RuntimeCluster.Settings = &operatorv1alpha1.Settings{
						TopologyAwareRouting: &operatorv1alpha1.SettingTopologyAwareRouting{
							Enabled: true,
						},
					}
					garden.Spec.VirtualCluster.ControlPlane = nil

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.runtimeCluster.settings.topologyAwareRouting.enabled"),
							"Detail": Equal("topology-aware routing can only be enabled when virtual cluster's high-availability is enabled"),
						})),
					))
				})

				It("should allow enabling topology-aware routing on multi-zone cluster with HA control-plane", func() {
					garden.Spec.RuntimeCluster.Provider.Zones = []string{"a", "b", "c"}
					garden.Spec.RuntimeCluster.Settings = &operatorv1alpha1.Settings{
						TopologyAwareRouting: &operatorv1alpha1.SettingTopologyAwareRouting{
							Enabled: true,
						},
					}
					garden.Spec.VirtualCluster.ControlPlane = &operatorv1alpha1.ControlPlane{HighAvailability: &operatorv1alpha1.HighAvailability{}}

					Expect(ValidateGarden(garden)).To(BeEmpty())
				})
			})

			Context("Ingress", func() {
				It("should complain about invalid ingress domain names", func() {
					garden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{{Name: ",,,", Provider: ptr.To("primary")}}

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.runtimeCluster.ingress.domains[0].name"),
						})),
					))
				})

				It("should complain about duplicate ingress domain names in 'domains'", func() {
					garden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{
						{Name: "example.com", Provider: ptr.To("primary")},
						{Name: "foo.bar", Provider: ptr.To("primary")},
						{Name: "example.com", Provider: ptr.To("primary")},
					}

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal("spec.runtimeCluster.ingress.domains[2].name"),
						})),
					))
				})

				It("should accept explicit domain provider", func() {
					garden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com", Provider: ptr.To("primary")}}

					Expect(ValidateGarden(garden)).To(BeEmpty())
				})

				It("should complain about missing domain provider", func() {
					garden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}}

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("spec.runtimeCluster.ingress.domains[0].provider"),
						})),
					))
				})

				It("should accept domain without provider if .spec.dns is unset", func() {
					garden.Spec.DNS = nil
					garden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}}

					Expect(ValidateGarden(garden)).To(BeEmpty())
				})

				It("should complain about unspecified provider", func() {
					garden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com", Provider: ptr.To("foo")}}

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.runtimeCluster.ingress.domains[0].provider"),
						})),
					))
				})
			})
		})

		Context("virtual cluster", func() {
			Context("DNS", func() {
				It("should complain about invalid domain name in 'domain'", func() {
					garden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{{Name: ",,,", Provider: ptr.To("primary")}}

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.virtualCluster.dns.domains[0].name"),
						})),
					))
				})

				It("should complain about duplicate domain names in 'domains'", func() {
					garden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{
						{Name: "example.com", Provider: ptr.To("primary")},
						{Name: "foo.bar", Provider: ptr.To("primary")},
						{Name: "example.com", Provider: ptr.To("primary")},
					}
					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal("spec.virtualCluster.dns.domains[2].name"),
						})),
					))
				})

				It("should accept explicit domain provider", func() {
					garden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com", Provider: ptr.To("primary")}}

					Expect(ValidateGarden(garden)).To(BeEmpty())
				})

				It("should complain about missing domain provider", func() {
					garden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}}

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("spec.virtualCluster.dns.domains[0].provider"),
						})),
					))
				})

				It("should accept domain without provider if .spec.dns is unset", func() {
					garden.Spec.DNS = nil
					garden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}}

					Expect(ValidateGarden(garden)).To(BeEmpty())
				})

				It("should complain about invalid domain provider", func() {
					garden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com", Provider: ptr.To("foo")}}

					Expect(ValidateGarden(garden)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.virtualCluster.dns.domains[0].provider"),
						})),
					))
				})
			})

			Context("ETCD", func() {
				It("should complain if both bucket name and provider config are set", func() {
					garden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{
						Main: &operatorv1alpha1.ETCDMain{
							Backup: &operatorv1alpha1.Backup{
								BucketName: ptr.To("foo"),
								Provider:   "foo-provider",
								ProviderConfig: &runtime.RawExtension{
									Raw: []byte(`{"foo":"bar"}`),
								},
							},
						},
					}

					Expect(ValidateGarden(garden)).To(ContainElements(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("spec.virtualCluster.etcd.main.backup.providerConfig"),
						})),
					))
				})
			})

			Context("Networking", func() {
				It("should complain about an invalid service CIDR", func() {
					garden.Spec.VirtualCluster.Networking.Services = "not-parseable-cidr"

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.networking.services"),
					}))))
				})

				It("should complain when pod network of runtime cluster intersects with service network of virtual cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Pods = garden.Spec.VirtualCluster.Networking.Services

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.networking.services"),
					}))))
				})

				It("should complain when service network of runtime cluster intersects with service network of virtual cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Services = garden.Spec.VirtualCluster.Networking.Services

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.networking.services"),
					}))))
				})

				It("should complain when node network of runtime cluster intersects with service network of virtual cluster", func() {
					garden.Spec.RuntimeCluster.Networking.Nodes = &garden.Spec.VirtualCluster.Networking.Services

					Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.networking.services"),
					}))))
				})
			})

			Context("Gardener", func() {
				Context("APIServer", func() {
					BeforeEach(func() {
						garden.Spec.VirtualCluster.Gardener.APIServer = &operatorv1alpha1.GardenerAPIServerConfig{}
					})

					Context("Feature gates", func() {
						It("should complain when non-existing feature gates were configured", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.FeatureGates = map[string]bool{"Foo": true}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.featureGates.Foo"),
							}))))
						})

						It("should complain when invalid feature gates were configured", func() {
							features.AllFeatureGates["Foo"] = featuregate.FeatureSpec{LockToDefault: true, Default: false}
							DeferCleanup(func() {
								delete(features.AllFeatureGates, "Foo")
							})

							garden.Spec.VirtualCluster.Gardener.APIServer = &operatorv1alpha1.GardenerAPIServerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: map[string]bool{"Foo": true},
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.featureGates.Foo"),
							}))))
						})
					})

					Context("Admission plugins", func() {
						It("should allow not specifying admission plugins", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.AdmissionPlugins = nil

							Expect(ValidateGarden(garden)).To(BeEmpty())
						})

						It("should forbid specifying admission plugins without a name", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{Name: ""}}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.admissionPlugins[0].name"),
							}))))
						})

						It("should forbid specifying non-existing admission plugin", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{{Name: "Foo"}}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeNotSupported),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.admissionPlugins[0].name"),
							}))))
						})
					})

					Context("AuditConfig", func() {
						It("should allow nil AuditConfig", func() {
							Expect(ValidateGarden(garden)).To(BeEmpty())
						})

						It("should forbid empty name", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.AuditConfig = &gardencorev1beta1.AuditConfig{
								AuditPolicy: &gardencorev1beta1.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{},
								},
							}

							Expect(ValidateGarden(garden)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.auditConfig.auditPolicy.configMapRef.name"),
							}))))
						})
					})

					Context("EncryptionConfig", func() {
						It("should deny specifying duplicate resources", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
								Resources: []string{"shoots.core.gardener.cloud", "shoots.core.gardener.cloud"},
							}

							Expect(ValidateGarden(garden)).To(ConsistOf(
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":     Equal(field.ErrorTypeDuplicate),
									"Field":    Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources[1]"),
									"BadValue": Equal("shoots.core.gardener.cloud"),
								})),
							))
						})

						It("should deny specifying resources encrypted by default", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
								Resources: []string{
									"controllerdeployments.core.gardener.cloud",
									"controllerregistrations.core.gardener.cloud",
									"internalsecrets.core.gardener.cloud",
									"shootstates.core.gardener.cloud",
									"shoots.core.gardener.cloud",
									"secretbindings.core.gardener.cloud",
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElements(
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":   Equal(field.ErrorTypeForbidden),
									"Field":  Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources[0]"),
									"Detail": Equal("\"controllerdeployments.core.gardener.cloud\" are always encrypted"),
								})),
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":   Equal(field.ErrorTypeForbidden),
									"Field":  Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources[1]"),
									"Detail": Equal("\"controllerregistrations.core.gardener.cloud\" are always encrypted"),
								})),
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":   Equal(field.ErrorTypeForbidden),
									"Field":  Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources[2]"),
									"Detail": Equal("\"internalsecrets.core.gardener.cloud\" are always encrypted"),
								})),
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":   Equal(field.ErrorTypeForbidden),
									"Field":  Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources[3]"),
									"Detail": Equal("\"shootstates.core.gardener.cloud\" are always encrypted"),
								})),
							))
						})

						It("should deny specifying resources which are not served by gardener-apiserver", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
								Resources: []string{"shoots.core.gardener.cloud",
									"bastions.operations.gardener.cloud",
									"ingresses.networking.io",
									"foo.gardener.cloud",
									"configmaps",
								},
							}

							Expect(ValidateGarden(garden)).To(ConsistOf(
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":     Equal(field.ErrorTypeInvalid),
									"Field":    Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources[2]"),
									"BadValue": Equal("ingresses.networking.io"),
									"Detail":   Equal("should be a resource served by gardener-apiserver. ie; should have any of the suffixes {core,operations,settings,seedmanagement}.gardener.cloud"),
								})),
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":     Equal(field.ErrorTypeInvalid),
									"Field":    Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources[3]"),
									"BadValue": Equal("foo.gardener.cloud"),
									"Detail":   Equal("should be a resource served by gardener-apiserver. ie; should have any of the suffixes {core,operations,settings,seedmanagement}.gardener.cloud"),
								})),
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":     Equal(field.ErrorTypeInvalid),
									"Field":    Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources[4]"),
									"BadValue": Equal("configmaps"),
									"Detail":   Equal("should be a resource served by gardener-apiserver. ie; should have any of the suffixes {core,operations,settings,seedmanagement}.gardener.cloud"),
								})),
							))
						})

						It("should deny specifying wildcard resources", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
								Resources: []string{"*.core.gardener.cloud", "*.operations.gardener.cloud"},
							}

							Expect(ValidateGarden(garden)).To(ConsistOf(
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":   Equal(field.ErrorTypeInvalid),
									"Field":  Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources[0]"),
									"Detail": Equal("wildcards are not supported"),
								})),
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":   Equal(field.ErrorTypeInvalid),
									"Field":  Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources[1]"),
									"Detail": Equal("wildcards are not supported"),
								})),
							))
						})
					})

					Context("Watch cache sizes", func() {
						var negativeSize int32 = -1

						DescribeTable("watch cache size validation",
							func(sizes *gardencorev1beta1.WatchCacheSizes, matcher gomegatypes.GomegaMatcher) {
								garden.Spec.VirtualCluster.Gardener.APIServer.WatchCacheSizes = sizes
								Expect(ValidateGarden(garden)).To(matcher)
							},

							Entry("valid (unset)", nil, BeEmpty()),
							Entry("valid (fields unset)", &gardencorev1beta1.WatchCacheSizes{}, BeEmpty()),
							Entry("valid (default=0)", &gardencorev1beta1.WatchCacheSizes{
								Default: ptr.To[int32](0),
							}, BeEmpty()),
							Entry("valid (default>0)", &gardencorev1beta1.WatchCacheSizes{
								Default: ptr.To[int32](42),
							}, BeEmpty()),
							Entry("invalid (default<0)", &gardencorev1beta1.WatchCacheSizes{
								Default: ptr.To(negativeSize),
							}, ConsistOf(
								field.Invalid(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.watchCacheSizes.default"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
							)),

							// APIGroup unset (core group)
							Entry("valid (core/secrets=0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									Resource:  "secrets",
									CacheSize: 0,
								}},
							}, BeEmpty()),
							Entry("valid (core/secrets=>0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									Resource:  "secrets",
									CacheSize: 42,
								}},
							}, BeEmpty()),
							Entry("invalid (core/secrets=<0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									Resource:  "secrets",
									CacheSize: negativeSize,
								}},
							}, ConsistOf(
								field.Invalid(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.watchCacheSizes.resources[0].size"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
							)),
							Entry("invalid (core/resource empty)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									Resource:  "",
									CacheSize: 42,
								}},
							}, ConsistOf(
								field.Required(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.watchCacheSizes.resources[0].resource"), "must not be empty"),
							)),

							// APIGroup set
							Entry("valid (apps/deployments=0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									APIGroup:  ptr.To("apps"),
									Resource:  "deployments",
									CacheSize: 0,
								}},
							}, BeEmpty()),
							Entry("valid (apps/deployments=>0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									APIGroup:  ptr.To("apps"),
									Resource:  "deployments",
									CacheSize: 42,
								}},
							}, BeEmpty()),
							Entry("invalid (apps/deployments=<0)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									APIGroup:  ptr.To("apps"),
									Resource:  "deployments",
									CacheSize: negativeSize,
								}},
							}, ConsistOf(
								field.Invalid(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.watchCacheSizes.resources[0].size"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
							)),
							Entry("invalid (apps/resource empty)", &gardencorev1beta1.WatchCacheSizes{
								Resources: []gardencorev1beta1.ResourceWatchCacheSize{{
									Resource:  "",
									CacheSize: 42,
								}},
							}, ConsistOf(
								field.Required(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.watchCacheSizes.resources[0].resource"), "must not be empty"),
							)),
						)
					})

					Context("Logging", func() {
						var negativeSize int32 = -1

						DescribeTable("Logging validation",
							func(loggingConfig *gardencorev1beta1.APIServerLogging, matcher gomegatypes.GomegaMatcher) {
								garden.Spec.VirtualCluster.Gardener.APIServer.Logging = loggingConfig
								Expect(ValidateGarden(garden)).To(matcher)
							},

							Entry("valid (unset)", nil, BeEmpty()),
							Entry("valid (fields unset)", &gardencorev1beta1.APIServerLogging{}, BeEmpty()),
							Entry("valid (verbosity=0)", &gardencorev1beta1.APIServerLogging{
								Verbosity: ptr.To[int32](0),
							}, BeEmpty()),
							Entry("valid (httpAccessVerbosity=0)", &gardencorev1beta1.APIServerLogging{
								HTTPAccessVerbosity: ptr.To[int32](0),
							}, BeEmpty()),
							Entry("valid (verbosity>0)", &gardencorev1beta1.APIServerLogging{
								Verbosity: ptr.To[int32](3),
							}, BeEmpty()),
							Entry("valid (httpAccessVerbosity>0)", &gardencorev1beta1.APIServerLogging{
								HTTPAccessVerbosity: ptr.To[int32](3),
							}, BeEmpty()),
							Entry("invalid (verbosity<0)", &gardencorev1beta1.APIServerLogging{
								Verbosity: ptr.To(negativeSize),
							}, ConsistOf(
								field.Invalid(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.logging.verbosity"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
							)),
							Entry("invalid (httpAccessVerbosity<0)", &gardencorev1beta1.APIServerLogging{
								HTTPAccessVerbosity: ptr.To(negativeSize),
							}, ConsistOf(
								field.Invalid(field.NewPath("spec.virtualCluster.gardener.gardenerAPIServer.logging.httpAccessVerbosity"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
							)),
						)
					})

					Context("Requests", func() {
						It("should not allow too high values for max inflight requests fields", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.Requests = &gardencorev1beta1.APIServerRequests{
								MaxNonMutatingInflight: ptr.To[int32](123123123),
								MaxMutatingInflight:    ptr.To[int32](412412412),
							}

							Expect(ValidateGarden(garden)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.requests.maxNonMutatingInflight"),
							})), PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.requests.maxMutatingInflight"),
							}))))
						})

						It("should not allow negative values for max inflight requests fields", func() {
							garden.Spec.VirtualCluster.Gardener.APIServer.Requests = &gardencorev1beta1.APIServerRequests{
								MaxNonMutatingInflight: ptr.To(int32(-1)),
								MaxMutatingInflight:    ptr.To(int32(-1)),
							}

							Expect(ValidateGarden(garden)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.requests.maxNonMutatingInflight"),
							})), PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerAPIServer.requests.maxMutatingInflight"),
							}))))
						})
					})
				})

				Context("AdmissionController", func() {
					It("should allow the configuration being set to nil", func() {
						garden.Spec.VirtualCluster.Gardener.AdmissionController = nil

						Expect(ValidateGarden(garden)).To(BeEmpty())
					})

					It("should allow the configuration being empty", func() {
						garden.Spec.VirtualCluster.Gardener.AdmissionController = &operatorv1alpha1.GardenerAdmissionControllerConfig{}

						Expect(ValidateGarden(garden)).To(BeEmpty())
					})

					Context("Resource Admission Configuration", func() {
						Context("Operation mode", func() {
							test := func(mode string) field.ErrorList {
								var (
									admissionConfig *operatorv1alpha1.ResourceAdmissionConfiguration
									operationMode   = operatorv1alpha1.ResourceAdmissionWebhookMode(mode)
								)

								if mode != "" {
									admissionConfig = &operatorv1alpha1.ResourceAdmissionConfiguration{
										OperationMode: &operationMode,
									}
								}

								garden.Spec.VirtualCluster.Gardener.AdmissionController = &operatorv1alpha1.GardenerAdmissionControllerConfig{
									ResourceAdmissionConfiguration: admissionConfig,
								}

								return ValidateGarden(garden)
							}

							It("should allow no mode", func() {
								Expect(test("")).To(BeEmpty())
							})

							It("should allow blocking mode", func() {
								Expect(test("block")).To(BeEmpty())
							})

							It("should allow logging mode", func() {
								Expect(test("log")).To(BeEmpty())
							})

							It("should deny non existing mode", func() {
								Expect(test("foo")).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeNotSupported),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.mode"),
								}))))
							})
						})

						Context("Limits validation", func() {
							var (
								apiGroups = []string{"core.gardener.cloud"}
								versions  = []string{"v1beta1", "v1alpha1"}
								resources = []string{"shoot"}
								size      = "1Ki"

								test = func(apiGroups []string, versions []string, resources []string, size string) field.ErrorList {
									s, err := resource.ParseQuantity(size)
									utilruntime.Must(err)

									garden.Spec.VirtualCluster.Gardener.AdmissionController = &operatorv1alpha1.GardenerAdmissionControllerConfig{
										ResourceAdmissionConfiguration: &operatorv1alpha1.ResourceAdmissionConfiguration{
											Limits: []operatorv1alpha1.ResourceLimit{
												{
													APIGroups:   apiGroups,
													APIVersions: versions,
													Resources:   resources,
													Size:        s,
												},
											},
										},
									}

									return ValidateGarden(garden)
								}
							)

							It("should allow request", func() {
								Expect(test(apiGroups, versions, resources, size)).To(BeEmpty())
							})

							It("should deny empty apiGroup", func() {
								Expect(test(nil, versions, resources, size)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].apiGroups"),
								}))))
							})

							It("should allow apiGroup w/ zero length", func() {
								Expect(test([]string{""}, versions, resources, size)).To(BeEmpty())
							})

							It("should deny empty versions", func() {
								Expect(test(apiGroups, nil, resources, size)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].versions"),
								}))))
							})

							It("should deny versions w/ zero length", func() {
								Expect(test(apiGroups, []string{""}, resources, size)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].versions[0]"),
								}))))
							})

							It("should deny empty resources", func() {
								Expect(test(apiGroups, versions, nil, size)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].resources"),
								}))))
							})

							It("should deny resources w/ zero length", func() {
								Expect(test(apiGroups, versions, []string{""}, size)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].resources[0]"),
								}))))
							})

							It("should deny invalid size", func() {
								Expect(test(apiGroups, versions, resources, "-1k")).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].size"),
								}))))
							})

							It("should deny invalid size and resources w/ zero length", func() {
								Expect(test(apiGroups, versions, []string{resources[0], ""}, "-1k")).To(ConsistOf(
									PointTo(MatchFields(IgnoreExtras, Fields{
										"Type":  Equal(field.ErrorTypeInvalid),
										"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].size"),
									})),
									PointTo(MatchFields(IgnoreExtras, Fields{
										"Type":  Equal(field.ErrorTypeInvalid),
										"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.limits[0].resources[1]"),
									}))))
							})
						})

						Context("User configuration validation", func() {
							var (
								userName       = "admin"
								namespace      = "default"
								emptyNamespace = ""

								test = func(kind string, name string, namespace string, apiGroup string) field.ErrorList {
									garden.Spec.VirtualCluster.Gardener.AdmissionController = &operatorv1alpha1.GardenerAdmissionControllerConfig{
										ResourceAdmissionConfiguration: &operatorv1alpha1.ResourceAdmissionConfiguration{
											UnrestrictedSubjects: []rbacv1.Subject{
												{
													Kind:      kind,
													Name:      name,
													Namespace: namespace,
													APIGroup:  apiGroup,
												},
											},
										},
									}

									return ValidateGarden(garden)
								}
							)

							It("should allow request for user", func() {
								Expect(test(rbacv1.UserKind, userName, emptyNamespace, rbacv1.GroupName)).To(BeEmpty())
							})

							It("should allow request for group", func() {
								Expect(test(rbacv1.GroupKind, userName, emptyNamespace, rbacv1.GroupName)).To(BeEmpty())
							})

							It("should allow request for service account", func() {
								Expect(test(rbacv1.ServiceAccountKind, userName, namespace, "")).To(BeEmpty())
							})

							It("should deny invalid apiGroup for user", func() {
								Expect(test(rbacv1.UserKind, userName, emptyNamespace, "invalid")).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeNotSupported),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].apiGroup"),
								}))))
							})

							It("should deny invalid apiGroup for group", func() {
								Expect(test(rbacv1.GroupKind, userName, emptyNamespace, "invalid")).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeNotSupported),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].apiGroup"),
								}))))
							})

							It("should deny invalid apiGroup for service account", func() {
								Expect(test(rbacv1.ServiceAccountKind, userName, namespace, "invalid")).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].apiGroup"),
								}))))
							})

							It("should deny invalid namespace setting for user", func() {
								Expect(test(rbacv1.UserKind, userName, namespace, rbacv1.GroupName)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].namespace"),
								}))))
							})

							It("should deny invalid namespace setting for group", func() {
								Expect(test(rbacv1.GroupKind, userName, namespace, rbacv1.GroupName)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].namespace"),
								}))))
							})

							It("should deny invalid kind", func() {
								Expect(test("invalidKind", userName, emptyNamespace, rbacv1.GroupName)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeNotSupported),
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].kind"),
								}))))
							})

							It("should deny empty name", func() {
								Expect(test(rbacv1.UserKind, "", emptyNamespace, rbacv1.GroupName)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
									"Field": Equal("spec.virtualCluster.gardener.gardenerAdmissionController.resourceAdmissionConfiguration.unrestrictedSubjects[0].name"),
								}))))
							})
						})
					})
				})

				Context("ControllerManager", func() {
					Context("Feature gates", func() {
						It("should complain when non-existing feature gates were configured", func() {
							garden.Spec.VirtualCluster.Gardener.ControllerManager = &operatorv1alpha1.GardenerControllerManagerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: map[string]bool{"Foo": true},
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerControllerManager.featureGates.Foo"),
							}))))
						})

						It("should complain when invalid feature gates were configured", func() {
							features.AllFeatureGates["Foo"] = featuregate.FeatureSpec{LockToDefault: true, Default: false}
							DeferCleanup(func() {
								delete(features.AllFeatureGates, "Foo")
							})

							garden.Spec.VirtualCluster.Gardener.ControllerManager = &operatorv1alpha1.GardenerControllerManagerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: map[string]bool{"Foo": true},
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerControllerManager.featureGates.Foo"),
							}))))
						})
					})

					Context("Default Project Quotas", func() {
						It("should complain when invalid label selectors were specified", func() {
							garden.Spec.VirtualCluster.Gardener.ControllerManager = &operatorv1alpha1.GardenerControllerManagerConfig{
								DefaultProjectQuotas: []operatorv1alpha1.ProjectQuotaConfiguration{{
									ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"-": "!"}},
								}},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerControllerManager.defaultProjectQuotas[0].projectSelector.matchLabels"),
							}))))
						})
					})
				})

				Context("Scheduler", func() {
					Context("Feature gates", func() {
						It("should complain when non-existing feature gates were configured", func() {
							garden.Spec.VirtualCluster.Gardener.Scheduler = &operatorv1alpha1.GardenerSchedulerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: map[string]bool{"Foo": true},
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerScheduler.featureGates.Foo"),
							}))))
						})

						It("should complain when invalid feature gates were configured", func() {
							features.AllFeatureGates["Foo"] = featuregate.FeatureSpec{LockToDefault: true, Default: false}
							DeferCleanup(func() {
								delete(features.AllFeatureGates, "Foo")
							})

							garden.Spec.VirtualCluster.Gardener.Scheduler = &operatorv1alpha1.GardenerSchedulerConfig{
								KubernetesConfig: gardencorev1beta1.KubernetesConfig{
									FeatureGates: map[string]bool{"Foo": true},
								},
							}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerScheduler.featureGates.Foo"),
							}))))
						})
					})
				})

				Context("Dashboard", func() {
					Context("Token login", func() {
						It("should complain when both token and oidc login is disabled", func() {
							garden.Spec.VirtualCluster.Gardener.Dashboard = &operatorv1alpha1.GardenerDashboardConfig{EnableTokenLogin: ptr.To(false)}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeForbidden),
								"Field": Equal("spec.virtualCluster.gardener.gardenerDashboard.enableTokenLogin"),
							}))))
						})
					})

					Context("OIDC config", func() {
						It("should complain when OIDC config is configured while it is unset in kube-apiserver config", func() {
							garden.Spec.VirtualCluster.Gardener.Dashboard = &operatorv1alpha1.GardenerDashboardConfig{OIDCConfig: &operatorv1alpha1.DashboardOIDC{}}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeInvalid),
								"Field": Equal("spec.virtualCluster.gardener.gardenerDashboard.oidcConfig"),
							}))))
						})

						It("should complain when clientID is missing in OIDC config", func() {
							garden.Spec.VirtualCluster.Gardener.Dashboard = &operatorv1alpha1.GardenerDashboardConfig{OIDCConfig: &operatorv1alpha1.DashboardOIDC{
								IssuerURL: ptr.To("https://example.com"),
							}}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("spec.virtualCluster.gardener.gardenerDashboard.oidcConfig.clientIDPublic"),
							}))))
						})

						It("should complain when clientID is missing in OIDC config but given in kube-apiserver config", func() {
							garden.Spec.VirtualCluster.Gardener.Dashboard = &operatorv1alpha1.GardenerDashboardConfig{OIDCConfig: &operatorv1alpha1.DashboardOIDC{
								IssuerURL: ptr.To("https://example.com"),
							}}
							garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = &operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{OIDCConfig: &gardencorev1beta1.OIDCConfig{
								ClientID: ptr.To("my-client-id"),
							}}}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("spec.virtualCluster.gardener.gardenerDashboard.oidcConfig.clientIDPublic"),
							}))))
						})

						It("should complain when issuerURL is missing in OIDC config", func() {
							garden.Spec.VirtualCluster.Gardener.Dashboard = &operatorv1alpha1.GardenerDashboardConfig{OIDCConfig: &operatorv1alpha1.DashboardOIDC{
								ClientIDPublic: ptr.To("my-client-id"),
							}}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("spec.virtualCluster.gardener.gardenerDashboard.oidcConfig.issuerURL"),
							}))))
						})

						It("should complain when issuerURL is missing in OIDC config but given in kube-apiserver config", func() {
							garden.Spec.VirtualCluster.Gardener.Dashboard = &operatorv1alpha1.GardenerDashboardConfig{OIDCConfig: &operatorv1alpha1.DashboardOIDC{
								ClientIDPublic: ptr.To("my-client-id"),
							}}
							garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = &operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{OIDCConfig: &gardencorev1beta1.OIDCConfig{
								IssuerURL: ptr.To("https://example.com"),
							}}}

							Expect(ValidateGarden(garden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":  Equal(field.ErrorTypeRequired),
								"Field": Equal("spec.virtualCluster.gardener.gardenerDashboard.oidcConfig.issuerURL"),
							}))))
						})

						It("should not complain when OIDC config is configured in both gardener-dashboard and kube-apiserver via structured authentication", func() {
							garden.Spec.VirtualCluster.Kubernetes.Version = "1.30.0"
							garden.Spec.VirtualCluster.Gardener.Dashboard = &operatorv1alpha1.GardenerDashboardConfig{OIDCConfig: &operatorv1alpha1.DashboardOIDC{
								IssuerURL:      ptr.To("https://example.com"),
								ClientIDPublic: ptr.To("my-client-id"),
							}}
							garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = &operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{
								StructuredAuthentication: &gardencorev1beta1.StructuredAuthentication{ConfigMapName: "auth-config"}},
							}

							Expect(ValidateGarden(garden)).To(BeEmpty())
						})

						It("should not complain when OIDC config is configured in both gardener-dashboard and kube-apiserver via OIDC config", func() {
							garden.Spec.VirtualCluster.Gardener.Dashboard = &operatorv1alpha1.GardenerDashboardConfig{OIDCConfig: &operatorv1alpha1.DashboardOIDC{
								IssuerURL:      ptr.To("https://example.com"),
								ClientIDPublic: ptr.To("my-client-id"),
							}}
							garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = &operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{OIDCConfig: &gardencorev1beta1.OIDCConfig{}}}

							Expect(ValidateGarden(garden)).To(BeEmpty())
						})

						It("should not complain when OIDC config is configured in both gardener-dashboard and kube-apiserver via OIDC config IssuerURL and ClientID", func() {
							garden.Spec.VirtualCluster.Gardener.Dashboard = &operatorv1alpha1.GardenerDashboardConfig{OIDCConfig: &operatorv1alpha1.DashboardOIDC{}}
							garden.Spec.VirtualCluster.Kubernetes.KubeAPIServer = &operatorv1alpha1.KubeAPIServerConfig{KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{OIDCConfig: &gardencorev1beta1.OIDCConfig{
								IssuerURL: ptr.To("https://example.com"),
								ClientID:  ptr.To("my-client-id"),
							}}}

							Expect(ValidateGarden(garden)).To(BeEmpty())
						})
					})
				})
			})
		})
	})

	Describe("#ValidateGardenUpdate", func() {
		var oldGarden, newGarden *operatorv1alpha1.Garden

		BeforeEach(func() {
			oldGarden = &operatorv1alpha1.Garden{
				ObjectMeta: metav1.ObjectMeta{
					Name: "garden",
				},
				Spec: operatorv1alpha1.GardenSpec{
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Ingress: operatorv1alpha1.Ingress{
							Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.bar.com"}},
						},
						Networking: operatorv1alpha1.RuntimeNetworking{
							Pods:     "10.1.0.0/16",
							Services: "10.2.0.0/16",
						},
					},
					VirtualCluster: operatorv1alpha1.VirtualCluster{
						Kubernetes: operatorv1alpha1.Kubernetes{
							Version: "1.27.0",
							KubeAPIServer: &operatorv1alpha1.KubeAPIServerConfig{
								KubeAPIServerConfig: &gardencorev1beta1.KubeAPIServerConfig{
									EncryptionConfig: &gardencorev1beta1.EncryptionConfig{},
								},
							},
						},
						Networking: operatorv1alpha1.Networking{
							Services: "10.4.0.0/16",
						},
					},
				},
			}

			newGarden = oldGarden.DeepCopy()
		})

		Context("runtime cluster", func() {
			Context("ingress", func() {
				It("should allow update if nothing changes", func() {
					oldGarden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}}
					newGarden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).NotTo(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Field": ContainSubstring("domain"),
					}))))
				})

				It("should allow adding a domain", func() {
					oldGarden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}}
					newGarden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}, {Name: "foo.bar"}}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).NotTo(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Field": ContainSubstring("domain"),
					}))))
				})

				It("should allow removing any domain but first entry", func() {
					oldGarden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{
						{Name: "example.com"},
						{Name: "foo.bar"},
						{Name: "bar.foo"},
					}
					newGarden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{
						{Name: "example.com"},
						{Name: "bar.foo"},
					}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).NotTo(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Field": ContainSubstring("domain"),
					}))))
				})

				It("should forbid removing the first entry", func() {
					oldGarden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{
						{Name: "example.com"},
						{Name: "foo.bar"},
						{Name: "bar.foo"},
					}
					newGarden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{{Name: "bar.foo"}}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.runtimeCluster.ingress.domains[0]"),
					}))))
				})

				It("should forbid changing the first entry", func() {
					oldGarden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{
						{Name: "example.com"},
						{Name: "foo.bar"},
						{Name: "bar.foo"},
					}
					newGarden.Spec.RuntimeCluster.Ingress.Domains = []operatorv1alpha1.DNSDomain{
						{Name: "example2.com"},
						{Name: "foo.bar"},
						{Name: "bar.foo"},
					}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.runtimeCluster.ingress.domains[0]"),
					}))))
				})
			})
		})

		Context("virtual cluster", func() {
			Context("dns", func() {
				Context("when domains are modified", func() {
					It("should allow update if nothing changes", func() {
						oldGarden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}}
						newGarden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).NotTo(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
							"Field": ContainSubstring("domain"),
						}))))
					})

					It("should allow adding a domain", func() {
						oldGarden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{{Name: "example.com"}}
						newGarden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{
							{Name: "example.com"},
							{Name: "foo.bar"},
						}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).NotTo(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
							"Field": ContainSubstring("domain"),
						}))))
					})

					It("should allow removing any domain but first entry", func() {
						oldGarden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{
							{Name: "example.com"},
							{Name: "foo.bar"},
							{Name: "bar.foo"},
						}
						newGarden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{
							{Name: "example.com"},
							{Name: "bar.foo"},
						}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).NotTo(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
							"Field": ContainSubstring("domain"),
						}))))
					})

					It("should forbid removing the first entry", func() {
						oldGarden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{
							{Name: "example.com"},
							{Name: "foo.bar"},
							{Name: "bar.foo"},
						}
						newGarden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{{Name: "bar.foo"}}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.virtualCluster.dns.domains[0].name"),
						}))))
					})

					It("should forbid changing the first entry", func() {
						oldGarden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{
							{Name: "example.com"},
							{Name: "foo.bar"},
							{Name: "bar.foo"},
						}
						newGarden.Spec.VirtualCluster.DNS.Domains = []operatorv1alpha1.DNSDomain{
							{Name: "example2.com"},
							{Name: "foo.bar"},
							{Name: "bar.foo"},
						}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.virtualCluster.dns.domains[0].name"),
						}))))
					})
				})
			})

			Context("ETCD", func() {
				It("should not be possible to set the backup bucket name if it was unset initially", func() {
					oldGarden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{
						Main: &operatorv1alpha1.ETCDMain{
							Backup: &operatorv1alpha1.Backup{
								Provider: "foo-provider",
								ProviderConfig: &runtime.RawExtension{
									Raw: []byte(`{"foo":"bar"}`),
								},
							},
						},
					}
					newGarden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{
						Main: &operatorv1alpha1.ETCDMain{
							Backup: &operatorv1alpha1.Backup{
								BucketName: ptr.To("foo-bucket"),
								Provider:   "foo-provider",
							},
						},
					}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.etcd.main.backup.bucketName"),
					}))))
				})

				It("should not be possible to delete the backup bucket name if it was set initially", func() {
					oldGarden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{
						Main: &operatorv1alpha1.ETCDMain{
							Backup: &operatorv1alpha1.Backup{
								Provider:   "foo-provider",
								BucketName: ptr.To("foo-bucket"),
							},
						},
					}
					newGarden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{
						Main: &operatorv1alpha1.ETCDMain{
							Backup: &operatorv1alpha1.Backup{
								Provider: "foo-provider",
							},
						},
					}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.etcd.main.backup.bucketName"),
					}))))
				})

				It("should not be possible to delete the backup if it was set initially", func() {
					oldGarden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{
						Main: &operatorv1alpha1.ETCDMain{
							Backup: &operatorv1alpha1.Backup{
								Provider:   "foo-provider",
								BucketName: ptr.To("foo-bucket"),
							},
						},
					}
					newGarden.Spec.VirtualCluster.ETCD = &operatorv1alpha1.ETCD{
						Main: &operatorv1alpha1.ETCDMain{
							Backup: nil,
						},
					}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.virtualCluster.etcd.main.backup"),
					}))))
				})
			})

			Context("control plane", func() {
				It("should not be possible to remove the high availability setting once set", func() {
					oldGarden.Spec.VirtualCluster.ControlPlane = &operatorv1alpha1.ControlPlane{HighAvailability: &operatorv1alpha1.HighAvailability{}}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.controlPlane.highAvailability"),
					}))))
				})
			})

			Context("kubernetes", func() {
				It("should not not allow version downgrade", func() {
					version := semver.MustParse(newGarden.Spec.VirtualCluster.Kubernetes.Version)
					previousMinor := semver.MustParse(fmt.Sprintf("%d.%d.%d", version.Major(), version.Minor()-1, version.Patch()))

					newGarden.Spec.VirtualCluster.Kubernetes.Version = previousMinor.String()

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.virtualCluster.kubernetes.version"),
					}))))
				})

				Context("encryptionConfig", func() {
					It("should deny changing items if the current resources in the status do not match the current spec", func() {
						oldResources := []string{"resource.custom.io", "deployments.apps"}
						oldGardenerResources := []string{"shoots.core.gardener.cloud", "bastions.operations.gardener.cloud"}

						oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
							Resources: oldResources,
						}
						oldGarden.Spec.VirtualCluster.Gardener = operatorv1alpha1.Gardener{
							APIServer: &operatorv1alpha1.GardenerAPIServerConfig{
								EncryptionConfig: &gardencorev1beta1.EncryptionConfig{
									Resources: oldGardenerResources,
								},
							},
						}

						newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"deployments.apps", "newresource.fancyresource.io"}
						newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"shoots.core.gardener.cloud"}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(field.ErrorTypeForbidden),
								"Field":  Equal("spec.virtualCluster.kubernetes.kubeAPIServer.encryptionConfig.resources"),
								"Detail": Equal("resources cannot be changed because a previous encryption configuration change is currently being rolled out"),
							})),
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(field.ErrorTypeForbidden),
								"Field":  Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources"),
								"Detail": Equal("resources cannot be changed because a previous encryption configuration change is currently being rolled out"),
							})),
						))
					})

					It("should deny changing items during ETCD Encryption Key rotation", func() {
						oldResources := []string{"resource.custom.io", "deployments.apps"}
						oldGardenerResources := []string{"shoots.core.gardener.cloud", "bastions.operations.gardener.cloud"}
						oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
							Resources: oldResources,
						}
						oldGarden.Spec.VirtualCluster.Gardener = operatorv1alpha1.Gardener{
							APIServer: &operatorv1alpha1.GardenerAPIServerConfig{
								EncryptionConfig: &gardencorev1beta1.EncryptionConfig{
									Resources: oldGardenerResources,
								},
							},
						}
						newGarden.Status.EncryptedResources = append(oldResources, oldGardenerResources...)

						newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"deployments.apps", "newresource.fancyresource.io"}
						newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"shoots.core.gardener.cloud"}

						newGarden.Status.Credentials = &operatorv1alpha1.Credentials{
							Rotation: &operatorv1alpha1.CredentialsRotation{
								ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
									Phase: gardencorev1beta1.RotationPreparing,
								},
							},
						}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(field.ErrorTypeForbidden),
								"Field":  Equal("spec.virtualCluster.kubernetes.kubeAPIServer.encryptionConfig.resources"),
								"Detail": Equal("resources cannot be changed when .status.credentials.rotation.etcdEncryptionKey.phase is not \"Completed\""),
							})),
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(field.ErrorTypeForbidden),
								"Field":  Equal("spec.virtualCluster.gardener.gardenerAPIServer.encryptionConfig.resources"),
								"Detail": Equal("resources cannot be changed when .status.credentials.rotation.etcdEncryptionKey.phase is not \"Completed\""),
							})),
						))
					})

					It("should allow changing items if ETCD Encryption Key rotation is in phase Completed or was never rotated", func() {
						oldResources := []string{"resource.custom.io", "deployments.apps"}
						oldGardenerResources := []string{"shoots.core.gardener.cloud", "bastions.operations.gardener.cloud"}
						oldGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
							Resources: oldResources,
						}
						oldGarden.Spec.VirtualCluster.Gardener = operatorv1alpha1.Gardener{
							APIServer: &operatorv1alpha1.GardenerAPIServerConfig{
								EncryptionConfig: &gardencorev1beta1.EncryptionConfig{
									Resources: oldGardenerResources,
								},
							},
						}
						newGarden.Status.EncryptedResources = append(oldResources, oldGardenerResources...)

						newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"deployments.apps", "newresource.fancyresource.io"}
						newGarden.Spec.VirtualCluster.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"shoots.core.gardener.cloud"}
						newGarden.Status.Credentials = nil

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(BeEmpty())

						newGarden.Status.Credentials = &operatorv1alpha1.Credentials{
							Rotation: &operatorv1alpha1.CredentialsRotation{
								ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
									Phase: gardencorev1beta1.RotationCompleted,
								},
							},
						}

						Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(BeEmpty())
					})
				})
			})
		})
	})
})
