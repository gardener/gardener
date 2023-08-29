// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseed

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
)

var parentCtx context.Context

var _ = Describe("ManagedSeed Tests", Label("ManagedSeed", "default"), func() {
	BeforeEach(func() {
		parentCtx = context.Background()
	})

	f := framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: e2e.DefaultGardenConfig("garden"),
	})
	f.Shoot = e2e.DefaultShoot("e2e-managedseed")

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

		verifier := gardenletKubeconfigRotationVerifier{gardenReader: f.GardenClient.Client(), seedReader: shootClient.Client(), seed: seed}

		By("Trigger gardenlet kubeconfig rotation by annotating ManagedSeed")
		{
			verifier.Before(ctx)
			Eventually(func() error {
				return triggerGardenletKubeconfigRotationViaManagedSeed(ctx, f.GardenClient.Client(), managedSeed)
			}).Should(Succeed())
			verifier.After(ctx, true)
		}

		By("Trigger gardenlet kubeconfig rotation by annotating its kubeconfig secret and deleting the pod")
		{
			verifier.Before(ctx)
			Eventually(func() error {
				return triggerGardenletKubeconfigRotationViaSecret(ctx, shootClient.Client(), seed.Status.Gardener.Name)
			}).Should(Succeed())
			verifier.After(ctx, true)
		}

		By("Trigger gardenlet kubeconfig auto-rotation by reducing kubeconfig validity")
		{
			By("Scale down gardenlet deployment and wait until no gardenlet pods exist anymore")
			CEventually(ctx, func(g Gomega) []corev1.Pod {
				deployment := &appsv1.Deployment{}
				g.Expect(shootClient.Client().Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: "garden"}, deployment)).To(Succeed())

				if pointer.Int32Deref(deployment.Spec.Replicas, 0) != 0 {
					By("Scale down gardenlet deployment to prevent interference of old pods with old validity settings")
					// See https://github.com/gardener/gardener/issues/6766 for details

					patch := client.MergeFrom(deployment.DeepCopy())
					deployment.Spec.Replicas = pointer.Int32(0)
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
				return patchGardenletKubeconfigValiditySettingsAndTriggerRotation(ctx, f.GardenClient.Client(), managedSeed, &gardenletv1alpha1.KubeconfigValidity{
					Validity:                        &metav1.Duration{Duration: 10 * time.Minute},
					AutoRotationJitterPercentageMin: pointer.Int32(40),
					AutoRotationJitterPercentageMax: pointer.Int32(41),
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

		CEventually(ctx, func(g Gomega) error {
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

		CEventually(ctx, func(g Gomega) error {
			if err := f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			var conditionMessage = fmt.Sprintf("%q condition missing", seedmanagementv1alpha1.ManagedSeedSeedRegistered)
			if condition := helper.GetCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered); condition != nil {
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
	gardenletKubeconfigSecretName      = "gardenlet-kubeconfig"
	gardenletKubeconfigSecretNamespace = "garden"
)

func buildManagedSeed(shoot *gardencorev1beta1.Shoot) (*seedmanagementv1alpha1.ManagedSeed, error) {
	gardenletConfig, err := encoding.EncodeGardenletConfiguration(&gardenletv1alpha1.GardenletConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardenletv1alpha1.SchemeGroupVersion.String(),
			Kind:       "GardenletConfiguration",
		},
		GardenClientConnection: &gardenletv1alpha1.GardenClientConnection{
			KubeconfigSecret: &corev1.SecretReference{
				Name:      gardenletKubeconfigSecretName,
				Namespace: gardenletKubeconfigSecretNamespace,
			},
		},
		SeedConfig: &gardenletv1alpha1.SeedConfig{
			SeedTemplate: gardencorev1beta1.SeedTemplate{
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{
							Enabled: pointer.Bool(false),
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
	})
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
			Gardenlet: &seedmanagementv1alpha1.Gardenlet{
				Config: *gardenletConfig,
				Deployment: &seedmanagementv1alpha1.GardenletDeployment{
					ReplicaCount: pointer.Int32(1), // the default replicaCount is 2, however in this e2e test we don't need 2 replicas
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

func triggerGardenletKubeconfigRotationViaSecret(ctx context.Context, seedClient client.Client, gardenletPodName string) error {
	secret := &corev1.Secret{}
	if err := seedClient.Get(ctx, client.ObjectKey{Name: gardenletKubeconfigSecretName, Namespace: gardenletKubeconfigSecretNamespace}, secret); err != nil {
		return err
	}

	patch := client.MergeFrom(secret.DeepCopy())
	metav1.SetMetaDataAnnotation(&secret.ObjectMeta, v1beta1constants.GardenerOperation, "renew")
	if err := seedClient.Patch(ctx, secret, patch); err != nil {
		return err
	}

	return seedClient.Delete(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: gardenletPodName, Namespace: v1beta1constants.GardenNamespace}})
}

func patchGardenletKubeconfigValiditySettingsAndTriggerRotation(
	ctx context.Context,
	gardenClient client.Client,
	managedSeed *seedmanagementv1alpha1.ManagedSeed,
	kubeconfigValidity *gardenletv1alpha1.KubeconfigValidity,
) error {
	if err := gardenClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed); err != nil {
		return err
	}

	gardenletConfig, err := encoding.DecodeGardenletConfiguration(&managedSeed.Spec.Gardenlet.Config, false)
	if err != nil {
		return err
	}

	if gardenletConfig.GardenClientConnection == nil {
		gardenletConfig.GardenClientConnection = &gardenletv1alpha1.GardenClientConnection{}
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

type gardenletKubeconfigRotationVerifier struct {
	gardenReader client.Reader
	seedReader   client.Reader
	seed         *gardencorev1beta1.Seed

	timeBeforeRotation time.Time
	oldGardenletName   string
}

func (v *gardenletKubeconfigRotationVerifier) Before(ctx context.Context) {
	v.timeBeforeRotation = time.Now().UTC()

	Eventually(func(g Gomega) {
		Expect(v.gardenReader.Get(ctx, client.ObjectKeyFromObject(v.seed), v.seed)).To(Succeed())
		v.oldGardenletName = v.seed.Status.Gardener.Name
	}).Should(Succeed())
}

func (v *gardenletKubeconfigRotationVerifier) After(parentCtx context.Context, expectPodRestart bool) {
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	if expectPodRestart {
		By("Verify that new gardenlet pod has taken over responsibility for seed")
		CEventually(ctx, func(g Gomega) error {
			if err := v.gardenReader.Get(ctx, client.ObjectKeyFromObject(v.seed), v.seed); err != nil {
				return err
			}

			if v.seed.Status.Gardener.Name != v.oldGardenletName {
				return nil
			}

			return fmt.Errorf("new gardenlet pod has not yet taken over responsibility for seed: %s", client.ObjectKeyFromObject(v.seed))
		}).WithPolling(5 * time.Second).Should(Succeed())
	}

	By("Verify that gardenlet's kubeconfig secret has actually been renewed")
	CEventually(ctx, func(g Gomega) error {
		secret := &corev1.Secret{}
		if err := v.seedReader.Get(ctx, client.ObjectKey{Name: gardenletKubeconfigSecretName, Namespace: gardenletKubeconfigSecretNamespace}, secret); err != nil {
			return err
		}

		kubeconfig := &clientcmdv1.Config{}
		if _, _, err := clientcmdlatest.Codec.Decode(secret.Data["kubeconfig"], nil, kubeconfig); err != nil {
			return err
		}

		clientCertificate, err := utils.DecodeCertificate(kubeconfig.AuthInfos[0].AuthInfo.ClientCertificateData)
		if err != nil {
			return err
		}

		newClientCertificateIssuedAt := clientCertificate.NotBefore.UTC()
		// The kube-controller-manager always backdates the issued certificate by 5m, see https://github.com/kubernetes/kubernetes/blob/252935368ab67f38cb252df0a961a6dcb81d20eb/pkg/controller/certificates/signer/signer.go#L197.
		// Consequently, we add these 5m so that we can assert whether the certificate was actually issued after the
		// time we recorded before the rotation was triggered.
		newClientCertificateIssuedAt = newClientCertificateIssuedAt.Add(5 * time.Minute)

		// The newClientCertificateIssuedAt time does not contain any nanoseconds, however the v.timeBeforeRotation
		// does. This was leading to failing tests in case the new client certificate was issued at the very same second
		// like the v.timeBeforeRotation, e.g. v.timeBeforeRotation = 2022-09-02 20:12:24.058418988 +0000 UTC,
		// newClientCertificateIssuedAt = 2022-09-02 20:12:24 +0000 UTC. Hence, let's round the times down to the second
		// to avoid such discrepancies. See https://github.com/gardener/gardener/issues/6618 for more details.
		newClientCertificateIssuedAt = newClientCertificateIssuedAt.Truncate(time.Second)
		timeBeforeRotation := v.timeBeforeRotation.Truncate(time.Second)

		if newClientCertificateIssuedAt.Equal(timeBeforeRotation) || newClientCertificateIssuedAt.After(timeBeforeRotation) {
			return nil
		}

		return fmt.Errorf("kubeconfig secret has not yet been renewed, timeBeforeRotation: %s, newClientCertificateIssuedAt: %s", v.timeBeforeRotation, newClientCertificateIssuedAt)
	}).WithPolling(5 * time.Second).Should(Succeed())
}
