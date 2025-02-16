// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
			ctx                         = context.TODO()
			shoot                       *core.Shoot
			credentialsRotationInterval = time.Hour
		)

		BeforeEach(func() {
			shoot = &core.Shoot{
				Spec: core.ShootSpec{
					Kubernetes: core.Kubernetes{
						Version:                     "1.26.5",
						EnableStaticTokenKubeconfig: ptr.To(false),
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

		It("should return a warning when static token kubeconfig is nil", func() {
			shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = nil
			Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(ContainElement(ContainSubstring("you should consider disabling the static token kubeconfig")))
		})

		It("should return a warning when static token kubeconfig is explicitly enabled", func() {
			shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = ptr.To(true)
			Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(ContainElement(ContainSubstring("you should consider disabling the static token kubeconfig")))
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
						Kubeconfig:             &core.ShootKubeconfigRotation{},
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

				Entry("kubeconfig nil", ContainElement(ContainSubstring("you should consider rotating the static token kubeconfig")),
					func(shoot *core.Shoot) {
						shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = ptr.To(true)
					},
					func(rotation *core.ShootCredentialsRotation) {
						rotation.Kubeconfig = nil
					},
				),
				Entry("kubeconfig last initiated too long ago", ContainElement(ContainSubstring("you should consider rotating the static token kubeconfig")),
					func(shoot *core.Shoot) {
						shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = ptr.To(true)
					},
					func(rotation *core.ShootCredentialsRotation) {
						rotation.Kubeconfig.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval * 2)}
					},
				),
				Entry("kubeconfig last initiated too long ago but disabled", Not(ContainElement(ContainSubstring("you should consider rotating the static token kubeconfig"))),
					func(shoot *core.Shoot) {
						shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = ptr.To(false)
					},
					func(rotation *core.ShootCredentialsRotation) {
						rotation.Kubeconfig.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval * 2)}
					},
				),
				Entry("kubeconfig last initiated not too long ago", Not(ContainElement(ContainSubstring("you should consider rotating the static token kubeconfig"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.Kubeconfig.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
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
			Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(ContainElement(Equal("you are setting the spec.kubernetes.kubeControllerManager.podEvictionTimeout field. The field does not have effect since Kubernetes 1.13. Instead, use the spec.kubernetes.kubeAPIServer.(defaultNotReadyTolerationSeconds/defaultUnreachableTolerationSeconds) fields.")))
		})

		Context("shoot.gardener.cloud/managed-seed-api-server annotation", func() {
			It("should not return a warning when the annotation is set but namespace is not garden", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/managed-seed-api-server", "apiServer.replicas=3,apiServer.autoscaler.maxReplicas=6,apiServer.autoscaler.minReplicas=3")
				shoot.Namespace = "garden-dev"

				Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).NotTo(ContainElement(ContainSubstring("shoot.gardener.cloud/managed-seed-api-server")))
			})

			It("should return a warning when the annotation is set and namespace is garden", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "shoot.gardener.cloud/managed-seed-api-server", "apiServer.replicas=3,apiServer.autoscaler.maxReplicas=6,apiServer.autoscaler.minReplicas=3")
				shoot.Namespace = "garden"

				Expect(GetWarnings(ctx, shoot, nil, credentialsRotationInterval)).To(ContainElement(Equal("annotation 'shoot.gardener.cloud/managed-seed-api-server' is deprecated, instead consider enabling high availability for the ManagedSeed's Shoot control plane")))
			})
		})
	})
})
