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

package secretsrotation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// CheckRenewSeedGardenSecretsCompleted checks if renewal of garden secrets is completed for all seeds.
func CheckRenewSeedGardenSecretsCompleted(ctx context.Context, log logr.Logger, c client.Client, operationAnnotation string) error {
	seedList := &metav1.PartialObjectMetadataList{}
	seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
	if err := c.List(ctx, seedList); err != nil {
		return err
	}

	for _, seed := range seedList.Items {
		if seed.Annotations[v1beta1constants.GardenerOperation] == operationAnnotation {
			return fmt.Errorf("renewing secrets for seed %q is not completed", seed.Name)
		}
	}

	return nil
}

// RenewSeedGardenSecrets annotates all seeds to trigger renewal of their garden secrets.
func RenewSeedGardenSecrets(ctx context.Context, log logr.Logger, c client.Client, operationAnnotation string) error {
	seedList := &metav1.PartialObjectMetadataList{}
	seedList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("SeedList"))
	if err := c.List(ctx, seedList); err != nil {
		return err
	}

	log.Info("Seeds requiring renewal of their garden secrets", v1beta1constants.GardenerOperation, operationAnnotation, "number", len(seedList.Items))

	for _, seed := range seedList.Items {
		log := log.WithValues("seed", seed.Name)
		if seed.Annotations[v1beta1constants.GardenerOperation] == operationAnnotation {
			continue
		}

		if seed.Annotations[v1beta1constants.GardenerOperation] != "" {
			return fmt.Errorf("error annotating seed %s: already annotated with \"%s: %s\"", seed.Name, v1beta1constants.GardenerOperation, seed.Annotations[v1beta1constants.GardenerOperation])
		}

		seed.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))
		patch := client.MergeFrom(seed.DeepCopy())
		kubernetesutils.SetMetaDataAnnotation(&seed.ObjectMeta, v1beta1constants.GardenerOperation, operationAnnotation)
		if err := c.Patch(ctx, &seed, patch); err != nil {
			return fmt.Errorf("error annotating seed %s: %w", seed.Name, err)
		}
		log.Info("Successfully annotated seed to renew its garden secrets", v1beta1constants.GardenerOperation, operationAnnotation)
	}

	return nil
}
