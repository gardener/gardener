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

package shoot

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Shoot Tests", Label("Shoot"), func() {
	f := defaultShootCreationFramework()
	f.Shoot = defaultShoot("rotate-ca-")

	It("Create Shoot, Rotate CA and Delete Shoot", Label("ca-rotation"), func() {
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()

		By("Create Shoot")
		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		By("Verify old CA secret")
		var oldCACert []byte
		Eventually(func(g Gomega) {
			secret := &corev1.Secret{}
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(f.Shoot.Name, gutil.ShootProjectSecretSuffixCACluster)}, secret)).To(Succeed())
			g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", Not(BeEmpty())))
			oldCACert = secret.Data["ca.crt"]

			verifyCABundleInKubeconfigSecret(ctx, g, f.GardenClient.Client(), client.ObjectKeyFromObject(f.Shoot), oldCACert)
		}).Should(Succeed(), "old CA cert should be synced to garden")

		By("Start CA rotation")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		patch := client.MergeFrom(f.Shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateCAStart)
		Eventually(func() error {
			return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
		}).Should(Succeed())

		Eventually(func(g Gomega) bool {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			return helper.GetShootCARotationPhase(f.Shoot.Status.Credentials) == gardencorev1beta1.RotationPreparing &&
				!metav1.HasAnnotation(f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation) &&
				time.Now().UTC().Sub(f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastInitiationTime.Time.UTC()) <= time.Minute
		}).Should(BeTrue())

		Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

		Eventually(func() error {
			return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)
		}).Should(Succeed())
		Expect(f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "ca rotation phase should be 'Prepared'")

		By("Verify CA bundle secret")
		var caBundle []byte
		Eventually(func(g Gomega) {
			secret := &corev1.Secret{}
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(f.Shoot.Name, gutil.ShootProjectSecretSuffixCACluster)}, secret)).To(Succeed())
			// For now, there is only one CA cert in the bundle, as the CA is not actually rotated yet
			// TODO: verify the old CA cert is still in there and a new is added, once the CA is actually rotated
			g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", oldCACert))
			caBundle = secret.Data["ca.crt"]

			verifyCABundleInKubeconfigSecret(ctx, g, f.GardenClient.Client(), client.ObjectKeyFromObject(f.Shoot), caBundle)
		}).Should(Succeed(), "CA bundle should be synced to garden")

		By("Complete CA rotation")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		patch = client.MergeFrom(f.Shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateCAComplete)
		Eventually(func() error {
			return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
		}).Should(Succeed())

		Eventually(func(g Gomega) bool {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			return helper.GetShootCARotationPhase(f.Shoot.Status.Credentials) == gardencorev1beta1.RotationCompleting &&
				!metav1.HasAnnotation(f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation)
		}).Should(BeTrue())

		Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

		Eventually(func(g Gomega) bool {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			return helper.GetShootCARotationPhase(f.Shoot.Status.Credentials) == gardencorev1beta1.RotationCompleted &&
				time.Now().UTC().Sub(f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTime.Time.UTC()) <= time.Minute
		}).Should(BeTrue())

		By("Verify new CA secret")
		var newCACert []byte
		Eventually(func(g Gomega) {
			secret := &corev1.Secret{}
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: gutil.ComputeShootProjectSecretName(f.Shoot.Name, gutil.ShootProjectSecretSuffixCACluster)}, secret)).To(Succeed())
			// For now, the secret will still contain the old CA cert, as the CA is not actually rotated yet
			// TODO: verify that only the new CA cert of the bundle is kept, once the CA is actually rotated
			g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", caBundle))
			newCACert = secret.Data["ca.crt"]

			verifyCABundleInKubeconfigSecret(ctx, g, f.GardenClient.Client(), client.ObjectKeyFromObject(f.Shoot), newCACert)
		}).Should(Succeed(), "new CA cert should be synced to garden")

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})

func verifyCABundleInKubeconfigSecret(ctx context.Context, g Gomega, c client.Reader, shootKey client.ObjectKey, expectedBundle []byte) {
	secret := &corev1.Secret{}
	shootKey.Name = gutil.ComputeShootProjectSecretName(shootKey.Name, gutil.ShootProjectSecretSuffixCACluster)
	g.Expect(c.Get(ctx, shootKey, secret)).To(Succeed())
	g.Expect(secret.Data).To(HaveKeyWithValue("ca.crt", expectedBundle))
}
