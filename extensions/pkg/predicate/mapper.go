// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	extensionsevent "github.com/gardener/gardener/extensions/pkg/event"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// MapperTrigger is a trigger a Mapper can react upon.
type MapperTrigger uint8

const (
	// CreateTrigger is a MapperTrigger for create events.
	CreateTrigger MapperTrigger = iota
	// UpdateOldTrigger is a MapperTrigger for update events with the old meta and object.
	UpdateOldTrigger
	// UpdateNewTrigger is a MapperTrigger for update events with the new meta and object.
	UpdateNewTrigger
	// DeleteTrigger is a MapperTrigger for delete events.
	DeleteTrigger
	// GenericTrigger is a MapperTrigger for generic events.
	GenericTrigger
)

// Mapper maps any event (in form of a GenericEvent) to a boolean whether the event shall be
// propagated or not.
type Mapper interface {
	Map(event event.GenericEvent) bool
}

// MapperFunc is a function that implements Mapper.
type MapperFunc func(event.GenericEvent) bool

// Map implements Mapper.
func (f MapperFunc) Map(event event.GenericEvent) bool {
	return f(event)
}

type mapperWithTriggers struct {
	triggers map[MapperTrigger]struct{}
	mapper   Mapper
}

// FromMapper creates a new predicate from the given Mapper that reacts on the given MapperTriggers.
func FromMapper(mapper Mapper, triggers ...MapperTrigger) predicate.Predicate {
	t := make(map[MapperTrigger]struct{})
	for _, trigger := range triggers {
		t[trigger] = struct{}{}
	}
	return &mapperWithTriggers{t, mapper}
}

// InjectFunc implements Injector.
func (m *mapperWithTriggers) InjectFunc(f inject.Func) error {
	return f(m.mapper)
}

// Create implements Predicate.
func (m *mapperWithTriggers) Create(e event.CreateEvent) bool {
	if _, ok := m.triggers[CreateTrigger]; ok {
		return m.mapper.Map(extensionsevent.NewGeneric(e.Meta, e.Object))
	}
	return true
}

// Delete implements Predicate.
func (m *mapperWithTriggers) Delete(e event.DeleteEvent) bool {
	if _, ok := m.triggers[DeleteTrigger]; ok {
		return m.mapper.Map(extensionsevent.NewGeneric(e.Meta, e.Object))
	}
	return true
}

// Update implements Predicate.
func (m *mapperWithTriggers) Update(e event.UpdateEvent) bool {
	if _, ok := m.triggers[UpdateOldTrigger]; ok {
		return m.mapper.Map(extensionsevent.NewGeneric(e.MetaOld, e.ObjectOld))
	}
	if _, ok := m.triggers[UpdateNewTrigger]; ok {
		return m.mapper.Map(extensionsevent.NewGeneric(e.MetaNew, e.ObjectNew))
	}
	return true
}

// Generic implements Predicate.
func (m *mapperWithTriggers) Generic(e event.GenericEvent) bool {
	if _, ok := m.triggers[GenericTrigger]; ok {
		return m.mapper.Map(extensionsevent.NewGeneric(e.Meta, e.Object))
	}
	return true
}
