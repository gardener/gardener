// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/seed"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot"
	"github.com/gardener/gardener/test/utils/rotation"
)

var _ = Describe("ManagedSeed Tests", Label("ManagedSeed", "default"), Ordered, func() {
	var s *ManagedSeedContext

	BeforeTestSetup(func() {
		shoot := DefaultShoot(DefaultManagedSeedName())
		shoot.Namespace = v1beta1constants.GardenNamespace
		managedSeed := buildManagedSeed(shoot)

		s = NewTestContext().ForManagedSeed(shoot, managedSeed)
	})

	ItShouldCreateShoot(s.ShootContext)
	ItShouldWaitForShootToBeReconciledAndHealthy(s.ShootContext)
	ItShouldInitializeShootClient(s.ShootContext)
	ItShouldCreateManagedSeed(s)
	ItShouldWaitForManagedSeedToBeReady(s)
	ItShouldWaitForSeedToBeReady(s.SeedContext)

	verifier := &rotation.GardenletKubeconfigRotationVerifier{
		GardenReader:                       s.GardenClient,
		GardenletKubeconfigSecretName:      gardenletKubeconfigSecretName,
		GardenletKubeconfigSecretNamespace: gardenletKubeconfigSecretNamespace,
	}

	It("Should initialize seed fields in verifier", func() {
		verifier.SeedReader = s.ShootContext.ShootClient
		verifier.Seed = s.SeedContext.Seed
	})

	Describe("Trigger gardenlet kubeconfig rotation by annotating Seed", func() {
		itShouldVerifyGardenletKubeconfigRotation(verifier, false, func() {
			ItShouldAnnotateSeed(s.SeedContext, map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRenewKubeconfig,
			})
		})
	})

	Describe("Trigger gardenlet kubeconfig rotation by annotating ManagedSeed", func() {
		itShouldVerifyGardenletKubeconfigRotation(verifier, false, func() {
			ItShouldAnnotateManagedSeed(s, map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRenewKubeconfig,
			})
		})
	})

	Describe("Trigger gardenlet kubeconfig rotation by annotating its kubeconfig secret", func() {
		itShouldVerifyGardenletKubeconfigRotation(verifier, false, func() {
			It("Should annotate kubeconfig secret", func(ctx SpecContext) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      gardenletKubeconfigSecretName,
						Namespace: gardenletKubeconfigSecretNamespace,
					},
				}

				Eventually(ctx, s.ShootContext.ShootKomega.Update(secret, func() {
					metav1.SetMetaDataAnnotation(&secret.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.KubeconfigSecretOperationRenew)
				})).Should(Succeed())
			}, SpecTimeout(time.Minute))
		})
	})

	Describe("Trigger gardenlet kubeconfig auto-rotation by reducing kubeconfig validity", func() {
		It("Scale down gardenlet deployment", func(ctx SpecContext) {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      v1beta1constants.DeploymentNameGardenlet,
					Namespace: v1beta1constants.GardenNamespace,
				},
			}

			Eventually(ctx, s.ShootContext.ShootKomega.Update(deployment, func() {
				deployment.Spec.Replicas = ptr.To(int32(0))
			})).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("Should wait until no gardenlet pods exist anymore", func(ctx SpecContext) {
			Eventually(ctx, s.ShootContext.ShootKomega.ObjectList(&corev1.PodList{}, client.InNamespace(v1beta1constants.GardenNamespace), client.MatchingLabels{"app": "gardener", "role": "gardenlet"})).
				WithPolling(5 * time.Second).To(HaveField("Items", BeEmpty()))
		}, SpecTimeout(3*time.Minute))

		itShouldVerifyGardenletKubeconfigRotation(verifier, true, func() {
			It("Update kubeconfig validity settings and trigger manual rotation so that gardenlet picks up new kubeconfig validity settings", func(ctx SpecContext) {
				Eventually(ctx, func() error {
					// This configuration will cause the gardenlet to automatically renew its client certificate roughly
					// every 60s. The actual certificate is valid for 15m (even though we specify only 10m here) because
					// kube-controller-manager backdates the issued certificate, see https://github.com/kubernetes/kubernetes/blob/252935368ab67f38cb252df0a961a6dcb81d20eb/pkg/controller/certificates/signer/signer.go#L197.
					// ~40% * 15m =~ 6m. The jittering in gardenlet adds this to the time at which the certificate became
					// valid and then renews it.
					return patchGardenletKubeconfigValiditySettingsAndTriggerRotation(ctx, s.GardenClient, s.ManagedSeed, &gardenletconfigv1alpha1.KubeconfigValidity{
						Validity:                        &metav1.Duration{Duration: 10 * time.Minute},
						AutoRotationJitterPercentageMin: ptr.To[int32](40),
						AutoRotationJitterPercentageMax: ptr.To[int32](41),
					})
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))
		})

		// Now we can expect some auto-rotation happening within the next minute, so let's just wait for it.
		itShouldVerifyGardenletKubeconfigRotation(verifier, false, func() {})

		It("Revert kubeconfig validity settings", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				return patchGardenletKubeconfigValiditySettingsAndTriggerRotation(ctx, s.GardenClient, s.ManagedSeed, nil)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))
	})

	ItShouldDeleteManagedSeed(s)
	ItShouldWaitForSeedToBeDeleted(s.SeedContext)
	ItShouldWaitForManagedSeedToBeDeleted(s)
	ItShouldDeleteShoot(s.ShootContext)
	ItShouldWaitForShootToBeDeleted(s.ShootContext)
})

const (
	gardenletKubeconfigSecretName      = "gardenlet-kubeconfig" // #nosec G101 -- No credential.
	gardenletKubeconfigSecretNamespace = "garden"
)

func buildManagedSeed(shoot *gardencorev1beta1.Shoot) *seedmanagementv1alpha1.ManagedSeed {
	gardenletConfig := &gardenletconfigv1alpha1.GardenletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
			Kind:       "GardenletConfiguration",
		},
		GardenClientConnection: &gardenletconfigv1alpha1.GardenClientConnection{
			KubeconfigSecret: &corev1.SecretReference{
				Name:      gardenletKubeconfigSecretName,
				Namespace: gardenletKubeconfigSecretNamespace,
			},
		},
		SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
			SeedTemplate: gardencorev1beta1.SeedTemplate{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{
							Enabled: ptr.To(false),
						},
						Scheduling: &gardencorev1beta1.SeedSettingScheduling{
							Visible: false,
						},
					},
					Ingress: &gardencorev1beta1.Ingress{
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
				},
			},
		},
	}
	if os.Getenv("IPFAMILY") == "ipv6" {
		gardenletConfig.SeedConfig.Spec.Networks.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}
	}
	rawGardenletConfig, err := encoding.EncodeGardenletConfiguration(gardenletConfig)
	Expect(err).NotTo(HaveOccurred())

	return &seedmanagementv1alpha1.ManagedSeed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shoot.Name,
			Namespace: shoot.Namespace,
		},
		Spec: seedmanagementv1alpha1.ManagedSeedSpec{
			Shoot: &seedmanagementv1alpha1.Shoot{Name: shoot.Name},
			Gardenlet: seedmanagementv1alpha1.GardenletConfig{
				Config: *rawGardenletConfig,
				Deployment: &seedmanagementv1alpha1.GardenletDeployment{
					ReplicaCount: ptr.To[int32](1), // the default replicaCount is 2, however in this e2e test we don't need 2 replicas
				},
			},
		},
	}
}

func itShouldVerifyGardenletKubeconfigRotation(v *rotation.GardenletKubeconfigRotationVerifier, expectPodRestart bool, rotationFunc func()) {
	It("Should verify before", func(ctx SpecContext) {
		v.Before(ctx)
	}, SpecTimeout(time.Minute))

	rotationFunc()

	It("Should verify after", func(ctx SpecContext) {
		v.After(ctx, expectPodRestart)
	}, SpecTimeout(2*time.Minute))
}

func patchGardenletKubeconfigValiditySettingsAndTriggerRotation(
	ctx context.Context,
	gardenClient client.Client,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	kubeconfigValidity *gardenletconfigv1alpha1.KubeconfigValidity,
) error {
	if err := gardenClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed); err != nil {
		return err
	}

	gardenletConfig, err := encoding.DecodeGardenletConfiguration(&managedSeed.Spec.Gardenlet.Config, false)
	if err != nil {
		return err
	}

	if gardenletConfig.GardenClientConnection == nil {
		gardenletConfig.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{}
	}
	gardenletConfig.GardenClientConnection.KubeconfigValidity = kubeconfigValidity

	gardenletConfigRaw, err := encoding.EncodeGardenletConfiguration(gardenletConfig)
	if err != nil {
		return err
	}

	patch := client.MergeFrom(managedSeed.DeepCopy())
	managedSeed.Spec.Gardenlet.Config = *gardenletConfigRaw
	metav1.SetMetaDataAnnotation(&managedSeed.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig)
	return gardenClient.Patch(ctx, managedSeed, patch)
}
