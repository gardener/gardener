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
	"k8s.io/utils/ptr"
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

var _ = FDescribe("Shoot Tests", Label("Shoot", "default"), func() {
	FDescribe("Create Shoot, Change Encryption Provider Type and Delete Shoot", Label("encryption-provider-change"), func() {
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
				verifyEncryptionConfigProvider(ctx, s, true, false)
			}, SpecTimeout(2*time.Minute))

			It("Verify encrypted data can be created and read before provider change", func(ctx SpecContext) {
				rotationutils.VerifyEncryptedData(ctx, s.ShootClient, defaultEncryptedResources())
			}, SpecTimeout(2*time.Minute))

			It("Update Shoot encryption provider type to AESGCM", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
					s.Shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
						Provider: gardencorev1beta1.EncryptionProvider{
							Type: ptr.To(gardencorev1beta1.EncryptionProviderTypeAESGCM),
						},
					}
				})).Should(Succeed())
			}, SpecTimeout(time.Minute))

			ItShouldWaitForShootToBeReconciledAndHealthy(s)

			It("Verify shoot status reflects AESGCM encryption provider", func(ctx SpecContext) {
				verifyShootEncryptionStatus(ctx, s, gardencorev1beta1.EncryptionProviderTypeAESGCM)
			}, SpecTimeout(2*time.Minute))

			It("Verify encryption config uses AESGCM after rotation", func(ctx SpecContext) {
				verifyEncryptionConfigProvider(ctx, s, false, true)
			}, SpecTimeout(2*time.Minute))

			It("Verify secret created before provider change can still be read", func(ctx SpecContext) {
				rotationutils.VerifyEncryptedData(ctx, s.ShootClient, defaultEncryptedResources())
			}, SpecTimeout(2*time.Minute))

			It("Verify encrypted data can be created and read before second provider change", func(ctx SpecContext) {
				rotationutils.VerifyEncryptedData(ctx, s.ShootClient, defaultEncryptedResources())
			}, SpecTimeout(2*time.Minute))

			It("Update Shoot encryption provider type back to AESCBC", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
					s.Shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &gardencorev1beta1.EncryptionConfig{
						Provider: gardencorev1beta1.EncryptionProvider{
							Type: ptr.To(gardencorev1beta1.EncryptionProviderTypeAESCBC),
						},
					}
				})).Should(Succeed())
			}, SpecTimeout(time.Minute))

			ItShouldWaitForShootToBeReconciledAndHealthy(s)

			It("Verify shoot status reflects AESCBC encryption provider", func(ctx SpecContext) {
				verifyShootEncryptionStatus(ctx, s, gardencorev1beta1.EncryptionProviderTypeAESCBC)
			}, SpecTimeout(2*time.Minute))

			It("Verify encryption config uses AESCBC after second rotation", func(ctx SpecContext) {
				verifyEncryptionConfigProvider(ctx, s, true, false)
			}, SpecTimeout(2*time.Minute))

			It("Verify encrypted data can still be read after second provider change", func(ctx SpecContext) {
				rotationutils.VerifyEncryptedData(ctx, s.ShootClient, defaultEncryptedResources())
			}, SpecTimeout(2*time.Minute))

			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		})
	})
})

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

func verifyEncryptionConfigProvider(ctx context.Context, s *ShootContext, expectAESCBC, expectAESGCM bool) {
	encryptionConfiguration := getEncryptionConfiguration(ctx, Default, s)
	Expect(encryptionConfiguration.Resources).To(HaveLen(1))
	Expect(encryptionConfiguration.Resources[0].Providers).To(HaveLen(2))
	if expectAESCBC {
		Expect(encryptionConfiguration.Resources[0].Providers[0].AESCBC).NotTo(BeNil(), "provider should be AESCBC")
	} else {
		Expect(encryptionConfiguration.Resources[0].Providers[0].AESCBC).To(BeNil(), "provider should not be AESCBC")
	}
	if expectAESGCM {
		Expect(encryptionConfiguration.Resources[0].Providers[0].AESGCM).NotTo(BeNil(), "provider should be AESGCM")
	} else {
		Expect(encryptionConfiguration.Resources[0].Providers[0].AESGCM).To(BeNil(), "provider should not be AESGCM")
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
