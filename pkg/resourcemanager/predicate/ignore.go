// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

// NotIgnored returns a predicate that detects if the object has the resources.gardener.cloud/ignore=true annotation.
func NotIgnored() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return !isIgnored(obj)
	})
}

// NoLongerIgnored returns a predicate that detects if resources.gardener.cloud/ignore=true annotation was removed
// during an update.
func NoLongerIgnored() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isIgnored(e.ObjectOld) && !isIgnored(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return true
		},
	}
}

// GotMarkedAsIgnored returns a predicate that detects if resources.gardener.cloud/ignore=true annotation was added
// during an update.
func GotMarkedAsIgnored() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Add event is received on controller startup. We need to reconcile this if we haven't updated the conditions yet
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !isIgnored(e.ObjectOld) && isIgnored(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return true
		},
	}
}

func isIgnored(obj client.Object) bool {
	value, ok := obj.GetAnnotations()[resourcesv1alpha1.Ignore]
	if !ok {
		return false
	}
	truthy, _ := strconv.ParseBool(value)
	return truthy
}
