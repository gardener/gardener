// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"crypto/rand"
	"strings"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVars(
		&secretsutils.GenerateRandomString, secretsutils.FakeGenerateRandomString,
		&secretsutils.GenerateKey, secretsutils.FakeGenerateKey,
	))
})

const testIdentity = "my-extension"

var _ = Describe("SecretsManager Extension Utils", func() {

	var (
		ctx        = context.Background()
		now        metav1.Time
		fakeClock  *testclock.FakeClock
		fakeClient client.Client

		caConfigs, secretConfigs []SecretConfigWithOptions
		certConfig, otherConfig  SecretConfigWithOptions

		sm      secretsmanager.Interface
		cluster *extensionscontroller.Cluster
	)

	BeforeEach(func() {
		now = metav1.NewTime(time.Unix(1649825730, 0))
		fakeClock = testclock.NewFakeClock(now.Time)
		fakeClient = fakeclient.NewClientBuilder().Build()

		deterministicReader := strings.NewReader(strings.Repeat("-", 10000))
		DeferCleanup(test.WithVars(
			&rand.Reader, deterministicReader,
			&secretsutils.Clock, fakeClock,
		))

		caConfigs = []SecretConfigWithOptions{
			{
				Config: &secretsutils.CertificateSecretConfig{
					Name:       "my-extension-ca",
					CommonName: "my-extension",
					CertType:   secretsutils.CACert,
				},
				Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
			},
			{
				Config: &secretsutils.CertificateSecretConfig{
					Name:       "my-extension-ca-2",
					CommonName: "my-extension-2",
					CertType:   secretsutils.CACert,
				},
			},
		}

		certConfig = SecretConfigWithOptions{
			Config: &secretsutils.CertificateSecretConfig{
				Name:       "some-server",
				CommonName: "some-server",
				CertType:   secretsutils.ServerCert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA("my-extension-ca"), secretsmanager.Persist()},
		}

		otherConfig = SecretConfigWithOptions{
			Config: &secretsutils.BasicAuthSecretConfig{
				Name:           "some-secret",
				Format:         secretsutils.BasicAuthFormatNormal,
				Username:       "admin",
				PasswordLength: 32,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
		}

		secretConfigs = append(caConfigs, certConfig, otherConfig)

		cluster = &extensionscontroller.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "shoot--garden--foo",
			},
			Shoot: &gardencorev1beta1.Shoot{},
		}
	})

	// create SecretsManager in JustBeforeEach to allow test containers to modify cluster in BeforeEach
	JustBeforeEach(func() {
		var err error

		// NB: we create a real SecretsManager here (no fake), so we indirectly test the real SecretsManager implementation as well.
		// This means, these tests might fail if the core logic is changed. This is good, because technically both packages
		// belong together, and we want to make sure that extensions using the wrapped SecretsManager do exactly the right
		// thing during CA rotation, otherwise things will be messed up.
		sm, err = SecretsManagerForCluster(ctx, logr.Discard(), fakeClock, fakeClient, cluster, testIdentity, secretConfigs)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("#secretsManager", func() {
		Context("non-CA config", func() {
			It("should pass non-cert config with unmodified options", func() {
				secret, err := sm.Generate(ctx, otherConfig.Config, otherConfig.Options...)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Labels).To(HaveKeyWithValue("persist", "true"))
			})

			It("should pass server cert config with unmodified options", func() {
				_, err := sm.Generate(ctx, caConfigs[0].Config, caConfigs[0].Options...)
				Expect(err).NotTo(HaveOccurred())

				secret, err := sm.Generate(ctx, certConfig.Config, certConfig.Options...)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Labels).To(HaveKeyWithValue("persist", "true"))
			})
		})

		Context("outside of rotation", func() {
			It("should not start rotating CA", func() {
				secret, err := sm.Generate(ctx, caConfigs[0].Config, caConfigs[0].Options...)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Labels).To(HaveKeyWithValue("persist", "true"))

				Expect(sm.Cleanup(ctx)).To(Succeed())

				expectSecretsForConfig(fakeClient, caConfigs[0].Config, "CA should not get rotated",
					"my-extension-ca-013c464d")
			})
		})

		Context("phase == Preparing", func() {
			BeforeEach(func() {
				v1beta1helper.MutateShootCARotation(cluster.Shoot, func(rotation *gardencorev1beta1.CARotation) {
					rotation.LastInitiationTime = &now
					rotation.Phase = gardencorev1beta1.RotationPreparing
				})
			})

			JustBeforeEach(func() {
				By("Create old CA secrets")
				createOldCASecrets(fakeClient, cluster, caConfigs)
			})

			It("should start rotating CA and keep old secret", func() {
				secret, err := sm.Generate(ctx, caConfigs[0].Config, caConfigs[0].Options...)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Labels).To(HaveKeyWithValue("persist", "true"))

				Expect(sm.Cleanup(ctx)).To(Succeed())

				expectSecretsForConfig(fakeClient, caConfigs[0].Config, "CA should get rotated",
					"my-extension-ca-013c464d", "my-extension-ca-013c464d-431ab",
				)
			})
		})

		Context("phase == Completing", func() {
			BeforeEach(func() {
				v1beta1helper.MutateShootCARotation(cluster.Shoot, func(rotation *gardencorev1beta1.CARotation) {
					rotation.LastInitiationTime = &now
					rotation.Phase = gardencorev1beta1.RotationCompleting
				})
			})

			JustBeforeEach(func() {
				By("Create old CA secrets")
				createOldCASecrets(fakeClient, cluster, caConfigs)
			})

			It("should complete rotating CA and cleanup old CA secret", func() {
				secret, err := sm.Generate(ctx, caConfigs[0].Config, caConfigs[0].Options...)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Labels).To(HaveKeyWithValue("persist", "true"))

				Expect(sm.Cleanup(ctx)).To(Succeed())

				expectSecretsForConfig(fakeClient, caConfigs[0].Config, "old CA secret should get cleaned up",
					"my-extension-ca-013c464d-431ab")
			})
		})
	})

	Describe("#filterCAConfigs", func() {
		It("should return a list with all CA configs", func() {
			Expect(filterCAConfigs(secretConfigs)).To(consistOfConfigs(caConfigs...))
		})
	})

	Describe("#lastSecretRotationStartTimesFromCluster", func() {
		It("should not contain any times if rotation was never triggered", func() {
			Expect(lastSecretRotationStartTimesFromCluster(cluster, secretConfigs)).To(BeEmpty())
		})

		It("should add the CA rotation initiation time for all CA configs", func() {
			v1beta1helper.MutateShootCARotation(cluster.Shoot, func(rotation *gardencorev1beta1.CARotation) {
				rotation.LastInitiationTime = &now
			})
			Expect(lastSecretRotationStartTimesFromCluster(cluster, secretConfigs)).To(MatchAllKeys(Keys{
				"my-extension-ca":   Equal(now.Time),
				"my-extension-ca-2": Equal(now.Time),
			}))
		})
	})

	Describe("#GenerateAllSecrets", func() {
		Context("outside of rotation (CA secrets don't exist yet)", func() {
			It("should create all wanted secrets", func() {
				Expect(GenerateAllSecrets(ctx, sm, secretConfigs)).To(MatchAllKeys(Keys{
					"my-extension-ca":   alwaysMatch,
					"my-extension-ca-2": alwaysMatch,
					"some-server":       alwaysMatch,
					"some-secret":       alwaysMatch,
				}))

				Expect(sm.Cleanup(ctx)).To(Succeed())

				expectSecrets(fakeClient,
					"my-extension-ca-013c464d", "my-extension-ca-bundle-d9cdd23a",
					"my-extension-ca-2-673cf9ab", "my-extension-ca-2-bundle-d54d5be6",
					"some-server-fb949f01", "some-secret-4b8f9d51")
			})
		})

		Context("CA secrets already exist", func() {
			JustBeforeEach(func() {
				By("Create old CA secrets")
				createOldCASecrets(fakeClient, cluster, caConfigs)
			})

			Context("outside of rotation", func() {
				It("should create all wanted secrets and keep existing CA secrets", func() {
					Expect(GenerateAllSecrets(ctx, sm, secretConfigs)).To(MatchAllKeys(Keys{
						"my-extension-ca":   alwaysMatch,
						"my-extension-ca-2": alwaysMatch,
						"some-server":       alwaysMatch,
						"some-secret":       alwaysMatch,
					}))

					Expect(sm.Cleanup(ctx)).To(Succeed())

					expectSecrets(fakeClient,
						"my-extension-ca-013c464d", "my-extension-ca-bundle-857457c0",
						"my-extension-ca-2-673cf9ab", "my-extension-ca-2-bundle-42155530",
						"some-server-2a71a28a", "some-secret-4b8f9d51")
				})
			})

			Context("phase == Preparing", func() {
				BeforeEach(func() {
					v1beta1helper.MutateShootCARotation(cluster.Shoot, func(rotation *gardencorev1beta1.CARotation) {
						rotation.LastInitiationTime = &now
						rotation.Phase = gardencorev1beta1.RotationPreparing
					})
				})

				It("should generate new CAs, but keep old CA and server certs", func() {
					Expect(GenerateAllSecrets(ctx, sm, secretConfigs)).To(MatchAllKeys(Keys{
						"my-extension-ca":   alwaysMatch,
						"my-extension-ca-2": alwaysMatch,
						"some-server":       alwaysMatch,
						"some-secret":       alwaysMatch,
					}))

					Expect(sm.Cleanup(ctx)).To(Succeed())

					expectSecrets(fakeClient,
						"my-extension-ca-013c464d", "my-extension-ca-013c464d-431ab", "my-extension-ca-bundle-d0017688",
						"my-extension-ca-2-673cf9ab", "my-extension-ca-2-673cf9ab-431ab", "my-extension-ca-2-bundle-d904c5b9",
						"some-server-2a71a28a", "some-secret-4b8f9d51")
				})
			})

			Context("phase == Completing", func() {
				BeforeEach(func() {
					v1beta1helper.MutateShootCARotation(cluster.Shoot, func(rotation *gardencorev1beta1.CARotation) {
						rotation.LastInitiationTime = &now
						rotation.Phase = gardencorev1beta1.RotationCompleting
					})
				})

				It("should drop old CA secrets and generate new server cert", func() {
					Expect(GenerateAllSecrets(ctx, sm, secretConfigs)).To(MatchAllKeys(Keys{
						"my-extension-ca":   alwaysMatch,
						"my-extension-ca-2": alwaysMatch,
						"some-server":       alwaysMatch,
						"some-secret":       alwaysMatch,
					}))

					Expect(sm.Cleanup(ctx)).To(Succeed())

					expectSecrets(fakeClient,
						"my-extension-ca-013c464d-431ab", "my-extension-ca-bundle-3455131c",
						"my-extension-ca-2-673cf9ab-431ab", "my-extension-ca-2-bundle-8ceaf6ac",
						"some-server-58b5baa2", "some-secret-4b8f9d51")
				})
			})
		})
	})
})

func createOldCASecrets(c client.Client, cluster *extensionscontroller.Cluster, caConfigs []SecretConfigWithOptions) {
	// For testing purposes, Generate() is faked to be deterministic.
	// Generate old CA secrets with different random reader to have different CA certs (before and after rotation),
	// otherwise server certs will not be regenerated with the new CA in this test.
	deterministicReader := strings.NewReader(strings.Repeat("#", 10000))
	defer test.WithVar(&rand.Reader, deterministicReader)()

	for _, caConfig := range caConfigs {
		secretData, err := caConfig.Config.Generate()
		Expect(err).NotTo(HaveOccurred(), caConfig.Config.GetName())
		secretMeta, err := secretsmanager.ObjectMeta(cluster.ObjectMeta.Name, testIdentity, caConfig.Config, false, "", nil, nil, nil)
		Expect(err).NotTo(HaveOccurred(), caConfig.Config.GetName())

		dataMap := secretData.SecretData()
		// dataMap["dummy"] = []byte("")

		Expect(c.Create(context.Background(), &corev1.Secret{
			ObjectMeta: secretMeta,
			Data:       dataMap,
		})).To(Succeed(), caConfig.Config.GetName())
	}
}

func consistOfConfigs(configs ...SecretConfigWithOptions) gomegatypes.GomegaMatcher {
	elements := make([]any, 0, len(configs))
	for _, config := range configs {
		elements = append(elements, config)
	}
	return ConsistOf(elements...)
}

var (
	objectIdentifier = Identifier(func(obj any) string {
		switch o := obj.(type) {
		case corev1.Secret:
			return o.GetName()
		}
		return obj.(client.Object).GetName()
	})
	alwaysMatch = And()
)

func consistOfObjects(names ...string) gomegatypes.GomegaMatcher {
	elements := make(Elements, len(names))
	for _, name := range names {
		elements[name] = alwaysMatch
	}

	return MatchAllElements(objectIdentifier, elements)
}

func expectSecrets(c client.Reader, secretNames ...string) {
	secretList := &corev1.SecretList{}
	ExpectWithOffset(1, c.List(context.Background(), secretList, client.MatchingLabels{"managed-by": "secrets-manager"})).To(Succeed())
	ExpectWithOffset(1, secretList.Items).To(consistOfObjects(secretNames...))
}

func expectSecretsForConfig(c client.Reader, config secretsutils.ConfigInterface, description string, secretNames ...string) {
	secretList := &corev1.SecretList{}
	ExpectWithOffset(1, c.List(context.Background(), secretList, client.MatchingLabels{"name": config.GetName()})).To(Succeed())
	ExpectWithOffset(1, secretList.Items).To(consistOfObjects(secretNames...), description)
}
