// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVar(&secretsutils.GenerateRandomString, secretsutils.FakeGenerateRandomString))
	DeferCleanup(test.WithVar(&secretsutils.GenerateKey, secretsutils.FakeGenerateKey))
})

var _ = Describe("Generate", func() {
	var (
		ctx       = context.TODO()
		namespace = "shoot--foo--bar"
		identity  = "test"

		m          *manager
		fakeClient client.Client
		fakeClock  = testclock.NewFakeClock(time.Time{})
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

		mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{})
		Expect(err).NotTo(HaveOccurred())
		m = mgr.(*manager)
	})

	Describe("#Generate", func() {
		name := "config"

		Context("for non-certificate secrets", func() {
			var config *secretsutils.BasicAuthSecretConfig

			BeforeEach(func() {
				config = &secretsutils.BasicAuthSecretConfig{
					Name:           name,
					Format:         secretsutils.BasicAuthFormatNormal,
					Username:       "foo",
					PasswordLength: 3,
				}
			})

			It("should generate a new secret", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("Verify internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(secret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should maintain the lifetime labels (w/o validity)", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())

				By("Read created secret from system")
				foundSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), foundSecret)).To(Succeed())

				By("Verify labels")
				Expect(foundSecret.Labels).To(And(
					HaveKeyWithValue("issued-at-time", strconv.FormatInt(fakeClock.Now().Unix(), 10)),
					Not(HaveKey("valid-until-time")),
				))
			})

			It("should maintain the lifetime labels (w/ validity)", func() {
				By("Generate new secret")
				validity := time.Hour
				secret, err := m.Generate(ctx, config, Validity(validity))
				Expect(err).NotTo(HaveOccurred())

				By("Read created secret from system")
				foundSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), foundSecret)).To(Succeed())

				issuedAtTime := fakeClock.Now()

				By("Verify labels")
				Expect(foundSecret.Labels).To(And(
					HaveKeyWithValue("issued-at-time", strconv.FormatInt(issuedAtTime.Unix(), 10)),
					HaveKeyWithValue("valid-until-time", strconv.FormatInt(issuedAtTime.Add(validity).Unix(), 10)),
				))

				By("Step the clock")
				fakeClock.Step(30 * time.Minute)

				By("Generate the same secret again w/ same validity")
				secret2, err := m.Generate(ctx, config, Validity(validity))
				Expect(err).NotTo(HaveOccurred())

				By("Read created secret from system")
				foundSecret2 := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret2), foundSecret2)).To(Succeed())

				By("Verify labels (validity should not have been changed)")
				Expect(foundSecret2.Labels).To(And(
					HaveKeyWithValue("issued-at-time", secret.Labels["issued-at-time"]),
					HaveKeyWithValue("valid-until-time", secret.Labels["valid-until-time"]),
				))

				By("Generate the same secret again w/ new validity")
				validity = 2 * time.Hour
				secret3, err := m.Generate(ctx, config, Validity(validity))
				Expect(err).NotTo(HaveOccurred())

				By("Read created secret from system")
				foundSecret3 := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret3), foundSecret3)).To(Succeed())

				By("Verify labels (validity should have been recomputed)")
				Expect(foundSecret3.Labels).To(And(
					HaveKeyWithValue("issued-at-time", secret.Labels["issued-at-time"]),
					HaveKeyWithValue("valid-until-time", strconv.FormatInt(issuedAtTime.Add(validity).Unix(), 10)),
				))

				By("Generate the same secret again w/o validity option this time")
				secret4, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())

				By("Read created secret from system")
				foundSecret4 := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret4), foundSecret4)).To(Succeed())

				By("Verify labels (validity should have been recomputed)")
				Expect(foundSecret4.Labels).To(And(
					HaveKeyWithValue("issued-at-time", secret.Labels["issued-at-time"]),
					Not(HaveKey("valid-until-time")),
				))
			})

			It("should generate a new secret when the config changes", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("Change secret config and generate again")
				config.PasswordLength = 4
				newSecret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("Verify internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should generate a new secret when the last rotation initiation time changes", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("Change last rotation initiation time and generate again")
				mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{SecretNamesToTimes: map[string]time.Time{name: time.Now()}})
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)

				newSecret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("Verify internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should store the old secret if rotation strategy is KeepOld", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("Change secret config and generate again with KeepOld strategy")
				config.PasswordLength = 4
				newSecret, err := m.Generate(ctx, config, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("Verify internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old.obj).To(Equal(secret))
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should not store the old secret even if rotation strategy is KeepOld when old secrets shall be ignored", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("Change secret config and generate again with KeepOld strategy and ignore old secrets option")
				config.PasswordLength = 4
				newSecret, err := m.Generate(ctx, config, Rotate(KeepOld), IgnoreOldSecrets())
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("Verify internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should drop the old secret if rotation strategy is KeepOld after IgnoreOldSecretsAfter has passed", func() {
				By("Generate secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("Change secret config and generating again")
				mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{})
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)

				config.PasswordLength = 4
				newSecret, err := m.Generate(ctx, config, Rotate(KeepOld), IgnoreOldSecretsAfter(time.Minute))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("Verify internal store contains both old and new secret")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old.obj).To(Equal(secret))
				Expect(secretInfos.bundle).To(BeNil())

				By("Generate secret again after given duration")
				fakeClock.Step(time.Minute)
				mgr, err = New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{})
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)

				newSecret, err = m.Generate(ctx, config, Rotate(KeepOld), IgnoreOldSecretsAfter(time.Minute))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("Verify internal store contains only new secret")
				secretInfos, found = m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should reconcile the secret", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("Mark secret as mutable")
				patch := client.MergeFrom(secret.DeepCopy())
				secret.Immutable = nil
				// ensure that label with empty value is added by another call to Generate
				delete(secret.Labels, "last-rotation-initiation-time")
				Expect(fakeClient.Patch(ctx, secret, patch)).To(Succeed())

				By("Change options and generate again")
				secret, err = m.Generate(ctx, config, Persist())
				Expect(err).NotTo(HaveOccurred())

				By("Verify labels got reconciled")
				foundSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), foundSecret)).To(Succeed())
				Expect(foundSecret.Labels).To(And(
					HaveKeyWithValue("persist", "true"),
					// ensure that label with empty value is added by another call to Generate
					HaveKeyWithValue("last-rotation-initiation-time", ""),
				))
				Expect(foundSecret.Immutable).To(PointTo(BeTrue()))
			})

			DescribeTable("should rotate secrets as configured",
				func(validity time.Duration, renewAfterValidityPercentage int, unchanged, renewed time.Duration) {
					fakeClock.SetTime(time.Now())
					options := []GenerateOption{Validity(validity), RenewAfterValidityPercentage(renewAfterValidityPercentage)}

					By("Generate new secret")
					firstSecret, err := m.Generate(ctx, config, options...)
					Expect(err).NotTo(HaveOccurred())
					expectSecretWasCreated(ctx, fakeClient, firstSecret)

					By("storing old bundle secret")
					_, found := m.getFromStore(name)
					Expect(found).To(BeTrue())

					By("some time later: no new CA should be generated")
					fakeClock.Step(unchanged)
					mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{})
					Expect(err).NotTo(HaveOccurred())
					m = mgr.(*manager)
					newSecret, err := m.Generate(ctx, config, options...)
					Expect(err).NotTo(HaveOccurred())
					Expect(newSecret.Name).To(Equal(firstSecret.Name))

					By("after expected renewal time: new secret should be generated")
					fakeClock.Step(renewed - unchanged)
					mgr, err = New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{})
					Expect(err).NotTo(HaveOccurred())
					m = mgr.(*manager)
					newSecret, err = m.Generate(ctx, config, options...)
					Expect(err).NotTo(HaveOccurred())
					Expect(newSecret.Name).NotTo(Equal(firstSecret.Name))
					expectSecretWasCreated(ctx, fakeClient, newSecret)
				},

				Entry("default 80% of 100d (=80d)", 100*24*time.Hour, 0, 79*24*time.Hour, 81*24*time.Hour),
				Entry("default 30d-10d (=20d)", 30*24*time.Hour, 0, 19*24*time.Hour, 21*24*time.Hour),
				Entry("renewAfterValidityPercentage 33% (=10d)", 30*24*time.Hour, 33, 9*24*time.Hour, 11*24*time.Hour),
				Entry("non-effective renewAfterValidityPercentage 70% (14d> default 20d-10d=10d)", 20*24*time.Hour, 70, 9*24*time.Hour, 11*24*time.Hour),
			)
		})

		Context("for CA certificate secrets", func() {
			var (
				config     *secretsutils.CertificateSecretConfig
				commonName = "my-ca-common-name"
			)

			BeforeEach(func() {
				config = &secretsutils.CertificateSecretConfig{
					Name:       name,
					CommonName: commonName,
					CertType:   secretsutils.CACert,
				}
			})

			It("should generate a new CA secret and a corresponding bundle", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Name).To(Equal(name + "-54620669"))
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("Find created bundle secret")
				secretList := &corev1.SecretList{}
				Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
					"managed-by":       "secrets-manager",
					"manager-identity": "test",
					"bundle-for":       name,
				})).To(Succeed())
				Expect(secretList.Items).To(HaveLen(1))

				By("Verify internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(secret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle.obj).To(Equal(&secretList.Items[0]))
			})

			It("should maintain the lifetime labels (w/o custom validity)", func() {
				DeferCleanup(test.WithVar(&secretsutils.Clock, fakeClock))

				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())

				By("Read created secret from system")
				foundSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), foundSecret)).To(Succeed())

				By("Verify labels")
				Expect(foundSecret.Labels).To(And(
					HaveKeyWithValue("issued-at-time", strconv.FormatInt(secretsutils.AdjustToClockSkew(fakeClock.Now()).Unix(), 10)),
					HaveKeyWithValue("valid-until-time", strconv.FormatInt(fakeClock.Now().AddDate(10, 0, 0).Unix(), 10)),
				))
			})

			It("should maintain the lifetime labels (w/ custom validity which is ignored for certificates)", func() {
				DeferCleanup(test.WithVar(&secretsutils.Clock, fakeClock))

				By("Generate new secret")
				secret, err := m.Generate(ctx, config, Validity(time.Hour))
				Expect(err).NotTo(HaveOccurred())

				By("Read created secret from system")
				foundSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), foundSecret)).To(Succeed())

				By("Verify labels")
				Expect(foundSecret.Labels).To(And(
					HaveKeyWithValue("issued-at-time", strconv.FormatInt(secretsutils.AdjustToClockSkew(fakeClock.Now()).Unix(), 10)),
					HaveKeyWithValue("valid-until-time", strconv.FormatInt(fakeClock.Now().AddDate(10, 0, 0).Unix(), 10)),
				))
			})

			It("should generate a new CA secret and use the secret name as common name", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				cert, err := secretsutils.LoadCertificate("", secret.Data["ca.key"], secret.Data["ca.crt"])
				Expect(err).NotTo(HaveOccurred())
				Expect(cert.Certificate.Subject.CommonName).To(Equal(secret.Name))
			})

			It("should generate a new CA secret and ignore the config checksum for its name", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config, IgnoreConfigChecksumForCASecretName())
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)
				Expect(secret.Name).To(Equal(name))
			})

			It("should rotate a CA secret and add old and new to the corresponding bundle", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("storing old bundle secret")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				oldBundleSecret := secretInfos.bundle.obj

				By("Change secret config and generate again")
				mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{SecretNamesToTimes: map[string]time.Time{name: time.Now()}})
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)

				newSecret, err := m.Generate(ctx, config, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("Find created bundle secret")
				secretList := &corev1.SecretList{}
				Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
					"managed-by":       "secrets-manager",
					"manager-identity": "test",
					"bundle-for":       name,
				})).To(Succeed())
				Expect(secretList.Items).To(HaveLen(2))

				By("Verify internal store reflects changes")
				secretInfos, found = m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old.obj).To(Equal(secret))
				Expect(secretInfos.bundle.obj).NotTo(PointTo(Equal(oldBundleSecret)))
			})

			DescribeTable("should rotate CA as configured",
				func(validity *time.Duration, renewAfterValidityPercentage int, unchanged, renewed time.Duration) {
					lastCommonName := config.CommonName
					config.Validity = validity
					options := []GenerateOption{Rotate(KeepOld), RenewAfterValidityPercentage(renewAfterValidityPercentage)}
					fakeClock.SetTime(time.Now())

					By("Generate new secret")
					firstSecret, err := m.Generate(ctx, config, options...)
					Expect(err).NotTo(HaveOccurred())
					expectSecretWasCreated(ctx, fakeClient, firstSecret)

					By("storing old bundle secret")
					_, found := m.getFromStore(name)
					Expect(found).To(BeTrue())

					By("some time later: no new CA should be generated")
					fakeClock.Step(unchanged)
					mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{CASecretAutoRotation: true})
					Expect(err).NotTo(HaveOccurred())
					m = mgr.(*manager)
					config.CommonName = lastCommonName
					newSecret, err := m.Generate(ctx, config, options...)
					Expect(err).NotTo(HaveOccurred())
					Expect(newSecret.Name).To(Equal(firstSecret.Name))

					By("after expected renewal time: new CA should be generated")
					fakeClock.Step(renewed - unchanged)
					mgr, err = New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{CASecretAutoRotation: true})
					Expect(err).NotTo(HaveOccurred())
					m = mgr.(*manager)
					config.CommonName = lastCommonName
					newSecret, err = m.Generate(ctx, config, options...)
					Expect(err).NotTo(HaveOccurred())
					Expect(newSecret.Name).NotTo(Equal(firstSecret.Name))
					expectSecretWasCreated(ctx, fakeClient, newSecret)

					By("Find created bundle secret")
					secretList := &corev1.SecretList{}
					Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
						"managed-by":       "secrets-manager",
						"manager-identity": "test",
						"bundle-for":       name,
					})).To(Succeed())
					Expect(secretList.Items).To(HaveLen(2))

					By("after validity of first CA")
					fakeClock.Step(*validity + 1*time.Hour - renewed)
					mgr, err = New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{CASecretAutoRotation: true})
					Expect(err).NotTo(HaveOccurred())
					m = mgr.(*manager)
					config.CommonName = lastCommonName
					newSecret2, err := m.Generate(ctx, config, options...)
					Expect(err).NotTo(HaveOccurred())
					Expect(newSecret2.Name).NotTo(Equal(newSecret.Name))

					By("Find created bundle secret")
					secretList = &corev1.SecretList{}
					Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
						"managed-by":       "secrets-manager",
						"manager-identity": "test",
						"bundle-for":       name,
					})).To(Succeed())
					Expect(secretList.Items).To(HaveLen(3))
					sort.Slice(secretList.Items, func(i, j int) bool {
						timeI, err := strconv.Atoi(secretList.Items[i].Labels[LabelKeyIssuedAtTime])
						Expect(err).NotTo(HaveOccurred())
						timeJ, err := strconv.Atoi(secretList.Items[j].Labels[LabelKeyIssuedAtTime])
						Expect(err).NotTo(HaveOccurred())
						return timeI < timeJ
					})
					By("Check removal of invalid CA")
					Expect(len(secretList.Items[0].Data["bundle.crt"])).To(BeNumerically("<", 1500), "expected bundle one CA")
					Expect(len(secretList.Items[1].Data["bundle.crt"])).To(BeNumerically(">", 1500), "expected bundle with two CAs")
					Expect(len(secretList.Items[2].Data["bundle.crt"])).To(BeNumerically("<", 1500), "expected bundle with new CA only")
				},

				Entry("default 80% of 100d (=80d)", ptr.To(100*24*time.Hour), 0, 79*24*time.Hour, 81*24*time.Hour),
				Entry("default 30d-10d (=20d)", ptr.To(30*24*time.Hour), 0, 19*24*time.Hour, 21*24*time.Hour),
				Entry("renewAfterValidityPercentage 33% (=10d)", ptr.To(30*24*time.Hour), 33, 9*24*time.Hour, 11*24*time.Hour),
				Entry("non-effective renewAfterValidityPercentage 70% (14d> default 20d-10d=10d)", ptr.To(20*24*time.Hour), 70, 9*24*time.Hour, 11*24*time.Hour),
			)
		})

		Context("for certificate secrets", func() {
			var (
				caName, serverName, clientName       = "ca", "server", "client"
				caConfig, serverConfig, clientConfig *secretsutils.CertificateSecretConfig
			)

			BeforeEach(func() {
				caConfig = &secretsutils.CertificateSecretConfig{
					Name:       caName,
					CommonName: caName,
					CertType:   secretsutils.CACert,
				}
				serverConfig = &secretsutils.CertificateSecretConfig{
					Name:                        serverName,
					CommonName:                  serverName,
					CertType:                    secretsutils.ServerCert,
					SkipPublishingCACertificate: true,
				}
				clientConfig = &secretsutils.CertificateSecretConfig{
					Name:                        clientName,
					CommonName:                  clientName,
					CertType:                    secretsutils.ClientCert,
					SkipPublishingCACertificate: true,
				}
			})

			It("should maintain the lifetime labels (w/o custom validity)", func() {
				DeferCleanup(test.WithVar(&secretsutils.Clock, fakeClock))

				By("Generate new CA secret")
				caSecret, err := m.Generate(ctx, caConfig)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, caSecret)

				By("Generate new server secret")
				serverSecret, err := m.Generate(ctx, serverConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, serverSecret)

				By("Read created secret from system")
				foundSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serverSecret), foundSecret)).To(Succeed())

				By("Verify labels")
				Expect(foundSecret.Labels).To(And(
					HaveKeyWithValue("issued-at-time", strconv.FormatInt(secretsutils.AdjustToClockSkew(fakeClock.Now()).Unix(), 10)),
					HaveKeyWithValue("valid-until-time", strconv.FormatInt(fakeClock.Now().AddDate(10, 0, 0).Unix(), 10)),
				))
			})

			It("should maintain the lifetime labels (w/ custom validity which is ignored for certificates)", func() {
				DeferCleanup(test.WithVar(&secretsutils.Clock, fakeClock))

				By("Generate new CA secret")
				caSecret, err := m.Generate(ctx, caConfig)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, caSecret)

				By("Generate new server secret")
				serverSecret, err := m.Generate(ctx, serverConfig, SignedByCA(caName), Validity(time.Hour))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, serverSecret)

				By("Read created secret from system")
				foundSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serverSecret), foundSecret)).To(Succeed())

				By("Verify labels")
				Expect(foundSecret.Labels).To(And(
					HaveKeyWithValue("issued-at-time", strconv.FormatInt(secretsutils.AdjustToClockSkew(fakeClock.Now()).Unix(), 10)),
					HaveKeyWithValue("valid-until-time", strconv.FormatInt(fakeClock.Now().AddDate(10, 0, 0).Unix(), 10)),
				))
			})

			It("should keep the same server cert even when the CA rotates", func() {
				By("Generate new CA secret")
				caSecret, err := m.Generate(ctx, caConfig)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, caSecret)

				By("Generate new server secret")
				serverSecret, err := m.Generate(ctx, serverConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, serverSecret)

				By("Rotate CA")
				mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{SecretNamesToTimes: map[string]time.Time{name: time.Now()}})
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)

				newCASecret, err := m.Generate(ctx, caConfig, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newCASecret)

				By("Get or generate server secret")
				newServerSecret, err := m.Generate(ctx, serverConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newServerSecret)

				By("Verify server secret is still the same")
				Expect(newServerSecret).To(Equal(serverSecret))
			})

			It("should regenerate the server cert when the CA rotates and the 'UseCurrentCA' option is set", func() {
				By("Generate new CA secret")
				caSecret, err := m.Generate(ctx, caConfig)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, caSecret)

				By("Generate new server secret")
				serverSecret, err := m.Generate(ctx, serverConfig, SignedByCA(caName, UseCurrentCA))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, serverSecret)

				By("Rotate CA")
				mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{SecretNamesToTimes: map[string]time.Time{caName: time.Now()}})
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)

				newCASecret, err := m.Generate(ctx, caConfig, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newCASecret)

				By("Get or generate server secret")
				newServerSecret, err := m.Generate(ctx, serverConfig, SignedByCA(caName, UseCurrentCA))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newServerSecret)

				By("Verify server secret is changed")
				Expect(newServerSecret).NotTo(Equal(serverSecret))
			})

			It("should not regenerate the client cert when the CA rotates and the 'UseOldCA' option is set", func() {
				By("Generate new CA secret")
				caSecret, err := m.Generate(ctx, caConfig)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, caSecret)

				By("Generate new client secret")
				clientSecret, err := m.Generate(ctx, clientConfig, SignedByCA(caName, UseOldCA))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, clientSecret)

				By("Rotate CA")
				mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{SecretNamesToTimes: map[string]time.Time{caName: time.Now()}})
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)

				newCASecret, err := m.Generate(ctx, caConfig, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newCASecret)

				By("Get or generate client secret")
				newClientSecret, err := m.Generate(ctx, clientConfig, SignedByCA(caName, UseOldCA))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newClientSecret)

				By("Verify client secret is not changed")
				Expect(newClientSecret).To(Equal(clientSecret))
			})

			It("should regenerate the client cert when the CA rotates", func() {
				By("Generate new CA secret")
				caSecret, err := m.Generate(ctx, caConfig)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, caSecret)

				By("Generate new client secret")
				clientSecret, err := m.Generate(ctx, clientConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, clientSecret)

				By("Rotate CA")
				mgr, err := New(ctx, logr.Discard(), fakeClock, fakeClient, namespace, identity, Config{SecretNamesToTimes: map[string]time.Time{caName: time.Now()}})
				Expect(err).NotTo(HaveOccurred())
				m = mgr.(*manager)

				newCASecret, err := m.Generate(ctx, caConfig, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newCASecret)

				By("Get or generate client secret")
				newClientSecret, err := m.Generate(ctx, clientConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newClientSecret)

				By("Verify client secret is changed")
				Expect(newClientSecret).NotTo(Equal(clientSecret))
			})

			It("should also accept ControlPlaneSecretConfigs", func() {
				DeferCleanup(test.WithVar(&secretsutils.Clock, fakeClock))

				By("Generate new CA secret")
				caSecret, err := m.Generate(ctx, caConfig)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, caSecret)

				By("Generate new control plane secret")
				serverConfig.Validity = ptr.To(1337 * time.Minute)
				controlPlaneSecretConfig := &secretsutils.ControlPlaneSecretConfig{
					Name:                    "control-plane-secret",
					CertificateSecretConfig: serverConfig,
					KubeConfigRequests: []secretsutils.KubeConfigRequest{{
						ClusterName:   namespace,
						APIServerHost: "some-host",
					}},
				}

				serverSecret, err := m.Generate(ctx, controlPlaneSecretConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, serverSecret)

				By("Verify labels")
				Expect(serverSecret.Labels).To(And(
					HaveKeyWithValue("issued-at-time", strconv.FormatInt(secretsutils.AdjustToClockSkew(fakeClock.Now()).Unix(), 10)),
					HaveKeyWithValue("valid-until-time", strconv.FormatInt(fakeClock.Now().Add(*serverConfig.Validity).Unix(), 10)),
				))
			})

			It("should correctly maintain lifetime labels for ControlPlaneSecretConfigs w/o certificate secret configs", func() {
				By("Generate new control plane secret")
				cpSecret, err := m.Generate(ctx, &secretsutils.ControlPlaneSecretConfig{Name: "control-plane-secret"})
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, cpSecret)

				By("Verify labels")
				Expect(cpSecret.Labels).To(And(
					HaveKeyWithValue("issued-at-time", strconv.FormatInt(fakeClock.Now().Unix(), 10)),
					Not(HaveKey("valid-until-time")),
				))
			})

			It("should generate a new server and client secret and keep the common name", func() {
				By("Generate new CA secret")
				caSecret, err := m.Generate(ctx, caConfig)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, caSecret)

				By("Generate new server secret")
				serverSecret, err := m.Generate(ctx, serverConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, serverSecret)

				By("Verify server certificate common name")
				serverCert, err := secretsutils.LoadCertificate("", serverSecret.Data["tls.key"], serverSecret.Data["tls.crt"])
				Expect(err).NotTo(HaveOccurred())
				Expect(serverCert.Certificate.Subject.CommonName).To(Equal(serverConfig.CommonName))

				By("Generate new client secret")
				clientSecret, err := m.Generate(ctx, clientConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, clientSecret)

				By("Verify client certificate common name")
				clientCert, err := secretsutils.LoadCertificate("", clientSecret.Data["tls.key"], clientSecret.Data["tls.crt"])
				Expect(err).NotTo(HaveOccurred())
				Expect(clientCert.Certificate.Subject.CommonName).To(Equal(clientConfig.CommonName))
			})
		})

		Context("for RSA Private Key secrets", func() {
			var config *secretsutils.RSASecretConfig

			BeforeEach(func() {
				config = &secretsutils.RSASecretConfig{
					Name: name,
					Bits: 3072,
				}
			})

			It("should generate a new RSA private key secret and a corresponding bundle", func() {
				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Name).To(Equal(name + "-c34363f0"))
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("Find created bundle secret")
				secretList := &corev1.SecretList{}
				Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
					"managed-by":       "secrets-manager",
					"manager-identity": "test",
					"bundle-for":       name,
				})).To(Succeed())
				Expect(secretList.Items).To(HaveLen(1))

				By("Verify internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(secret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle.obj).To(Equal(&secretList.Items[0]))
			})

			It("should generate a new RSA private key secret but no bundle since it's used for SSH", func() {
				config.UsedForSSH = true

				By("Generate new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Name).To(Equal(name + "-c0d53d4a"))
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("Verify internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(secret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle).To(BeNil())
			})
		})

		Context("adoption of existing secret data", func() {
			var (
				oldData = map[string][]byte{"id_rsa": []byte("some-old-data")}
				config  *secretsutils.RSASecretConfig
			)

			BeforeEach(func() {
				config = &secretsutils.RSASecretConfig{
					Name: "kube-apiserver-etcd-encryption-key",
					Bits: 4096,
				}
			})

			It("should generate a new data if no existing secrets indicates keeping the old data", func() {
				By("Generate secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())

				By("Verify new data was generated")
				Expect(secret.Data).NotTo(Equal(oldData))
			})

			It("should keep the old data if an existing secrets indicate keeping it", func() {
				By("Create existing secret with old key")
				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-existing-secret",
						Namespace: namespace,
						Labels:    map[string]string{"secrets-manager-use-data-for-name": config.GetName()},
					},
					Type: corev1.SecretTypeOpaque,
					Data: oldData,
				}
				Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

				By("Generate secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())

				By("Verify old data was kept")
				Expect(secret.Data).To(Equal(oldData))
			})

			It("should return an error if multiple existing secrets indicate keeping the old data", func() {
				By("Create existing secret with old key")
				for i := 0; i < 2; i++ {
					existingSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      fmt.Sprintf("some-existing-secret-%d", i),
							Namespace: namespace,
							Labels:    map[string]string{"secrets-manager-use-data-for-name": config.GetName()},
						},
					}
					Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed(), "for secret "+existingSecret.Name)
				}

				By("Generate secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).To(MatchError(ContainSubstring(`found more than one existing secret with "secrets-manager-use-data-for-name" label for config "kube-apiserver-etcd-encryption-key"`)))
				Expect(secret).To(BeNil())
			})
		})
	})
})

func expectSecretWasCreated(ctx context.Context, fakeClient client.Client, secret *corev1.Secret) {
	foundSecret := &corev1.Secret{}
	Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), foundSecret)).To(Succeed())

	Expect(foundSecret).To(Equal(secret))
}
