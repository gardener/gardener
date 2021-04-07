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

package managedseedset

import (
	"k8s.io/client-go/tools/cache"

	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

func (c *Controller) managedSeedSetAdd(obj interface{}) {
	_, ok := obj.(*seedmanagementv1alpha1.ManagedSeedSet)
	if !ok {
		return
	}
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	c.managedSeedSetQueue.Add(key)
}

func (c *Controller) managedSeedSetUpdate(_, newObj interface{}) {
	set, ok := newObj.(*seedmanagementv1alpha1.ManagedSeedSet)
	if !ok {
		return
	}
	if set.Generation == set.Status.ObservedGeneration {
		return
	}
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		return
	}

	c.managedSeedSetQueue.Add(key)
}

func (c *Controller) managedSeedSetDelete(obj interface{}) {
	if _, ok := obj.(*seedmanagementv1alpha1.ManagedSeedSet); !ok {
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); !ok {
			return
		} else if _, ok := tombstone.Obj.(*seedmanagementv1alpha1.ManagedSeedSet); !ok {
			return
		}
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	c.managedSeedSetQueue.Add(key)
}
