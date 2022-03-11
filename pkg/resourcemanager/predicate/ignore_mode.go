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

package predicate

import (
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NotIgnoreMode returns a predicate that detects if the object has the resources.gardener.cloud/mode=Ignore annotation.
func NotIgnoreMode() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return !hasIgnoreMode(obj)
	})
}

// IgnoreModeRemoved returns a predicate that detects if resources.gardener.cloud/mode=Ignore annotation was removed
// during an update.
func IgnoreModeRemoved() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return hasIgnoreMode(e.ObjectOld) && !hasIgnoreMode(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return true
		},
	}
}

func hasIgnoreMode(obj client.Object) bool {
	return obj.GetAnnotations()[resourcesv1alpha1.Mode] == resourcesv1alpha1.ModeIgnore
}
