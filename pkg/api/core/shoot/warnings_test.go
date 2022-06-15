// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_test

import (
	"context"
	"time"

	. "github.com/gardener/gardener/pkg/api/core/shoot"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/utils/pointer"
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
						EnableStaticTokenKubeconfig: pointer.Bool(false),
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
			shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(true)
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
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.ShootCARotation, true)()
					defer test.WithFeatureGate(utilfeature.DefaultFeatureGate, features.ShootSARotation, true)()

					if mutateShoot != nil {
						mutateShoot(shoot)
					}

					rotation := &core.ShootCredentialsRotation{
						CertificateAuthorities: &core.ShootCARotation{},
						Kubeconfig:             &core.ShootKubeconfigRotation{},
						SSHKeypair:             &core.ShootSSHKeypairRotation{},
						Observability:          &core.ShootObservabilityRotation{},
						ServiceAccountKey:      &core.ShootServiceAccountKeyRotation{},
						ETCDEncryptionKey:      &core.ShootETCDEncryptionKeyRotation{},
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
				Entry("ca completion is due (never completed yet)", ContainElement(ContainSubstring("the certificate authorities rotation was initiated more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.CertificateAuthorities.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.CertificateAuthorities.LastCompletionTime = nil
					},
				),
				Entry("ca completion is due (current rotation not completed)", ContainElement(ContainSubstring("the certificate authorities rotation was initiated more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.CertificateAuthorities.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.CertificateAuthorities.LastCompletionTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval)}
					},
				),
				Entry("ca completion is not due (current rotation not completed)", Not(ContainElement(ContainSubstring("the certificate authorities rotation was initiated more than"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.CertificateAuthorities.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.CertificateAuthorities.LastCompletionTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 3)}
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
				Entry("etcdEncryptionKey completion is due (never completed yet)", ContainElement(ContainSubstring("the ETCD encryption key rotation was initiated more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ETCDEncryptionKey.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ETCDEncryptionKey.LastCompletionTime = nil
					},
				),
				Entry("etcdEncryptionKey completion is due (current rotation not completed)", ContainElement(ContainSubstring("the ETCD encryption key rotation was initiated more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ETCDEncryptionKey.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ETCDEncryptionKey.LastCompletionTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval)}
					},
				),
				Entry("etcdEncryptionKey completion is not due (current rotation not completed)", Not(ContainElement(ContainSubstring("the ETCD encryption key rotation was initiated more than"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ETCDEncryptionKey.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ETCDEncryptionKey.LastCompletionTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 3)}
					},
				),

				Entry("kubeconfig nil", ContainElement(ContainSubstring("you should consider rotating the static token kubeconfig")),
					func(shoot *core.Shoot) {
						shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(true)
					},
					func(rotation *core.ShootCredentialsRotation) {
						rotation.Kubeconfig = nil
					},
				),
				Entry("kubeconfig last initiated too long ago", ContainElement(ContainSubstring("you should consider rotating the static token kubeconfig")),
					func(shoot *core.Shoot) {
						shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(true)
					},
					func(rotation *core.ShootCredentialsRotation) {
						rotation.Kubeconfig.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval * 2)}
					},
				),
				Entry("kubeconfig last initiated too long ago but disabled", Not(ContainElement(ContainSubstring("you should consider rotating the static token kubeconfig"))),
					func(shoot *core.Shoot) {
						shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(false)
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
				Entry("serviceAccountKey completion is due (never completed yet)", ContainElement(ContainSubstring("the ServiceAccount token signing key rotation was initiated more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ServiceAccountKey.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ServiceAccountKey.LastCompletionTime = nil
					},
				),
				Entry("serviceAccountKey completion is due (current rotation not completed)", ContainElement(ContainSubstring("the ServiceAccount token signing key rotation was initiated more than")), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ServiceAccountKey.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ServiceAccountKey.LastCompletionTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval)}
					},
				),
				Entry("serviceAccountKey completion is not due (current rotation not completed)", Not(ContainElement(ContainSubstring("the ServiceAccount token signing key rotation was initiated more than"))), nil,
					func(rotation *core.ShootCredentialsRotation) {
						rotation.ServiceAccountKey.LastInitiationTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 2)}
						rotation.ServiceAccountKey.LastCompletionTime = &metav1.Time{Time: time.Now().Add(-credentialsRotationInterval / 3)}
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
			)
		})
	})
})
