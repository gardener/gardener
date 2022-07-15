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

package worker

import (
	"context"
	"time"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// ClusterToWorkerMapper returns a mapper that returns requests for Worker whose
// referenced clusters have been modified.
func ClusterToWorkerMapper(predicates []predicate.Predicate) mapper.Mapper {
	return mapper.ClusterToObjectMapper(func() client.ObjectList { return &extensionsv1alpha1.WorkerList{} }, predicates)
}

// MachineSetToWorkerMapper returns a mapper that returns requests for Worker whose
// referenced MachineSets have been modified.
func MachineSetToWorkerMapper(predicates []predicate.Predicate) mapper.Mapper {
	return newMachineSetToObjectMapper(func() client.ObjectList { return &extensionsv1alpha1.WorkerList{} }, predicates)
}

// MachineToWorkerMapper returns a mapper that returns requests for Worker whose
// referenced Machines have been modified.
func MachineToWorkerMapper(predicates []predicate.Predicate) mapper.Mapper {
	return newMachineToObjectMapper(func() client.ObjectList { return &extensionsv1alpha1.WorkerList{} }, predicates)
}

type machineSetToObjectMapper struct {
	newObjListFunc func() client.ObjectList
	predicates     []predicate.Predicate
}

func (m *machineSetToObjectMapper) Map(ctx context.Context, _ logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	machineSet, ok := obj.(*machinev1alpha1.MachineSet)
	if !ok {
		return nil
	}

	objList := m.newObjListFunc()
	if err := reader.List(ctx, objList, client.InNamespace(machineSet.Namespace)); err != nil {
		return nil
	}

	return mapper.ObjectListToRequests(objList, func(o client.Object) bool {
		return predicateutils.EvalGeneric(o, m.predicates...)
	})
}

// newMachineSetToObjectMapper returns a mapper that returns requests for objects whose
// referenced MachineSets have been modified.
func newMachineSetToObjectMapper(newObjListFunc func() client.ObjectList, predicates []predicate.Predicate) mapper.Mapper {
	return &machineSetToObjectMapper{newObjListFunc: newObjListFunc, predicates: predicates}
}

type machineToObjectMapper struct {
	newObjListFunc func() client.ObjectList
	predicates     []predicate.Predicate
}

func (m *machineToObjectMapper) InjectFunc(f inject.Func) error {
	for _, p := range m.predicates {
		if err := f(p); err != nil {
			return err
		}
	}
	return nil
}

func (m *machineToObjectMapper) Map(ctx context.Context, _ logr.Logger, reader client.Reader, obj client.Object) []reconcile.Request {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	machine, ok := obj.(*machinev1alpha1.Machine)
	if !ok {
		return nil
	}

	objList := m.newObjListFunc()
	if err := reader.List(ctx, objList, client.InNamespace(machine.Namespace)); err != nil {
		return nil
	}

	return mapper.ObjectListToRequests(objList, func(o client.Object) bool {
		return predicateutils.EvalGeneric(o, m.predicates...)
	})
}

// newMachineToObjectMapper returns a mapper that returns requests for objects whose
// referenced Machines have been modified.
func newMachineToObjectMapper(newObjListFunc func() client.ObjectList, predicates []predicate.Predicate) mapper.Mapper {
	return &machineToObjectMapper{newObjListFunc: newObjListFunc, predicates: predicates}
}
