// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	managedresources "github.com/gardener/gardener/pkg/utils/managedresources"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedResourceName is the name of the managed resource used to deploy referenced resources to the Seed cluster.
	ManagedResourceName = "referenced-resources"
)

// DeployReferencedResources reads all referenced resources from the Garden cluster and writes a managed resource to the Seed cluster.
func (b *Botanist) DeployReferencedResources(ctx context.Context) error {
	// Read referenced objects into a slice of unstructured objects
	var unstructuredObjs []*unstructured.Unstructured
	for _, resourceRef := range b.Shoot.ResourceRefs {
		// Read the resource from the Garden cluster
		obj, err := utils.GetObjectByRef(ctx, b.K8sGardenClient.Client(), &resourceRef, b.Shoot.Info.Namespace)
		if err != nil {
			return err
		}
		if obj == nil {
			return fmt.Errorf("object not found %v", resourceRef)
		}

		// Create an unstructured object and append it to the slice
		unstructuredObj := &unstructured.Unstructured{Object: obj}
		unstructuredObj.SetNamespace(b.Shoot.SeedNamespace)
		unstructuredObj.SetName(v1beta1constants.ReferencedResourcesPrefix + unstructuredObj.GetName())
		unstructuredObjs = append(unstructuredObjs, unstructuredObj)
	}

	// Create managed resource from the slice of unstructured objects
	return managedresources.CreateManagedResourceFromUnstructured(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, ManagedResourceName,
		v1beta1constants.SeedResourceManagerClass, unstructuredObjs, false, nil)
}

// DestroyReferencedResources deletes the managed resource containing referenced resources from the Seed cluster.
func (b *Botanist) DestroyReferencedResources(ctx context.Context) error {
	return client.IgnoreNotFound(managedresources.DeleteManagedResource(ctx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, ManagedResourceName))
}
