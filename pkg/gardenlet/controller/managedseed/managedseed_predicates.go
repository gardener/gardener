// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *Controller) filterSeed(obj, _, controller client.Object, deleted bool) bool {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}
	ms, ok := controller.(*seedmanagementv1alpha1.ManagedSeed)
	if !ok {
		return false
	}

	if ms.DeletionTimestamp != nil && deleted {
		c.log.V(1).Info("ManagedSeed is deleting and seed no longer exists", "managedSeed", client.ObjectKeyFromObject(ms), "seedName", seed.Name)
		return true
	}
	return false
}
