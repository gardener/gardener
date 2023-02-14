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

package garden

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/test/e2e/operator/garden/internal/rotation"
	rotationutils "github.com/gardener/gardener/test/utils/rotation"
)

var _ = Describe("Garden Tests", Label("Garden", "default"), func() {
	var (
		backupSecret = defaultBackupSecret()
		garden       = defaultGarden(backupSecret)
	)

	It("Create Garden, Rotate Credentials and Delete Garden", Label("credentials-rotation"), func() {
		By("Create Garden")
		ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
		defer cancel()

		Expect(runtimeClient.Create(ctx, backupSecret)).To(Succeed())
		Expect(runtimeClient.Create(ctx, garden)).To(Succeed())
		waitForGardenToBeReconciled(ctx, garden)

		DeferCleanup(func() {
			By("Delete Garden")
			ctx, cancel = context.WithTimeout(parentCtx, 5*time.Minute)
			defer cancel()

			Expect(gardenerutils.ConfirmDeletion(ctx, runtimeClient, garden)).To(Succeed())
			Expect(runtimeClient.Delete(ctx, garden)).To(Succeed())
			Expect(runtimeClient.Delete(ctx, backupSecret)).To(Succeed())
			waitForGardenToBeDeleted(ctx, garden)
			cleanupVolumes(ctx)
		})

		v := rotationutils.Verifiers{
			// basic verifiers checking secrets
			&rotation.CAVerifier{RuntimeClient: runtimeClient, Garden: garden},
		}

		DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(parentCtx, 2*time.Minute)
			defer cancel()

			v.Cleanup(ctx)
		})

		v.Before(ctx)

		By("Start credentials rotation")
		ctx, cancel = context.WithTimeout(parentCtx, 5*time.Minute)
		defer cancel()

		patch := client.MergeFrom(garden.DeepCopy())
		metav1.SetMetaDataAnnotation(&garden.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.OperationRotateCredentialsStart)
		Eventually(func() error {
			return runtimeClient.Patch(ctx, garden, patch)
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			g.Expect(garden.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			v.ExpectPreparingStatus(g)
		}).Should(Succeed())

		waitForGardenToBeReconciled(ctx, garden)

		Eventually(func() error {
			return runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
		}).Should(Succeed())

		v.AfterPrepared(ctx)

		By("Complete credentials rotation")
		ctx, cancel = context.WithTimeout(parentCtx, 5*time.Minute)
		defer cancel()

		patch = client.MergeFrom(garden.DeepCopy())
		metav1.SetMetaDataAnnotation(&garden.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.OperationRotateCredentialsComplete)
		Eventually(func() error {
			return runtimeClient.Patch(ctx, garden, patch)
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
			g.Expect(garden.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			v.ExpectCompletingStatus(g)
		}).Should(Succeed())

		waitForGardenToBeReconciled(ctx, garden)

		Eventually(func(g Gomega) {
			g.Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)).To(Succeed())
		}).Should(Succeed())

		v.AfterCompleted(ctx)
	})
})
