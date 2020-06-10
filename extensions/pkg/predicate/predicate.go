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
	"errors"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsevent "github.com/gardener/gardener/extensions/pkg/event"
	extensionsinject "github.com/gardener/gardener/extensions/pkg/inject"
	gardencore "github.com/gardener/gardener/pkg/api/core"
	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// Log is the logger for predicates.
var Log logr.Logger = log.Log

// EvalGeneric returns true if all predicates match for the given object.
func EvalGeneric(obj runtime.Object, predicates ...predicate.Predicate) bool {
	e := extensionsevent.NewFromObject(obj)

	for _, p := range predicates {
		if !p.Generic(e) {
			return false
		}
	}

	return true
}

type shootNotFailedMapper struct {
	log logr.Logger
	extensionsinject.WithClient
	extensionsinject.WithContext
	extensionsinject.WithCache
}

func (s *shootNotFailedMapper) Map(e event.GenericEvent) bool {
	if e.Meta == nil {
		return false
	}

	// Wait for cache sync because of backing client cache.
	if !s.Cache.WaitForCacheSync(s.Context.Done()) {
		err := errors.New("failed to wait for caches to sync")
		s.log.Error(err, "Could not wait for Cache to sync", "predicate", "ShootNotFailed")
		return false
	}

	cluster, err := extensionscontroller.GetCluster(s.Context, s.Client, e.Meta.GetNamespace())
	if err != nil {
		s.log.Error(err, "Could not retrieve corresponding cluster")
		return false
	}

	if extensionscontroller.IsFailed(cluster) {
		return cluster.Shoot.Generation != cluster.Shoot.Status.ObservedGeneration
	}

	return true
}

// ShootNotFailed is a predicate for failed shoots.
func ShootNotFailed() predicate.Predicate {
	return FromMapper(&shootNotFailedMapper{log: Log.WithName("shoot-not-failed")},
		CreateTrigger, UpdateNewTrigger, DeleteTrigger, GenericTrigger)
}

type or struct {
	predicates []predicate.Predicate
}

func (o *or) orRange(f func(predicate.Predicate) bool) bool {
	for _, p := range o.predicates {
		if f(p) {
			return true
		}
	}
	return false
}

// Create implements Predicate.
func (o *or) Create(event event.CreateEvent) bool {
	return o.orRange(func(p predicate.Predicate) bool { return p.Create(event) })
}

// Delete implements Predicate.
func (o *or) Delete(event event.DeleteEvent) bool {
	return o.orRange(func(p predicate.Predicate) bool { return p.Delete(event) })
}

// Update implements Predicate.
func (o *or) Update(event event.UpdateEvent) bool {
	return o.orRange(func(p predicate.Predicate) bool { return p.Update(event) })
}

// Generic implements Predicate.
func (o *or) Generic(event event.GenericEvent) bool {
	return o.orRange(func(p predicate.Predicate) bool { return p.Generic(event) })
}

// InjectFunc implements Injector.
func (o *or) InjectFunc(f inject.Func) error {
	for _, p := range o.predicates {
		if err := f(p); err != nil {
			return err
		}
	}
	return nil
}

// Or builds a logical OR gate of passed predicates.
func Or(predicates ...predicate.Predicate) predicate.Predicate {
	return &or{predicates}
}

// HasType filters the incoming OperatingSystemConfigs for ones that have the same type
// as the given type.
func HasType(typeName string) predicate.Predicate {
	return FromMapper(MapperFunc(func(e event.GenericEvent) bool {
		acc, err := extensions.Accessor(e.Object)
		if err != nil {
			return false
		}

		return acc.GetExtensionSpec().GetExtensionType() == typeName
	}), CreateTrigger, UpdateNewTrigger, DeleteTrigger, GenericTrigger)
}

// HasName returns a predicate that matches the given name of a resource.
func HasName(name string) predicate.Predicate {
	return FromMapper(MapperFunc(func(e event.GenericEvent) bool {
		return e.Meta.GetName() == name
	}), CreateTrigger, UpdateNewTrigger, DeleteTrigger, GenericTrigger)
}

// HasOperationAnnotation is a predicate for the operation annotation.
func HasOperationAnnotation() predicate.Predicate {
	return FromMapper(MapperFunc(func(e event.GenericEvent) bool {
		return e.Meta.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationReconcile ||
			e.Meta.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore ||
			e.Meta.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationMigrate
	}), CreateTrigger, UpdateNewTrigger, GenericTrigger)
}

// LastOperationNotSuccessful is a predicate for unsuccessful last operations **only** for creation events.
func LastOperationNotSuccessful() predicate.Predicate {
	operationNotSucceeded := func(obj runtime.Object) bool {
		acc, err := extensions.Accessor(obj)
		if err != nil {
			return false
		}

		lastOp := acc.GetExtensionStatus().GetLastOperation()
		return lastOp == nil ||
			lastOp.State != gardencorev1beta1.LastOperationStateSucceeded
	}

	return predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return operationNotSucceeded(event.Object)
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return false
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return false
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			return false
		},
	}
}

// IsDeleting is a predicate for objects having a deletion timestamp.
func IsDeleting() predicate.Predicate {
	return FromMapper(MapperFunc(func(e event.GenericEvent) bool {
		return e.Meta.GetDeletionTimestamp() != nil
	}), CreateTrigger, UpdateNewTrigger, GenericTrigger)
}

// AddTypePredicate returns a new slice which contains a type predicate and the given `predicates`.
// if more than one extensionTypes is given all given types are or combined
func AddTypePredicate(predicates []predicate.Predicate, extensionTypes ...string) []predicate.Predicate {
	resultPredicates := make([]predicate.Predicate, 0, len(predicates)+1)
	resultPredicates = append(resultPredicates, predicates...)

	if len(extensionTypes) == 1 {
		resultPredicates = append(resultPredicates, HasType(extensionTypes[0]))
		return resultPredicates
	}

	orPreds := make([]predicate.Predicate, 0, len(extensionTypes))
	for _, extensionType := range extensionTypes {
		orPreds = append(orPreds, HasType(extensionType))
	}
	orPred := Or(orPreds...)

	return append(resultPredicates, orPred)
}

// HasPurpose filters the incoming Controlplanes  for the given spec.purpose
func HasPurpose(purpose extensionsv1alpha1.Purpose) predicate.Predicate {
	return FromMapper(MapperFunc(func(e event.GenericEvent) bool {
		controlPlane, ok := e.Object.(*extensionsv1alpha1.ControlPlane)
		if !ok {
			return false
		}

		// needed because ControlPlane of type "normal" has the spec.purpose field not set
		if controlPlane.Spec.Purpose == nil && purpose == extensionsv1alpha1.Normal {
			return true
		}

		if controlPlane.Spec.Purpose == nil {
			return false
		}

		return *controlPlane.Spec.Purpose == purpose
	}), CreateTrigger, UpdateNewTrigger, DeleteTrigger, GenericTrigger)
}

// ClusterShootProviderType is a predicate for the provider type of the shoot in the cluster resource.
func ClusterShootProviderType(decoder runtime.Decoder, providerType string) predicate.Predicate {
	f := func(obj runtime.Object) bool {
		if obj == nil {
			return false
		}

		cluster, ok := obj.(*extensionsv1alpha1.Cluster)
		if !ok {
			return false
		}

		shoot, err := extensionscontroller.ShootFromCluster(decoder, cluster)
		if err != nil {
			return false
		}

		return shoot.Spec.Provider.Type == providerType
	}

	return predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return f(event.Object)
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return f(event.ObjectNew)
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return f(event.Object)
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			return f(event.Object)
		},
	}
}

// GardenCoreProviderType is a predicate for the provider type of a `gardencore.Object` implementation.
func GardenCoreProviderType(providerType string) predicate.Predicate {
	f := func(obj runtime.Object) bool {
		if obj == nil {
			return false
		}

		accessor, err := gardencore.Accessor(obj)
		if err != nil {
			return false
		}

		return accessor.GetProviderType() == providerType
	}

	return predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return f(event.Object)
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return f(event.ObjectNew)
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return f(event.Object)
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			return f(event.Object)
		},
	}
}

// ClusterShootKubernetesVersionAtLeast is a predicate for the kubernetes version of the shoot in the cluster resource.
func ClusterShootKubernetesVersionAtLeast(decoder runtime.Decoder, kubernetesVersion string) predicate.Predicate {
	f := func(obj runtime.Object) bool {
		if obj == nil {
			return false
		}

		cluster, ok := obj.(*extensionsv1alpha1.Cluster)
		if !ok {
			return false
		}

		shoot, err := extensionscontroller.ShootFromCluster(decoder, cluster)
		if err != nil {
			return false
		}

		constraint, err := version.CompareVersions(shoot.Spec.Kubernetes.Version, ">=", kubernetesVersion)
		if err != nil {
			return false
		}

		return constraint
	}

	return predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return f(event.Object)
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return f(event.ObjectNew)
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return f(event.Object)
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			return f(event.Object)
		},
	}
}
