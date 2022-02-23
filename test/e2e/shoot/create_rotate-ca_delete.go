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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Shoot Tests", Label("Shoot"), func() {
	f := defaultShootCreationFramework()
	f.Shoot = defaultShoot("rotate-ca-")

	It("Create Shoot, Rotate CA and Delete Shoot", Label("ca-rotation"), func() {
		By("Create Shoot")
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		By("Start CA rotation")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		Eventually(func() error {
			patch := client.MergeFrom(f.Shoot.DeepCopy())
			metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateCAStart)
			return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
		}).Should(Succeed())

		Eventually(func(g Gomega) gardencorev1beta1.ShootCredentialsRotationPhase {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			if phase := helper.GetShootCARotationPhase(f.Shoot.Status.Credentials); len(phase) > 0 {
				return phase
			}
			return ""
		}).Should(Equal(gardencorev1beta1.RotationPreparing), "ca rotation phase should be 'Preparing'")

		Eventually(func(g Gomega) bool {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			_, ok := f.Shoot.Annotations[v1beta1constants.GardenerOperation]
			return ok
		}).Should(BeFalse())

		Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

		Eventually(func() error {
			return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)
		}).Should(Succeed())
		Expect(f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared), "ca rotation phase should be 'Prepared'")

		By("Complete CA rotation")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		Eventually(func() error {
			patch := client.MergeFrom(f.Shoot.DeepCopy())
			metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateCAComplete)
			return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
		}).Should(Succeed())

		Eventually(func(g Gomega) gardencorev1beta1.ShootCredentialsRotationPhase {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			if phase := helper.GetShootCARotationPhase(f.Shoot.Status.Credentials); len(phase) > 0 {
				return phase
			}
			return ""
		}).Should(Equal(gardencorev1beta1.RotationCompleting), "ca rotation phase should be 'Completing'")

		Eventually(func(g Gomega) bool {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			_, ok := f.Shoot.Annotations[v1beta1constants.GardenerOperation]
			return ok
		}).Should(BeFalse())

		Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

		Eventually(func() error {
			return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)
		}).Should(Succeed())
		Expect(f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationCompleted))
		Expect(time.Now().UTC().Sub(f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.LastCompletionTime.Time.UTC())).To(BeNumerically("<=", time.Minute))

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})
