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

package manager

import (
	"context"
	"crypto/rand"
	"strings"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVars(
		&secretutils.GenerateRandomString, secretutils.FakeGenerateRandomString,
		&secretutils.GenerateKey, secretutils.FakeGenerateKey,
	))
})

const testIdentity = "my-extension"

var _ = Describe("SecretsManager Extension Utils", func() {

	var (
		ctx        = context.Background()
		now        metav1.Time
		fakeClock  *clock.FakeClock
		fakeClient client.Client

		caConfigs, secretConfigs []SecretConfigWithOptions
		certConfig, otherConfig  SecretConfigWithOptions

		sm      secretsmanager.Interface
		cluster *extensionscontroller.Cluster
	)

	BeforeEach(func() {
		now = metav1.NewTime(time.Unix(1649825730, 0))
		fakeClock = clock.NewFakeClock(now.Time)
		fakeClient = fakeclient.NewClientBuilder().Build()

		deterministicReader := strings.NewReader(strings.Repeat("-", 10000))
		DeferCleanup(test.WithVar(&rand.Reader, deterministicReader))

		caConfigs = []SecretConfigWithOptions{
			{
				Config: &secretutils.CertificateSecretConfig{
					Name:       "my-extension-ca",
					CommonName: "my-extension",
					CertType:   secretutils.CACert,
					Clock:      fakeClock,
				},
				Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
			},
			{
				Config: &secretutils.CertificateSecretConfig{
					Name:       "my-extension-ca-2",
					CommonName: "my-extension-2",
					CertType:   secretutils.CACert,
					Clock:      fakeClock,
				},
			},
		}

		certConfig = SecretConfigWithOptions{
			Config: &secretutils.CertificateSecretConfig{
				Name:       "some-server",
				CommonName: "some-server",
				CertType:   secretutils.ServerCert,
				Clock:      fakeClock,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA("my-extension-ca"), secretsmanager.Persist()},
		}

		otherConfig = SecretConfigWithOptions{
			Config: &secretutils.BasicAuthSecretConfig{
				Name:           "some-secret",
				Format:         secretutils.BasicAuthFormatCSV,
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
					"my-extension-ca-42e0ee8a")
			})
		})

		Context("phase == Preparing", func() {
			BeforeEach(func() {
				v1beta1helper.MutateShootCARotation(cluster.Shoot, func(rotation *gardencorev1beta1.ShootCARotation) {
					rotation.LastInitiationTime = &now
					rotation.Phase = gardencorev1beta1.RotationPreparing
				})
			})

			JustBeforeEach(func() {
				By("creating old CA secrets")
				createOldCASecrets(fakeClient, cluster, caConfigs)
			})

			It("should start rotating CA and keep old secret", func() {
				secret, err := sm.Generate(ctx, caConfigs[0].Config, caConfigs[0].Options...)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Labels).To(HaveKeyWithValue("persist", "true"))

				Expect(sm.Cleanup(ctx)).To(Succeed())

				expectSecretsForConfig(fakeClient, caConfigs[0].Config, "CA should get rotated",
					"my-extension-ca-42e0ee8a", "my-extension-ca-42e0ee8a-431ab",
				)
			})
		})

		Context("phase == Completing", func() {
			BeforeEach(func() {
				v1beta1helper.MutateShootCARotation(cluster.Shoot, func(rotation *gardencorev1beta1.ShootCARotation) {
					rotation.LastInitiationTime = &now
					rotation.Phase = gardencorev1beta1.RotationCompleting
				})
			})

			JustBeforeEach(func() {
				By("creating old CA secrets")
				createOldCASecrets(fakeClient, cluster, caConfigs)
			})

			It("should complete rotating CA and cleanup old CA secret", func() {
				secret, err := sm.Generate(ctx, caConfigs[0].Config, caConfigs[0].Options...)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Labels).To(HaveKeyWithValue("persist", "true"))

				Expect(sm.Cleanup(ctx)).To(Succeed())

				expectSecretsForConfig(fakeClient, caConfigs[0].Config, "old CA secret should get cleaned up",
					"my-extension-ca-42e0ee8a-431ab")
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
			v1beta1helper.MutateShootCARotation(cluster.Shoot, func(rotation *gardencorev1beta1.ShootCARotation) {
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
					"my-extension-ca-42e0ee8a", "my-extension-ca-bundle-b8ddbc7f",
					"my-extension-ca-2-5aab76c0", "my-extension-ca-2-bundle-62b9412d",
					"some-server-4b592699", "some-secret-2583adfe")
			})
		})

		Context("CA secrets already exist", func() {
			JustBeforeEach(func() {
				By("creating old CA secrets")
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
						"my-extension-ca-42e0ee8a", "my-extension-ca-bundle-7c9e4d64",
						"my-extension-ca-2-5aab76c0", "my-extension-ca-2-bundle-97af5249",
						"some-server-ec17c27a", "some-secret-2583adfe")
				})
			})

			Context("phase == Preparing", func() {
				BeforeEach(func() {
					v1beta1helper.MutateShootCARotation(cluster.Shoot, func(rotation *gardencorev1beta1.ShootCARotation) {
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
						"my-extension-ca-42e0ee8a", "my-extension-ca-42e0ee8a-431ab", "my-extension-ca-bundle-13e1abd9",
						"my-extension-ca-2-5aab76c0", "my-extension-ca-2-5aab76c0-431ab", "my-extension-ca-2-bundle-77d9e734",
						"some-server-ec17c27a", "some-secret-2583adfe")
				})
			})

			Context("phase == Completing", func() {
				BeforeEach(func() {
					v1beta1helper.MutateShootCARotation(cluster.Shoot, func(rotation *gardencorev1beta1.ShootCARotation) {
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
						"my-extension-ca-42e0ee8a-431ab", "my-extension-ca-bundle-d0aec49c",
						"my-extension-ca-2-5aab76c0-431ab", "my-extension-ca-2-bundle-c3d8dbb5",
						"some-server-d521ae63", "some-secret-2583adfe")
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
	elements := make([]interface{}, 0, len(configs))
	for _, config := range configs {
		elements = append(elements, config)
	}
	return ConsistOf(elements...)
}

var (
	objectIdentifier = Identifier(func(obj interface{}) string {
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

func expectSecretsForConfig(c client.Reader, config secretutils.ConfigInterface, description string, secretNames ...string) {
	secretList := &corev1.SecretList{}
	ExpectWithOffset(1, c.List(context.Background(), secretList, client.MatchingLabels{"name": config.GetName()})).To(Succeed())
	ExpectWithOffset(1, secretList.Items).To(consistOfObjects(secretNames...), description)
}
