// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"k8s.io/client-go/util/retry"
)

// MaintainShootAnnotations ensures that given deprecated Shoot annotations are maintained also
// with their new equivalent in the Shoot metadata.
func (b *Botanist) MaintainShootAnnotations(ctx context.Context) error {
	annotationsToAdd := make(map[string]string)

	// Maintain common.GardenCreatedByDeprecated annotation
	deprecatedValue, deprecatedExists := b.Shoot.Info.Annotations[common.GardenCreatedByDeprecated]
	_, newExists := b.Shoot.Info.Annotations[common.GardenCreatedBy]
	if deprecatedExists && !newExists {
		annotationsToAdd[common.GardenCreatedBy] = deprecatedValue
	}

	if len(annotationsToAdd) > 0 {
		if _, err := kutil.TryUpdateShootAnnotations(b.K8sGardenClient.GardenCore(), retry.DefaultRetry, b.Shoot.Info.ObjectMeta, func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			for key, value := range annotationsToAdd {
				shoot.Annotations[key] = value
			}
			return shoot, nil
		}); err != nil {
			return err
		}
	}

	return nil
}
