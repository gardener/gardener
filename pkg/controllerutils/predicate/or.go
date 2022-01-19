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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// Or returns a composite predicate that implements a logical OR of the predicates passed to it.
// TODO: This is a copy of "sigs.k8s.io/controller-runtime/pkg/predicate.Or" with the addition of the InjectFunc.
//  Delete this code once the upstream library supports the InjectFunc as well.
func Or(predicates ...predicate.Predicate) predicate.Predicate {
	return or{predicates}
}

type or struct {
	predicates []predicate.Predicate
}

func (o or) InjectFunc(f inject.Func) error {
	for _, p := range o.predicates {
		if err := f(p); err != nil {
			return err
		}
	}
	return nil
}

func (o or) Create(e event.CreateEvent) bool {
	for _, p := range o.predicates {
		if p.Create(e) {
			return true
		}
	}
	return false
}

func (o or) Update(e event.UpdateEvent) bool {
	for _, p := range o.predicates {
		if p.Update(e) {
			return true
		}
	}
	return false
}

func (o or) Delete(e event.DeleteEvent) bool {
	for _, p := range o.predicates {
		if p.Delete(e) {
			return true
		}
	}
	return false
}

func (o or) Generic(e event.GenericEvent) bool {
	for _, p := range o.predicates {
		if p.Generic(e) {
			return true
		}
	}
	return false
}
