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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/test/e2e/shoot/internal/rotation"
)

var _ = Describe("Shoot Tests", Label("Shoot"), func() {
	f := defaultShootCreationFramework()
	f.Shoot = defaultShoot("rotate-")

	It("Create Shoot, Rotate Credentials and Delete Shoot", Label("credentials-rotation"), func() {
		ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()

		By("Create Shoot")
		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		v := rotation.Verifiers{
			&rotation.CAVerifier{ShootCreationFramework: f},
		}

		v.Before(ctx)

		By("Start credentials rotation")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		patch := client.MergeFrom(f.Shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateCredentialsStart)
		Eventually(func() error {
			return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			g.Expect(f.Shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			v.ExpectPreparingStatus(g)
		}).Should(Succeed())

		Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

		Eventually(func() error {
			return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)
		}).Should(Succeed())

		v.AfterPrepared(ctx)

		By("Complete credentials rotation")
		ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
		defer cancel()

		patch = client.MergeFrom(f.Shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateCredentialsComplete)
		Eventually(func() error {
			return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			g.Expect(f.Shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			v.ExpectCompletingStatus(g)
		}).Should(Succeed())

		Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
		}).Should(Succeed())

		v.AfterCompleted(ctx)

		By("Delete Shoot")
		ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
		defer cancel()
		Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
	})
})
