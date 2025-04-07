// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	unstructuredutils "github.com/gardener/gardener/pkg/utils/kubernetes/unstructured"
)

// PrepareReferencedResourcesForSeedCopy reads referenced objects prepares them for deployment to the seed cluster.
func PrepareReferencedResourcesForSeedCopy(ctx context.Context, cl client.Client, resources []gardencorev1beta1.NamedResourceReference, sourceNamespace, targetNamespace string) ([]*unstructured.Unstructured, error) {
	var unstructuredObjs []*unstructured.Unstructured

	for _, resource := range resources {
		// Read the resource from the Garden cluster
		obj, err := unstructuredutils.GetObjectByRef(ctx, cl, &resource.ResourceRef, sourceNamespace)
		if err != nil {
			return nil, err
		}
		if obj == nil {
			return nil, fmt.Errorf("object not found %v", resource.ResourceRef)
		}

		obj = unstructuredutils.FilterMetadata(obj, "finalizers")

		// Create an unstructured object and append it to the slice
		unstructuredObj := &unstructured.Unstructured{Object: obj}
		unstructuredObj.SetNamespace(targetNamespace)
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

	return unstructuredObjs, nil
}
