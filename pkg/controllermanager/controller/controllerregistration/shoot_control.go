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

package controllerregistration

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

func (c *Controller) shootAdd(obj interface{}) {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}

	if shoot.Spec.SeedName == nil {
		return
	}

	c.controllerRegistrationSeedQueue.Add(*shoot.Spec.SeedName)
}

func (c *Controller) shootUpdate(oldObj, newObj interface{}) {
	oldObject, ok := oldObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}

	newObject, ok := newObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}

	if apiequality.Semantic.DeepEqual(oldObject.Spec.SeedName, newObject.Spec.SeedName) &&
		apiequality.Semantic.DeepEqual(oldObject.Spec.Provider.Workers, newObject.Spec.Provider.Workers) &&
		apiequality.Semantic.DeepEqual(oldObject.Spec.Extensions, newObject.Spec.Extensions) &&
		apiequality.Semantic.DeepEqual(oldObject.Spec.DNS, newObject.Spec.DNS) &&
		oldObject.Spec.Networking.Type == newObject.Spec.Networking.Type &&
		oldObject.Spec.Provider.Type == newObject.Spec.Provider.Type {
		return
	}

	c.shootAdd(newObj)
}

func (c *Controller) shootDelete(obj interface{}) {
	c.shootAdd(obj)
}
