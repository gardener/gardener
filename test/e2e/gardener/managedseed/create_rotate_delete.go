// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
	"github.com/gardener/gardener/test/utils/rotation"
)

// GetSeedName returns the name of the managed seed used in this e2e test
func GetSeedName() string {
	if os.Getenv("OPERATOR_SEED") == "true" {
		return "e2e-mngdseed-op"
	}
	return "e2e-managedseed"
}

var parentCtx context.Context

var _ = Describe("ManagedSeed Tests", Label("ManagedSeed", "default"), func() {
	BeforeEach(func() {
		parentCtx = context.Background()
	})

	f := framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: e2e.DefaultGardenConfig("garden"),
	})
	f.Shoot = e2e.DefaultShoot(GetSeedName())
	f.Shoot.Namespace = "garden"

	It("Create Shoot, Create ManagedSeed, Delete ManagedSeed, Delete Shoot", func() {
		By("Create Shoot")
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()

		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		By("Create ManagedSeed")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		managedSeed, err := buildManagedSeed(f.Shoot)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			g.Expect(f.GardenClient.Client().Create(ctx, managedSeed)).To(Succeed())
		}).Should(Succeed())

		By("Wait for ManagedSeed to be registered")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		CEventually(ctx, func(g Gomega) error {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
			if err := health.CheckManagedSeed(managedSeed); err != nil {
				return fmt.Errorf("ManagedSeed is not ready yet: %w", err)
			}
			return nil
		}).WithPolling(5 * time.Second).Should(Succeed())

		By("Wait for Seed to be ready")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		seed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: managedSeed.Name}}
		CEventually(ctx, func(g Gomega) error {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
			if err := health.CheckSeed(seed, seed.Status.Gardener); err != nil {
				return fmt.Errorf("seed is not ready yet: %w", err)
			}
			return nil
		}).WithPolling(5 * time.Second).Should(Succeed())

		By("Verify gardenlet kubeconfig rotation scenarios")
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()

		var shootClient kubernetes.Interface
		Eventually(func(g Gomega) {
			var err error
			shootClient, err = access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
			g.Expect(err).NotTo(HaveOccurred())
		}).Should(Succeed())

		verifier := rotation.GardenletKubeconfigRotationVerifier{
			GardenReader:                       f.GardenClient.Client(),
			SeedReader:                         shootClient.Client(),
			Seed:                               seed,
			GardenletKubeconfigSecretName:      gardenletKubeconfigSecretName,
			GardenletKubeconfigSecretNamespace: gardenletKubeconfigSecretNamespace,
		}

		By("Trigger gardenlet kubeconfig rotation by annotating Seed")
		{
			verifier.Before(ctx)
			Eventually(func() error {
				return triggerGardenletKubeconfigRotationViaSeed(ctx, f.GardenClient.Client(), seed)
			}).Should(Succeed())
			verifier.After(ctx, false)
		}

		By("Trigger gardenlet kubeconfig rotation by annotating ManagedSeed")
		{
			verifier.Before(ctx)
			Eventually(func() error {
				return triggerGardenletKubeconfigRotationViaManagedSeed(ctx, f.GardenClient.Client(), managedSeed)
			}).Should(Succeed())
			verifier.After(ctx, true)
		}

		By("Trigger gardenlet kubeconfig rotation by annotating its kubeconfig secret")
		{
			verifier.Before(ctx)
			Eventually(func() error {
				return triggerGardenletKubeconfigRotationViaSecret(ctx, shootClient.Client())
			}).Should(Succeed())
			verifier.After(ctx, false)
		}

		By("Trigger gardenlet kubeconfig auto-rotation by reducing kubeconfig validity")
		{
			By("Scale down gardenlet deployment and wait until no gardenlet pods exist anymore")
			CEventually(ctx, func(g Gomega) []corev1.Pod {
				deployment := &appsv1.Deployment{}
				g.Expect(shootClient.Client().Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: "garden"}, deployment)).To(Succeed())

				if ptr.Deref(deployment.Spec.Replicas, 0) != 0 {
					By("Scale down gardenlet deployment to prevent interference of old pods with old validity settings")
					// See https://github.com/gardener/gardener/issues/6766 for details

					patch := client.MergeFrom(deployment.DeepCopy())
					deployment.Spec.Replicas = ptr.To[int32](0)
					g.Expect(shootClient.Client().Patch(ctx, deployment, patch)).To(Succeed())
				}

				podList := &corev1.PodList{}
				g.Expect(shootClient.Client().List(ctx, podList, client.InNamespace("garden"), client.MatchingLabels{"app": "gardener", "role": "gardenlet"})).To(Succeed())
				return podList.Items
			}).WithPolling(5 * time.Second).Should(BeEmpty())

			By("Update kubeconfig validity settings and trigger manual rotation so that gardenlet picks up new kubeconfig validity settings")
			verifier.Before(ctx)
			Eventually(func() error {
				// This configuration will cause the gardenlet to automatically renew its client certificate roughly
				// every 60s. The actual certificate is valid for 15m (even though we specify only 10m here) because
				// kube-controller-manager backdates the issued certificate, see https://github.com/kubernetes/kubernetes/blob/252935368ab67f38cb252df0a961a6dcb81d20eb/pkg/controller/certificates/signer/signer.go#L197.
				// ~40% * 15m =~ 6m. The jittering in gardenlet adds this to the time at which the certificate became
				// valid and then renews it.
				return patchGardenletKubeconfigValiditySettingsAndTriggerRotation(ctx, f.GardenClient.Client(), managedSeed, &gardenletconfigv1alpha1.KubeconfigValidity{
					Validity:                        &metav1.Duration{Duration: 10 * time.Minute},
					AutoRotationJitterPercentageMin: ptr.To[int32](40),
					AutoRotationJitterPercentageMax: ptr.To[int32](41),
				})
			}).Should(Succeed())
			verifier.After(ctx, true)

			// Now we can expect some auto-rotation happening within the next minute, so let's just wait for it.
			By("Wait for kubeconfig auto-rotation to take place")
			verifier.Before(ctx)
			verifier.After(ctx, false)

			By("Revert kubeconfig validity settings")
			Eventually(func() error {
				return patchGardenletKubeconfigValiditySettingsAndTriggerRotation(ctx, f.GardenClient.Client(), managedSeed, nil)
			}).Should(Succeed())
		}

		By("Delete ManagedSeed")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		Eventually(func(g Gomega) {
			g.Expect(client.IgnoreNotFound(f.GardenClient.Client().Delete(ctx, managedSeed))).To(Succeed())
		}).Should(Succeed())

		By("Wait for Seed to be deleted")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		CEventually(ctx, func() error {
			if err := f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(seed), seed); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			var message = "last operation missing"
			if lastOp := seed.Status.LastOperation; lastOp != nil {
				message = lastOp.Description
			}

			return fmt.Errorf("seed %q is not deleted yet: %s", client.ObjectKeyFromObject(seed), message)
		}).WithPolling(5 * time.Second).Should(Succeed())

		By("Wait for ManagedSeed to be deleted")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		CEventually(ctx, func() error {
			if err := f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			var conditionMessage = fmt.Sprintf("%q condition missing", seedmanagementv1alpha1.SeedRegistered)
			if condition := helper.GetCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.SeedRegistered); condition != nil {
				conditionMessage = condition.Message
			}

			return fmt.Errorf("ManagedSeed %q is not deleted yet: %s", client.ObjectKeyFromObject(managedSeed), conditionMessage)
		}).WithPolling(5 * time.Second).Should(Succeed())

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})

const (
	gardenletKubeconfigSecretName      = "gardenlet-kubeconfig" // #nosec G101 -- No credential.
	gardenletKubeconfigSecretNamespace = "garden"
)

func buildManagedSeed(shoot *gardencorev1beta1.Shoot) (*seedmanagementv1alpha1.ManagedSeed, error) {
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
		gardenletConfig.SeedConfig.SeedTemplate.Spec.Networks.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv6}
	}
	rawGardenletConfig, err := encoding.EncodeGardenletConfiguration(gardenletConfig)
	if err != nil {
		return nil, err
	}

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
	}, nil
}

func triggerGardenletKubeconfigRotationViaManagedSeed(ctx context.Context, gardenClient client.Client, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	if err := gardenClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed); err != nil {
		return err
	}

	patch := client.MergeFrom(managedSeed.DeepCopy())
	metav1.SetMetaDataAnnotation(&managedSeed.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig)
	return gardenClient.Patch(ctx, managedSeed, patch)
}

func triggerGardenletKubeconfigRotationViaSeed(ctx context.Context, gardenClient client.Client, seed *gardencorev1beta1.Seed) error {
	if err := gardenClient.Get(ctx, client.ObjectKeyFromObject(seed), seed); err != nil {
		return err
	}

	patch := client.MergeFrom(seed.DeepCopy())
	metav1.SetMetaDataAnnotation(&seed.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig)
	return gardenClient.Patch(ctx, seed, patch)
}

func triggerGardenletKubeconfigRotationViaSecret(ctx context.Context, seedClient client.Client) error {
	secret := &corev1.Secret{}
	if err := seedClient.Get(ctx, client.ObjectKey{Name: gardenletKubeconfigSecretName, Namespace: gardenletKubeconfigSecretNamespace}, secret); err != nil {
		return err
	}

	patch := client.MergeFrom(secret.DeepCopy())
	metav1.SetMetaDataAnnotation(&secret.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.KubeconfigSecretOperationRenew)
	return seedClient.Patch(ctx, secret, patch)
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
