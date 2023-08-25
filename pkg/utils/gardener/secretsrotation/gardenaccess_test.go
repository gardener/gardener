// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secretsrotation_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
)

var _ = Describe("RenewGardenAccess", func() {
	const (
		renewGardenAccessSecrets = "renew-garden-access-secrets"
	)

	var (
		ctx    context.Context
		logger logr.Logger

		gardenClient client.Client

		seeds []gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctx = context.TODO()
		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		seeds = []gardencorev1beta1.Seed{
			{ObjectMeta: metav1.ObjectMeta{Name: "seed1"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "seed2"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "seed3"}},
		}
	})

	createSeeds := func() error {
		for _, seed := range seeds {
			if err := gardenClient.Create(ctx, &seed); err != nil {
				return err
			}
		}
		return nil
	}

	Context("#CheckRenewSeedGardenSecretsCompleted", func() {
		It("should succeed if no seed is annotated anymore", func() {
			Expect(createSeeds()).To(Succeed())

			Expect(CheckRenewSeedGardenSecretsCompleted(ctx, logger, gardenClient, renewGardenAccessSecrets)).To(Succeed())
		})

		It("should succeed if some seeds have a different `gardener.cloud/operation` annotation", func() {
			seeds[0].SetAnnotations(map[string]string{"gardener.cloud/operation": "reconcile"})
			Expect(createSeeds()).To(Succeed())

			Expect(CheckRenewSeedGardenSecretsCompleted(ctx, logger, gardenClient, renewGardenAccessSecrets)).To(Succeed())
		})

		It("should fail if some seeds are still annotated with `renew-garden-access-secrets`", func() {
			seeds[1].SetAnnotations(map[string]string{"gardener.cloud/operation": renewGardenAccessSecrets})
			Expect(createSeeds()).To(Succeed())

			Expect(CheckRenewSeedGardenSecretsCompleted(ctx, logger, gardenClient, renewGardenAccessSecrets)).To(MatchError(ContainSubstring("renewing secrets for seed \"seed2\" is not completed")))
		})
	})

	Context("#RenewSeedGardenSecrets", func() {
		It("should succeed and annotate all seeds", func() {
			Expect(createSeeds()).To(Succeed())

			Expect(RenewSeedGardenSecrets(ctx, logger, gardenClient, renewGardenAccessSecrets)).To(Succeed())

			seedList := gardencorev1beta1.SeedList{}
			Expect(gardenClient.List(ctx, &seedList)).To(Succeed())
			for _, seed := range seedList.Items {
				Expect(seed.Annotations["gardener.cloud/operation"]).To(Equal(renewGardenAccessSecrets))
			}
		})

		It("should succeed if some seeds are already annotated with `renew-garden-access-secrets`", func() {
			seeds[0].SetAnnotations(map[string]string{"gardener.cloud/operation": renewGardenAccessSecrets})
			Expect(createSeeds()).To(Succeed())

			Expect(RenewSeedGardenSecrets(ctx, logger, gardenClient, renewGardenAccessSecrets)).To(Succeed())

			seedList := gardencorev1beta1.SeedList{}
			Expect(gardenClient.List(ctx, &seedList)).To(Succeed())
			for _, seed := range seedList.Items {
				Expect(seed.Annotations["gardener.cloud/operation"]).To(Equal(renewGardenAccessSecrets))
			}
		})

		It("should fail if some seeds have a different `gardener.cloud/operation` annotation", func() {
			seeds[0].SetAnnotations(map[string]string{"gardener.cloud/operation": "reconcile"})
			Expect(createSeeds()).To(Succeed())

			Expect(RenewSeedGardenSecrets(ctx, logger, gardenClient, renewGardenAccessSecrets)).To(MatchError(ContainSubstring("error annotating seed seed1: already annotated with \"gardener.cloud/operation: reconcile\"")))
		})
	})
})
