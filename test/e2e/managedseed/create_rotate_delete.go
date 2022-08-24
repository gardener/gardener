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

package managedseed

import (
	"context"
	"fmt"
	"os"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/test/e2e"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/shoots/access"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var parentCtx context.Context

var _ = Describe("ManagedSeed Tests", Label("ManagedSeed", "default"), func() {
	BeforeEach(func() {
		parentCtx = context.Background()
	})

	f := framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: &framework.GardenerConfig{
			ProjectNamespace:   "garden",
			GardenerKubeconfig: os.Getenv("KUBECONFIG"),
			SkipAccessingShoot: true,
		},
	})
	f.Shoot = e2e.DefaultShoot("seed-")

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

		ceventually(ctx, func(g Gomega) error {
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
		ceventually(ctx, func(g Gomega) error {
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
			shootClient, err = access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "managedseed", Namespace: "garden"}})
			g.Expect(err).NotTo(HaveOccurred())
		}).Should(Succeed())

		verifier := gardenletKubeconfigRotationVerifier{gardenReader: f.GardenClient.Client(), seedReader: shootClient.Client(), seed: seed}

		By("Trigger gardenlet kubeconfig rotation by annotating ManagedSeed")
		{
			verifier.Before(ctx)
			Eventually(func() error {
				return triggerGardenletKubeconfigRotationViaManagedSeed(ctx, f.GardenClient.Client(), managedSeed)
			}).Should(Succeed())
			verifier.After(ctx)
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

		ceventually(ctx, func(g Gomega) error {
			if err := f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(seed), seed); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			var conditionMessage = fmt.Sprintf("%q condition missing", gardencorev1beta1.SeedBootstrapped)
			if condition := helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedBootstrapped); condition != nil {
				conditionMessage = condition.Message
			}

			return fmt.Errorf("seed %q is not deleted yet: %s", client.ObjectKeyFromObject(seed), conditionMessage)
		}).WithPolling(5 * time.Second).Should(Succeed())

		By("Wait for ManagedSeed to be deleted")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		ceventually(ctx, func(g Gomega) error {
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

func ceventually(ctx context.Context, actual interface{}) AsyncAssertion {
	deadline, ok := ctx.Deadline()
	if !ok {
		return Eventually(actual)
	}
	return Eventually(actual).WithTimeout(time.Until(deadline))
}

const (
	gardenletKubeconfigSecretName      = "gardenlet-kubeconfig"
	gardenletKubeconfigSecretNamespace = "garden"
)

func buildManagedSeed(shoot *gardencorev1beta1.Shoot) (*seedmanagementv1alpha1.ManagedSeed, error) {
	gardenletConfig, err := encoding.EncodeGardenletConfiguration(&gardenletconfigv1alpha1.GardenletConfiguration{
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
							Enabled: false,
						},
						OwnerChecks: &gardencorev1beta1.SeedSettingOwnerChecks{
							Enabled: false,
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
			Shoot:     &seedmanagementv1alpha1.Shoot{Name: shoot.Name},
			Gardenlet: &seedmanagementv1alpha1.Gardenlet{Config: *gardenletConfig},
		},
	}, nil
}

func triggerGardenletKubeconfigRotationViaManagedSeed(ctx context.Context, gardenClient client.Client, managedSeed *seedmanagementv1alpha1.ManagedSeed) error {
	patch := client.MergeFrom(managedSeed.DeepCopy())
	metav1.SetMetaDataAnnotation(&managedSeed.ObjectMeta, "gardener.cloud/operation", "renew-kubeconfig")
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

func (v *gardenletKubeconfigRotationVerifier) After(parentCtx context.Context) {
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	By("Verify that new gardenlet pod has taken over responsibility for seed")
	ceventually(ctx, func(g Gomega) error {
		if err := v.gardenReader.Get(ctx, client.ObjectKeyFromObject(v.seed), v.seed); err != nil {
			return err
		}

		if v.seed.Status.Gardener.Name != v.oldGardenletName {
			return nil
		}

		return fmt.Errorf("new gardenlet pod has not yet taken over responsibility for seed: %s", client.ObjectKeyFromObject(v.seed))
	}).WithPolling(5 * time.Second).Should(Succeed())

	By("Verify that gardenlet's kubeconfig secret has actually been renewed")
	ceventually(ctx, func(g Gomega) error {
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

		if newClientCertificateIssuedAt.After(v.timeBeforeRotation.UTC()) {
			return nil
		}

		return fmt.Errorf("kubeconfig secret has not yet been renewed, timeBeforeRotation: %s, newClientCertificateIssuedAt: %s", v.timeBeforeRotation, newClientCertificateIssuedAt)
	}).WithPolling(5 * time.Second).Should(Succeed())
}
