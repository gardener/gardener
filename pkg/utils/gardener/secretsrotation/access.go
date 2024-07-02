// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// RenewAccessSecretsInAllObjectsOfKind annotates all objects of the kind (e.g. seed) to trigger renewal of their access secrets.
// This function works for kinds which have implemented the "gardener.cloud/operation" annotation only.
func RenewAccessSecretsInAllObjectsOfKind(ctx context.Context, log logr.Logger, c client.Client, object client.Object, operationAnnotation string) error {
	gvk, err := c.GroupVersionKindFor(object)
	if err != nil {
		return err
	}
	gvkList := schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	}

	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(gvkList)
	if err := c.List(ctx, objMetaList); err != nil {
		return err
	}

	log.Info("Objects requiring renewal of their access secrets", "kind", gvk.Kind, v1beta1constants.GardenerOperation, operationAnnotation, "number", len(objMetaList.Items))

	for _, objMeta := range objMetaList.Items {
		log := log.WithValues("kind", gvk.Kind, "name", objMeta.Name)
		if objMeta.Annotations[v1beta1constants.GardenerOperation] == operationAnnotation {
			continue
		}

		if objMeta.Annotations[v1beta1constants.GardenerOperation] != "" {
			return fmt.Errorf("error annotating %s %s: already annotated with \"%s: %s\"", gvk.Kind, objMeta.Name, v1beta1constants.GardenerOperation, objMeta.Annotations[v1beta1constants.GardenerOperation])
		}

		objMeta.SetGroupVersionKind(gvk)
		patch := client.MergeFrom(objMeta.DeepCopy())
		kubernetesutils.SetMetaDataAnnotation(&objMeta.ObjectMeta, v1beta1constants.GardenerOperation, operationAnnotation)
		if err := c.Patch(ctx, &objMeta, patch); err != nil {
			return fmt.Errorf("error annotating %s %s: %w", gvk.Kind, objMeta.Name, err)
		}
		log.Info("Successfully annotated object to renew its access secrets", v1beta1constants.GardenerOperation, operationAnnotation)
	}

	return nil
}

// CheckIfAccessSecretsRenewalCompletedInAllObjectsOfKind checks if renewal of access secrets is completed for all objects of the kind.
// This function works for kinds which have implemented the "gardener.cloud/operation" annotation only.
func CheckIfAccessSecretsRenewalCompletedInAllObjectsOfKind(ctx context.Context, c client.Client, object client.Object, operationAnnotation string) error {
	gvk, err := c.GroupVersionKindFor(object)
	if err != nil {
		return err
	}
	gvkList := schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	}
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(gvkList)
	if err := c.List(ctx, objMetaList); err != nil {
		return err
	}

	for _, objMeta := range objMetaList.Items {
		if objMeta.Annotations[v1beta1constants.GardenerOperation] == operationAnnotation {
			return fmt.Errorf("renewing secrets for %s %q is not yet completed", gvk.Kind, objMeta.Name)
		}
	}

	return nil
}
