// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	rotationutils "github.com/gardener/gardener/test/utils/rotation"
)

var encryptionConfigDecoder runtime.Decoder

func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(apiserverconfigv1.AddToScheme(scheme))
	encryptionConfigDecoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
}

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create Shoot, Change Encryption Provider Type and Delete Shoot", Label("encryption-provider-change"), func() {
		Context("Shoot with workers", Ordered, func() {
			var s *ShootContext

			BeforeTestSetup(func() {
				shoot := DefaultShoot("e2e-encr-chg")
				shoot.Spec.Maintenance.AutoRotation = &gardencorev1beta1.MaintenanceAutoRotation{
					Credentials: &gardencorev1beta1.MaintenanceCredentialsAutoRotation{
						ETCDEncryptionKey: &gardencorev1beta1.MaintenanceRotationConfig{
							RotationPeriod: &metav1.Duration{Duration: v1beta1constants.ETCDEncryptionKeyAutoRotationPeriod},
						},
					},
				}
				s = NewTestContext().ForShoot(shoot)
			})
			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldInitializeShootClient(s)
			ItShouldGetResponsibleSeed(s)
			seed.ItShouldInitializeSeedClient(&s.SeedContext)

			It("Verify initial encryption config uses AESCBC", func(ctx SpecContext) {
				verifyEncryptionConfigProvider(ctx, s, gardencorev1beta1.EncryptionProviderTypeAESCBC)
			}, SpecTimeout(2*time.Minute))

			It("Verify encrypted data can be created and read before provider change", func(ctx SpecContext) {
				rotationutils.VerifyEncryptedData(ctx, s.ShootClient, defaultEncryptedResources())
			}, SpecTimeout(2*time.Minute))

			itShouldChangeEncryptionProviderAndVerify(s, gardencorev1beta1.EncryptionProviderTypeAESGCM)
			itShouldChangeEncryptionProviderAndVerify(s, gardencorev1beta1.EncryptionProviderTypeSecretbox)
			itShouldChangeEncryptionProviderAndVerify(s, gardencorev1beta1.EncryptionProviderTypeAESCBC)

			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		})
	})
})

func itShouldChangeEncryptionProviderAndVerify(s *ShootContext, providerType gardencorev1beta1.EncryptionProviderType) {
	GinkgoHelper()

	It("Update Shoot encryption provider type to "+string(providerType), func(ctx SpecContext) {
		Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
			s.Shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
				Provider: gardencorev1beta1.EncryptionProvider{
					Type: new(providerType),
				},
			}
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldWaitForShootToBeReconciledAndHealthy(s)

	It("Verify shoot status reflects "+string(providerType)+" encryption provider", func(ctx SpecContext) {
		verifyShootEncryptionStatus(ctx, s, providerType)
	}, SpecTimeout(2*time.Minute))

	It("Verify encryption config uses "+string(providerType), func(ctx SpecContext) {
		verifyEncryptionConfigProvider(ctx, s, providerType)
	}, SpecTimeout(2*time.Minute))

	It("Verify encrypted data can still be read after provider change to "+string(providerType), func(ctx SpecContext) {
		rotationutils.VerifyEncryptedData(ctx, s.ShootClient, defaultEncryptedResources())
	}, SpecTimeout(2*time.Minute))
}

func getEncryptionConfiguration(ctx context.Context, g Gomega, s *ShootContext) *apiserverconfigv1.EncryptionConfiguration {
	secretList := &corev1.SecretList{}
	g.Expect(s.SeedClient.List(
		ctx,
		secretList,
		client.InNamespace(s.Shoot.Status.TechnicalID),
		client.MatchingLabels{v1beta1constants.LabelRole: v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration},
	)).To(Succeed())
	g.Expect(secretList.Items).NotTo(BeEmpty())
	sort.Sort(sort.Reverse(rotationutils.AgeSorter(secretList.Items)))

	encryptionConfiguration := &apiserverconfigv1.EncryptionConfiguration{}
	g.Expect(runtime.DecodeInto(encryptionConfigDecoder, secretList.Items[0].Data["encryption-configuration.yaml"], encryptionConfiguration)).To(Succeed())
	return encryptionConfiguration
}

func verifyShootEncryptionStatus(ctx context.Context, s *ShootContext, expectedType gardencorev1beta1.EncryptionProviderType) {
	Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(s.Shoot), s.Shoot)).To(Succeed())
	Expect(s.Shoot.Status.Credentials).NotTo(BeNil())
	Expect(s.Shoot.Status.Credentials.Rotation).NotTo(BeNil())
	Expect(s.Shoot.Status.Credentials.Rotation.ETCDEncryptionKey).NotTo(BeNil())
	Expect(s.Shoot.Status.Credentials.Rotation.ETCDEncryptionKey.Phase).To(Equal(gardencorev1beta1.RotationCompleted))
	Expect(s.Shoot.Status.Credentials.EncryptionAtRest).NotTo(BeNil())
	Expect(s.Shoot.Status.Credentials.EncryptionAtRest.Provider.Type).To(Equal(expectedType))
}

func verifyEncryptionConfigProvider(ctx context.Context, s *ShootContext, expectedProvider gardencorev1beta1.EncryptionProviderType) {
	encryptionConfiguration := getEncryptionConfiguration(ctx, Default, s)
	Expect(encryptionConfiguration.Resources).To(HaveLen(1))
	Expect(encryptionConfiguration.Resources[0].Providers).To(HaveLen(2))

	provider := encryptionConfiguration.Resources[0].Providers[0]
	switch expectedProvider {
	case gardencorev1beta1.EncryptionProviderTypeAESCBC:
		Expect(provider.AESCBC).NotTo(BeNil())
		Expect(provider.AESGCM).To(BeNil())
		Expect(provider.Secretbox).To(BeNil())
	case gardencorev1beta1.EncryptionProviderTypeAESGCM:
		Expect(provider.AESCBC).To(BeNil())
		Expect(provider.AESGCM).NotTo(BeNil())
		Expect(provider.Secretbox).To(BeNil())
	case gardencorev1beta1.EncryptionProviderTypeSecretbox:
		Expect(provider.AESCBC).To(BeNil())
		Expect(provider.AESGCM).To(BeNil())
		Expect(provider.Secretbox).NotTo(BeNil())
	}

	Expect(encryptionConfiguration.Resources[0].Providers[1].Identity).NotTo(BeNil())
}

func defaultEncryptedResources() []rotationutils.EncryptedResource {
	return []rotationutils.EncryptedResource{
		{
			NewObject: func() client.Object {
				return &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "test-encr-", Namespace: "default"},
					StringData: map[string]string{"content": "foo"},
				}
			},
			NewEmptyList: func() client.ObjectList { return &corev1.SecretList{} },
		},
	}
}
