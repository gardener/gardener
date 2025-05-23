// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// Class is a type alias for describing the class of a resource.
type Class uint8

const (
	// Runtime means that the resource belongs to the runtime cluster (seed).
	Runtime Class = iota
	// Application means that the resource belongs to the target cluster (shoot).
	Application
)

// ResourceConfig contains the configuration for a resource. More concretely, it specifies the class and a mutation
// function MutateFn which should mutate the specification of the provided object Obj.
type ResourceConfig struct {
	Obj      client.Object
	Class    Class
	MutateFn func()
}

// ResourceConfigs is a list of multiple ResourceConfig objects.
type ResourceConfigs []ResourceConfig

// AllRuntimeObjects returns all objects of class Runtime from the provided ResourceConfigs lists.
func AllRuntimeObjects(configsLists ...ResourceConfigs) []client.Object {
	return allObjectsOfClass(Runtime, configsLists...)
}

// AllApplicationObjects returns all objects of class Application from the provided ResourceConfigs lists.
func AllApplicationObjects(configsLists ...ResourceConfigs) []client.Object {
	return allObjectsOfClass(Application, configsLists...)
}

func allObjectsOfClass(class Class, configsLists ...ResourceConfigs) []client.Object {
	var out []client.Object

	for _, list := range configsLists {
		for _, o := range list {
			if o.Class == class {
				out = append(out, o.Obj)
			}
		}
	}

	return out
}

// MergeResourceConfigs merges the provided ResourceConfigs lists into a new ResourceConfigs object.
func MergeResourceConfigs(configsLists ...ResourceConfigs) ResourceConfigs {
	var length int
	for _, list := range configsLists {
		length += len(list)
	}

	out := make(ResourceConfigs, 0, length)
	for _, list := range configsLists {
		out = append(out, list...)
	}
	return out
}

// ClusterType is a type alias for a cluster type.
type ClusterType string

const (
	// ClusterTypeSeed is a constant for the 'seed' cluster type.
	ClusterTypeSeed ClusterType = "seed"
	// ClusterTypeShoot is a constant for the 'shoot' cluster type.
	ClusterTypeShoot ClusterType = "shoot"
)

// DeployResourceConfigs deploys the provided ResourceConfigs <allResources> based on the ClusterType.
// For seeds, all resources are deployed via a single ManagedResource (independent of their Class).
// For shoots, all Runtime resources are applied directly with the client while all Application resources are deployed
// via a ManagedResource.
func DeployResourceConfigs(
	ctx context.Context,
	c client.Client,
	namespace string,
	clusterType ClusterType,
	managedResourceName string,
	managedResourceLabels map[string]string,
	registry *managedresources.Registry,
	allResources ResourceConfigs,
) error {
	if clusterType == ClusterTypeSeed {
		for _, r := range allResources {
			if r.MutateFn != nil {
				r.MutateFn()
			}
			if err := registry.Add(r.Obj); err != nil {
				return err
			}
		}

		serializedObjects, err := registry.SerializedObjects()
		if err != nil {
			return err
		}

		return managedresources.CreateForSeedWithLabels(ctx, c, namespace, managedResourceName, false, managedResourceLabels, serializedObjects)
	}

	for _, r := range allResources {
		switch r.Class {
		case Application:
			if r.MutateFn != nil {
				r.MutateFn()
			}
			if err := registry.Add(r.Obj); err != nil {
				return err
			}

		case Runtime:
			if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, r.Obj, func() error {
				if r.MutateFn != nil {
					r.MutateFn()
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}

	serializedObjects, err := registry.SerializedObjects()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, c, namespace, managedResourceName, managedresources.LabelValueGardener, false, serializedObjects)
}

// DestroyResourceConfigs destroys the provided ResourceConfigs <allResources> based on the ClusterType.
// For seeds, all resources are deleted indirectly by deleting the ManagedResource.
// For shoots, all Runtime resources are deleted directly with the client while all Application resources are deleted
// indirectly by deleting the ManagedResource.
func DestroyResourceConfigs(
	ctx context.Context,
	c client.Client,
	namespace string,
	clusterType ClusterType,
	managedResourceName string,
	resourceConfigs ...ResourceConfigs,
) error {
	if clusterType == ClusterTypeSeed {
		return managedresources.DeleteForSeed(ctx, c, namespace, managedResourceName)
	}

	if err := managedresources.DeleteForShoot(ctx, c, namespace, managedResourceName); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, c, AllRuntimeObjects(resourceConfigs...)...)
}
