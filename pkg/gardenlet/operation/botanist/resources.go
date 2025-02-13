// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	unstructuredutils "github.com/gardener/gardener/pkg/utils/kubernetes/unstructured"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ManagedResourceName is the name of the managed resource used to deploy referenced resources to the Seed cluster.
	ManagedResourceName = "referenced-resources"
)

// DeployReferencedResources reads all referenced resources from the Garden cluster and writes a managed resource to the Seed cluster.
func (b *Botanist) DeployReferencedResources(ctx context.Context) error {
	// Read referenced objects into a slice of unstructured objects
	var unstructuredObjs []*unstructured.Unstructured
	for _, resource := range b.Shoot.GetInfo().Spec.Resources {
		// Read the resource from the Garden cluster
		obj, err := unstructuredutils.GetObjectByRef(ctx, b.GardenClient, &resource.ResourceRef, b.Shoot.GetInfo().Namespace)
		if err != nil {
			return err
		}
		if obj == nil {
			return fmt.Errorf("object not found %v", resource.ResourceRef)
		}

		obj = unstructuredutils.FilterMetadata(obj, "finalizers")

		// Create an unstructured object and append it to the slice
		unstructuredObj := &unstructured.Unstructured{Object: obj}
		unstructuredObj.SetNamespace(b.Shoot.ControlPlaneNamespace)
		unstructuredObj.SetName(v1beta1constants.ReferencedResourcesPrefix + unstructuredObj.GetName())

		// Drop unwanted annotations before copying the resource to the seed.
		// All annotations contained in the ManagedResource secret will end up in `ManagedResource.status.resources[].annotations`.
		// We don't want this to happen for the last applied annotation of secrets, which includes the secret data in plain
		// text. This would put sensitive secret data into the ManagedResource object which is probably unencrypted in etcd.
		annotations := unstructuredObj.GetAnnotations()
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
		unstructuredObj.SetAnnotations(annotations)

		unstructuredObjs = append(unstructuredObjs, unstructuredObj)
	}

	// Create managed resource from the slice of unstructured objects
	return managedresources.CreateFromUnstructured(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, ManagedResourceName,
		false, v1beta1constants.SeedResourceManagerClass, unstructuredObjs, false, nil)
}

// DestroyReferencedResources deletes the managed resource containing referenced resources from the Seed cluster.
func (b *Botanist) DestroyReferencedResources(ctx context.Context) error {
	return client.IgnoreNotFound(managedresources.Delete(ctx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, ManagedResourceName, false))
}
