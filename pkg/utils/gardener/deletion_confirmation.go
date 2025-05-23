// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	// DeletionProtected is a label on CustomResourceDefinitions indicating that the deletion is protected, i.e.
	// it must be confirmed with the `confirmation.gardener.cloud/deletion=true` annotation before a `DELETE` call
	// is accepted.
	DeletionProtected = "gardener.cloud/deletion-protected"
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// CheckIfDeletionIsConfirmed returns whether the deletion of an object is confirmed or not.
func CheckIfDeletionIsConfirmed(obj client.Object) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return confirmationAnnotationRequiredError()
	}

	value := annotations[v1beta1constants.ConfirmationDeletion]
	if confirmed, err := strconv.ParseBool(value); err != nil || !confirmed {
		return confirmationAnnotationRequiredError()
	}
	return nil
}

// ConfirmDeletion adds Gardener's deletion confirmation and timestamp annotation to the given object and sends a PATCH
// request.
func ConfirmDeletion(ctx context.Context, w client.Writer, obj client.Object) error {
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	kubernetesutils.SetMetaDataAnnotation(obj, v1beta1constants.ConfirmationDeletion, "true")
	kubernetesutils.SetMetaDataAnnotation(obj, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))
	return w.Patch(ctx, obj, patch)
}

func confirmationAnnotationRequiredError() error {
	return fmt.Errorf("must have a %q annotation to delete", v1beta1constants.ConfirmationDeletion)
}
