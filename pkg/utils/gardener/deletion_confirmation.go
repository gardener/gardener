// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ConfirmationDeletion is an annotation on a Shoot and Project resources whose value must be set to "true" in order to
	// allow deleting the resource (if the annotation is not set any DELETE request will be denied).
	ConfirmationDeletion = "confirmation.gardener.cloud/deletion"
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

	value := annotations[ConfirmationDeletion]
	if confirmed, err := strconv.ParseBool(value); err != nil || !confirmed {
		return confirmationAnnotationRequiredError()
	}
	return nil
}

// ConfirmDeletion adds Gardener's deletion confirmation and timestamp annotation to the given object and sends a PATCH
// request. It does not ignore `NotFound` errors while patching.
func ConfirmDeletion(ctx context.Context, w client.Writer, obj client.Object) error {
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	kutil.SetMetaDataAnnotation(obj, ConfirmationDeletion, "true")
	kutil.SetMetaDataAnnotation(obj, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())
	return w.Patch(ctx, obj, patch)
}

func confirmationAnnotationRequiredError() error {
	return fmt.Errorf("must have a %q annotation to delete", ConfirmationDeletion)
}
