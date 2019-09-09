/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package resources

import (
	"github.com/gardener/controller-manager-library/pkg/logger"
	"k8s.io/client-go/tools/cache"
)

func convert(resource Interface, funcs *ResourceEventHandlerFuncs) *cache.ResourceEventHandlerFuncs {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			o, err := resource.Wrap(obj.(ObjectData))
			if err == nil {
				funcs.AddFunc(o)
			}
		},
		DeleteFunc: func(obj interface{}) {
			data, ok:= obj.(ObjectData)
			if !ok {
				stale, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					logger.Errorf("informer %q reported unknown object to be deleted (%T)", resource.Name(), obj)
					return
				}
				if stale.Obj==nil {
					logger.Errorf("informer %q reported no stale object to be deleted", resource.Name())
					return
				}
				data, ok=stale.Obj.(ObjectData)
				if !ok {
					logger.Errorf("informer %q reported unknown stale object to be deleted (%T)", resource.Name() , stale.Obj)
					return
				}
			}
			o, err := resource.Wrap(data)
			if err == nil {
				funcs.DeleteFunc(o)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			o, err := resource.Wrap(old.(ObjectData))
			if err == nil {
				n, err := resource.Wrap(new.(ObjectData))
				if err == nil {
					funcs.UpdateFunc(o, n)
				}
			}
		},
	}
}
