// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package seed

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/test/utils/rotation"
)

var _ = Describe("Seed Tests", Label("Seed", "default"), func() {
	Describe("Renew garden kubeconfig for seed", func() {
		const (
			gardenletKubeconfigSecretName      = "gardenlet-kubeconfig"
			gardenletKubeconfigSecretNamespace = "garden"
			gardenletDeploymentName            = "gardenlet"
			gardenletDeploymentNamespace       = "garden"
		)

		var (
			seed *gardencorev1beta1.Seed
		)

		BeforeEach(func() {
			// find the first seed (seed name differs between test scenarios, e.g., non-ha/ha)
			seedList := &gardencorev1beta1.SeedList{}
			Expect(testClient.List(ctx, seedList, client.Limit(1))).To(Succeed())
			seed = seedList.Items[0].DeepCopy()
		})

		It("should renew the gardenlet garden kubeconfig when triggered by annotation", func() {
			verifier := rotation.GardenletKubeconfigRotationVerifier{
				GardenReader:                       testClient,
				SeedReader:                         testClient,
				Seed:                               seed,
				GardenletKubeconfigSecretName:      gardenletKubeconfigSecretName,
				GardenletKubeconfigSecretNamespace: gardenletKubeconfigSecretNamespace,
			}

			By("Verify before state")
			verifier.Before(ctx)

			By("Trigger renewal of gardenlet garden kubeconfig")
			patch := client.MergeFrom(seed.DeepCopy())
			metav1.SetMetaDataAnnotation(&seed.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig)
			Eventually(func() error {
				return testClient.Patch(ctx, seed, patch)
			}).Should(Succeed())

			By("Wait for operation annotation to be removed from Seed")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(seed.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			}).Should(Succeed())

			By("Verify result")
			verifier.After(ctx, false)
		})
	})
})
