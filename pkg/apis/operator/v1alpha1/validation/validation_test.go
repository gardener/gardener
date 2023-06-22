// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/validation"
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
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Networking: operatorv1alpha1.RuntimeNetworking{
							Pods:     "10.1.0.0/16",
							Services: "10.2.0.0/16",
						},
					},
					VirtualCluster: operatorv1alpha1.VirtualCluster{
						DNS: operatorv1alpha1.DNS{
							Domain: pointer.String("foo.bar.com"),
						},
						Kubernetes: operatorv1alpha1.Kubernetes{
							Version: "1.26.3",
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
			)

			DescribeTable("starting rotation of all credentials",
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-start")
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
				Entry("sa rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPreparing,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is preparing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
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
				Entry("sa rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
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
				Entry("sa rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleting,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completing", false, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
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
				Entry("sa rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
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
				func(allowed bool, status operatorv1alpha1.GardenStatus) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "rotate-etcd-encryption-key-start")
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
				Entry("rotation phase is prepared", false, operatorv1alpha1.GardenStatus{
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
				Entry("rotation phase is completed", true, operatorv1alpha1.GardenStatus{
					Credentials: &operatorv1alpha1.Credentials{
						Rotation: &operatorv1alpha1.CredentialsRotation{
							ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{
								Phase: gardencorev1beta1.RotationCompleted,
							},
						},
					},
				}),
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
		})

		Context("virtual cluster", func() {
			Context("DNS", func() {
				It("should complain about invalid domain names", func() {
					garden.Spec.VirtualCluster.DNS.Domain = pointer.String(",,,")
					garden.Spec.VirtualCluster.DNS.Domains = []string{",,,"}

					Expect(ValidateGarden(garden)).To(ContainElements(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.virtualCluster.dns.domain"),
						})),
					))
				})

				It("should complain about duplicate domain names in 'domains'", func() {
					garden.Spec.VirtualCluster.DNS.Domains = []string{
						"example.com",
						"foo.bar",
						"example.com",
					}

					Expect(ValidateGarden(garden)).To(ContainElements(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal("spec.virtualCluster.dns.domains[2]"),
						})),
					))
				})

				It("should complain about duplicate domain names in 'domain'", func() {
					garden.Spec.VirtualCluster.DNS.Domain = pointer.String("example.com")
					garden.Spec.VirtualCluster.DNS.Domains = []string{
						"example.com",
						"foo.bar",
					}

					Expect(ValidateGarden(garden)).To(ContainElements(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal("spec.virtualCluster.dns.domain"),
						})),
					))
				})
			})

			Context("networking", func() {
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
		})
	})

	Describe("#ValidateGardenUpdate", func() {
		var oldGarden, newGarden *operatorv1alpha1.Garden

		BeforeEach(func() {
			oldGarden = &operatorv1alpha1.Garden{
				ObjectMeta: metav1.ObjectMeta{
					Name: "garden",
				},
			}
			newGarden = oldGarden.DeepCopy()
		})

		Context("virtual cluster", func() {
			Context("control plane", func() {
				It("should not be possible to remove the high availability setting once set", func() {
					oldGarden.Spec.VirtualCluster.ControlPlane = &operatorv1alpha1.ControlPlane{HighAvailability: &operatorv1alpha1.HighAvailability{}}

					Expect(ValidateGardenUpdate(oldGarden, newGarden)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.virtualCluster.controlPlane.highAvailability"),
					}))))
				})
			})
		})
	})
})
