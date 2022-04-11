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
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencore "github.com/gardener/gardener/pkg/api/core"
	"github.com/gardener/gardener/pkg/api/extensions"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/version"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var logger = log.Log.WithName("predicate")

// HasType filters the incoming OperatingSystemConfigs for ones that have the same type
// as the given type.
func HasType(typeName string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		acc, err := extensions.Accessor(obj)
		if err != nil {
			return false
		}

		return acc.GetExtensionSpec().GetExtensionType() == typeName
	})
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

	return append(resultPredicates, predicate.Or(orPreds...))
}

// HasPurpose filters the incoming ControlPlanes for the given spec.purpose.
func HasPurpose(purpose extensionsv1alpha1.Purpose) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		controlPlane, ok := obj.(*extensionsv1alpha1.ControlPlane)
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
	})
}

// ClusterShootProviderType is a predicate for the provider type of the shoot in the cluster resource.
func ClusterShootProviderType(providerType string) predicate.Predicate {
	f := func(obj client.Object) bool {
		if obj == nil {
			return false
		}

		cluster, ok := obj.(*extensionsv1alpha1.Cluster)
		if !ok {
			return false
		}

		shoot, err := extensionscontroller.ShootFromCluster(cluster)
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
	f := func(obj client.Object) bool {
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

// ClusterShootKubernetesVersionForCSIMigrationAtLeast is a predicate for the kubernetes version of the shoot in the cluster resource.
func ClusterShootKubernetesVersionForCSIMigrationAtLeast(kubernetesVersion string) predicate.Predicate {
	f := func(obj client.Object) bool {
		if obj == nil {
			return false
		}

		cluster, ok := obj.(*extensionsv1alpha1.Cluster)
		if !ok {
			return false
		}

		shoot, err := extensionscontroller.ShootFromCluster(cluster)
		if err != nil {
			return false
		}

		kubernetesVersionForCSIMigration := kubernetesVersion
		if overwrite, ok := shoot.Annotations[extensionsv1alpha1.ShootAlphaCSIMigrationKubernetesVersion]; ok {
			kubernetesVersionForCSIMigration = overwrite
		}

		constraint, err := version.CompareVersions(shoot.Spec.Kubernetes.Version, ">=", kubernetesVersionForCSIMigration)
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
