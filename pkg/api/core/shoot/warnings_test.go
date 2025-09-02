// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/api/core/shoot"
	"github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("Warnings", func() {
	Describe("#GetWarnings", func() {
		var (
			ctx                         = context.Background()
			shoot                       *core.Shoot
			credentialsRotationInterval = time.Hour
		)

		BeforeEach(func() {
			shoot = &core.Shoot{
				Spec: core.ShootSpec{
					Kubernetes: core.Kubernetes{
						Version: "1.31.1",
					},
					Provider: core.Provider{
						Workers: []core.Worker{{Name: "test"}},
					},
				},
			}
		})

		It("should return nil when shoot is nil", func() {
			Expect(GetWarnings(ctx, nil, nil, credentialsRotationInterval)).To(BeEmpty())
		})

		It("should return nil when shoot does not have any problematic configuration", func() {
			Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(BeEmpty())
		})

		Context("credentials rotation", func() {
			BeforeEach(func() {
				shoot.CreationTimestamp = metav1.Time{Time: time.Now().Add(-credentialsRotationInterval * 2)}
			})

			It("should not return a warning when credentials rotation is due in case shoot is too young", func() {
				shoot.CreationTimestamp = metav1.Now()
				Expect(GetWarnings(ctx, shoot, shoot, credentialsRotationInterval)).To(BeEmpty())
			})

			It("should return a warning when credentials rotation is due", func() {
				Expect(GetWarnings(ctx, shoot, shoot, credentialsRotationInterval)).To(ContainElement(ContainSubstring("you should consider rotating the shoot credentials")))
			})

			DescribeTable("warnings for specific credentials rotations",
				func(matcher gomegatypes.GomegaMatcher, mutateShoot func(*core.Shoot), mutateRotation func(rotation *core.ShootCredentialsRotation)) {
					if mutateShoot != nil {
						mutateShoot(shoot)
					}

					rotation := &core.ShootCredentialsRotation{
						CertificateAuthorities: &core.CARotation{},
						SSHKeypair:             &core.ShootSSHKeypairRotation{},
						Observability:          &core.ObservabilityRotation{},
						ServiceAccountKey:      &core.ServiceAccountKeyRotation{},
						ETCDEncryptionKey:      &core.ETCDEncryptionKeyRotation{},
					}
					mutateRotation(rotation)
					shoot.Status.Credentials = &core.ShootCredentials{Rotation: rotation}

					Expect(GetWarnings(ctx, shoot, shoot, credentialsRotationInterval)).To(matcher)
				},

				Entry("ca nil", ContainElement(ContainSubstring("you should consider rotating the certificate authorities")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.CertificateAuthorities = nil
					},
				),
				Entry("ca last initiated too long ago", ContainElement(ContainSubstring("you should consider rotating the certificate authorities")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.CertificateAuthorities.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval * 2)}
					},
				),
				Entry("ca last initiated not too long ago", Not(ContainElement(ContainSubstring("you should consider rotating the certificate authorities"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.CertificateAuthorities.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
					},
				),
				Entry("ca completion is due (never completed yet)", ContainElement(ContainSubstring("the certificate authorities rotation initiation was finished more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.CertificateAuthorities.LastInitiationFinishedTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.CertificateAuthorities.LastCompletionTriggeredTime = nil
					},
				),
				Entry("ca completion is due (current rotation not completed)", ContainElement(ContainSubstring("the certificate authorities rotation initiation was finished more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.CertificateAuthorities.LastInitiationFinishedTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.CertificateAuthorities.LastCompletionTriggeredTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval)}
					},
				),
				Entry("ca completion is not due (current rotation not completed)", Not(ContainElement(ContainSubstring("the certificate authorities rotation initiation was finished more than"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.CertificateAuthorities.LastInitiationFinishedTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.CertificateAuthorities.LastCompletionTriggeredTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 3)}
					},
				),

				Entry("etcdEncryptionKey nil", ContainElement(ContainSubstring("you should consider rotating the ETCD encryption key")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ETCDEncryptionKey = nil
					},
				),
				Entry("etcdEncryptionKey last initiated too long ago", ContainElement(ContainSubstring("you should consider rotating the ETCD encryption key")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ETCDEncryptionKey.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval * 2)}
					},
				),
				Entry("etcdEncryptionKey last initiated not too long ago", Not(ContainElement(ContainSubstring("you should consider rotating the ETCD encryption key"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ETCDEncryptionKey.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
					},
				),
				Entry("etcdEncryptionKey completion is due (never completed yet)", ContainElement(ContainSubstring("the ETCD encryption key rotation initiation was finished more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ETCDEncryptionKey.LastInitiationFinishedTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ETCDEncryptionKey.LastCompletionTriggeredTime = nil
					},
				),
				Entry("etcdEncryptionKey completion is due (current rotation not completed)", ContainElement(ContainSubstring("the ETCD encryption key rotation initiation was finished more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ETCDEncryptionKey.LastInitiationFinishedTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ETCDEncryptionKey.LastCompletionTriggeredTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval)}
					},
				),
				Entry("etcdEncryptionKey completion is not due (current rotation not completed)", Not(ContainElement(ContainSubstring("the ETCD encryption key rotation initiation was finished more than"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ETCDEncryptionKey.LastInitiationFinishedTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ETCDEncryptionKey.LastCompletionTriggeredTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 3)}
					},
				),

				Entry("observability nil", ContainElement(ContainSubstring("you should consider rotating the observability passwords")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.Observability = nil
					},
				),
				Entry("observability last initiated too long ago", ContainElement(ContainSubstring("you should consider rotating the observability passwords")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.Observability.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval * 2)}
					},
				),
				Entry("observability last initiated too long ago but shoot purpose is testing", Not(ContainElement(ContainSubstring("you should consider rotating the observability passwords"))),
					func(shoot *core.Shoot) {
						p := core.ShootPurposeTesting
						shoot.Spec.Purpose = &p
					},
					func(rotation *core.ShootCredentialsRotation) {
						rotation.Observability.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval * 2)}
					},
				),
				Entry("observability last initiated not too long ago", Not(ContainElement(ContainSubstring("you should consider rotating the observability passwords"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.Observability.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
					},
				),

				Entry("serviceAccountKey nil", ContainElement(ContainSubstring("you should consider rotating the ServiceAccount token signing key")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ServiceAccountKey = nil
					},
				),
				Entry("serviceAccountKey last initiated too long ago", ContainElement(ContainSubstring("you should consider rotating the ServiceAccount token signing key")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ServiceAccountKey.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval * 2)}
					},
				),
				Entry("serviceAccountKey last initiated not too long ago", Not(ContainElement(ContainSubstring("you should consider rotating the ServiceAccount token signing key"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ServiceAccountKey.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
					},
				),
				Entry("serviceAccountKey completion is due (never completed yet)", ContainElement(ContainSubstring("the ServiceAccount token signing key rotation initiation was finished more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ServiceAccountKey.LastInitiationFinishedTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ServiceAccountKey.LastCompletionTriggeredTime = nil
					},
				),
				Entry("serviceAccountKey completion is due (current rotation not completed)", ContainElement(ContainSubstring("the ServiceAccount token signing key rotation initiation was finished more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ServiceAccountKey.LastInitiationFinishedTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ServiceAccountKey.LastCompletionTriggeredTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval)}
					},
				),
				Entry("serviceAccountKey completion is not due (current rotation not completed)", Not(ContainElement(ContainSubstring("the ServiceAccount token signing key rotation initiation was finished more than"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ServiceAccountKey.LastInitiationFinishedTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ServiceAccountKey.LastCompletionTriggeredTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 3)}
					},
				),

				Entry("sshKeypair nil", ContainElement(ContainSubstring("you should consider rotating the SSH keypair")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.SSHKeypair = nil
					},
				),
				Entry("sshKeypair last initiated too long ago", ContainElement(ContainSubstring("you should consider rotating the SSH keypair")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.SSHKeypair.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval * 2)}
					},
				),
				Entry("sshKeypair last initiated not too long ago", Not(ContainElement(ContainSubstring("you should consider rotating the SSH keypair"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.SSHKeypair.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
					},
				),
				Entry("ssh is disabled for shoot with workers", Not(ContainElement(ContainSubstring("you should consider rotating the SSH keypair"))), func(shoot *core.Shoot) {
					shoot.Spec.Provider.WorkersSettings = &core.WorkersSettings{
						SSHAccess: &core.SSHAccess{
							Enabled: false,
						},
					}
				}, func(rotation *core.ShootCredentialsRotation) {
					rotation.SSHKeypair = nil
				},
				),
				Entry("workerless shoot", Not(ContainElement(ContainSubstring("you should consider rotating the SSH keypair"))), func(shoot *core.Shoot) {
					shoot.Spec.Provider.Workers = nil
				}, func(rotation *core.ShootCredentialsRotation) {
					rotation.SSHKeypair = nil
				},
				),
			)
		})

		It("should return a warning when podEvictionTimeout is set", func() {
			shoot.Spec.Kubernetes.KubeControllerManager = &core.KubeControllerManagerConfig{
				PodEvictionTimeout: &metav1.Duration{Duration: 2 * time.Minute},
			}
			Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(ContainElement(Equal("you are setting the spec.kubernetes.kubeControllerManager.podEvictionTimeout field. The field does not have effect since Kubernetes 1.13 and is forbidden to be set starting from Kubernetes 1.33. Instead, use the spec.kubernetes.kubeAPIServer.(defaultNotReadyTolerationSeconds/defaultUnreachableTolerationSeconds) fields.")))
		})

		It("should warn when maxEmptyBulkDelete is set for shoots using kubernetes < v1.33", func() {
			shoot.Spec.Kubernetes.Version = "1.32.4"
			shoot.Spec.Kubernetes.ClusterAutoscaler = &core.ClusterAutoscaler{
				MaxEmptyBulkDelete: ptr.To(int32(5)),
			}
			Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(ContainElement(Equal("you are setting the spec.kubernetes.clusterAutoscaler.maxEmptyBulkDelete field. The field has been deprecated and is forbidden to be set starting from Kubernetes 1.33. Instead, use the spec.kubernetes.clusterAutoscaler.maxScaleDownParallelism field.")))
		})

		It("should warn when rotate-etcd-encryption-key-start operation annotation is set", func() {
			shoot.Annotations = map[string]string{
				"gardener.cloud/operation": "rotate-etcd-encryption-key-start",
			}
			Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(ContainElement(Equal("you are setting the operation annotation to rotate-etcd-encryption-key-start. This annotation has been deprecated and is forbidden to be set starting from Kubernetes 1.34. Instead, use the rotate-etcd-encryption-key annotation, which performs a full rotation of the ETCD encryption key.")))
		})

		It("should warn when rotate-etcd-encryption-key-complete operation annotation is set", func() {
			shoot.Annotations = map[string]string{
				"gardener.cloud/operation": "rotate-etcd-encryption-key-complete",
			}
			Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(ContainElement(Equal("you are setting the operation annotation to rotate-etcd-encryption-key-complete. This annotation has been deprecated and is forbidden to be set starting from Kubernetes 1.34. Instead, use the rotate-etcd-encryption-key annotation, which performs a full rotation of the ETCD encryption key.")))
		})

		It("should return a warning when enableAnonymousAuthentication is set", func() {
			shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{EnableAnonymousAuthentication: ptr.To(true)}
			Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(ContainElement(Equal("you are setting the spec.kubernetes.kubeAPIServer.enableAnonymousAuthentication field. The field is deprecated. Using Kubernetes v1.32 and above, please use anonymous authentication configuration. See: https://kubernetes.io/docs/reference/access-authn-authz/authentication/#anonymous-authenticator-configuration")))
		})

		DescribeTable("shoot.spec.secretBindingName",
			func(secretBindingName *string, expectedWarning gomegatypes.GomegaMatcher) {
				shoot.Spec.SecretBindingName = secretBindingName
				Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(expectedWarning)
			},

			Entry("should return a warning when secretBindingName is set", ptr.To("my-secret-binding"),
				ContainElement(Equal("spec.secretBindingName is deprecated and will be disallowed starting with Kubernetes 1.34. For migration instructions, see: https://github.com/gardener/gardener/blob/master/docs/usage/shoot-operations/secretbinding-to-credentialsbinding-migration.md"))),
			Entry("should not return a warning when secretBindingName is not set", nil, BeEmpty()),
		)

		Describe("shoot.spec.cloudProfileName", func() {
			It("should not return a warning when cloudProfileName is set and the Kubernetes version is < v1.33", func() {
				shoot.Spec.Kubernetes.Version = "1.32.3"
				shoot.Spec.CloudProfileName = ptr.To("local-profile")
				Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(BeEmpty())
			})

			It("should return a warning when cloudProfileName is set and the Kubernetes version is >= v1.33", func() {
				shoot.Spec.Kubernetes.Version = "1.33.1"
				shoot.Spec.CloudProfileName = ptr.To("local-profile")
				Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(ContainElement(ContainSubstring("you are setting the spec.cloudProfileName field. The field is deprecated")))
			})

			It("should not return a warning when cloudProfileName is empty and the Kubernetes version is >= v1.33", func() {
				shoot.Spec.Kubernetes.Version = "1.33.1"
				Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(BeEmpty())
			})
		})
	})
})
