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

package health

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckExtensionObject checks if an extension Object is healthy or not.
// An extension object is healthy if
// * Its observed generation is up-to-date
// * No gardener.cloud/operation is set
// * No lastError is in the status
// * A last operation is state succeeded is present
func CheckExtensionObject(o client.Object) error {
	obj, ok := o.(extensionsv1alpha1.Object)
	if !ok {
		return fmt.Errorf("expected extensionsv1alpha1.Object but got %T", o)
	}

	status := obj.GetExtensionStatus()
	return checkExtensionObject(obj.GetGeneration(), status.GetObservedGeneration(), obj.GetAnnotations(), status.GetLastError(), status.GetLastOperation())
}

// ExtensionOperationHasBeenUpdatedSince returns a health check function that checks if an extension Object's last
// operation has been updated since `lastUpdateTime`.
func ExtensionOperationHasBeenUpdatedSince(lastUpdateTime v1.Time) Func {
	return func(o client.Object) error {
		obj, ok := o.(extensionsv1alpha1.Object)
		if !ok {
			return fmt.Errorf("expected extensionsv1alpha1.Object but got %T", o)
		}

		lastOperation := obj.GetExtensionStatus().GetLastOperation()
		if lastOperation == nil || !lastOperation.LastUpdateTime.After(lastUpdateTime.Time) {
			return fmt.Errorf("extension operation has not been updated yet")
		}
		return nil
	}
}

// checkExtensionObject checks if an extension Object is healthy or not.
func checkExtensionObject(generation int64, observedGeneration int64, annotations map[string]string, lastError *gardencorev1beta1.LastError, lastOperation *gardencorev1beta1.LastOperation) error {
	if lastError != nil {
		return v1beta1helper.NewErrorWithCodes(fmt.Errorf("error during reconciliation: %s", lastError.Description), lastError.Codes...)
	}

	if observedGeneration != generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", observedGeneration, generation)
	}

	if op, ok := annotations[v1beta1constants.GardenerOperation]; ok {
		return fmt.Errorf("gardener operation %q is not yet picked up by controller", op)
	}

	if lastOperation == nil {
		return fmt.Errorf("extension did not record a last operation yet")
	}

	if lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
		return fmt.Errorf("extension state is not succeeded but %v", lastOperation.State)
	}

	return nil
}

// CheckBackupBucket checks if an backup bucket object is healthy or not.
func CheckBackupBucket(obj client.Object) error {
	bb, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return fmt.Errorf("expected *gardencorev1beta1.BackupBucket but got %T", obj)
	}
	return checkExtensionObject(bb.Generation, bb.Status.ObservedGeneration, bb.Annotations, bb.Status.LastError, bb.Status.LastOperation)
}

// CheckBackupEntry checks if an backup entry object is healthy or not.
func CheckBackupEntry(obj client.Object) error {
	be, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return fmt.Errorf("expected *gardencorev1beta1.BackupEntry but got %T", obj)
	}
	return checkExtensionObject(be.Generation, be.Status.ObservedGeneration, be.Annotations, be.Status.LastError, be.Status.LastOperation)
}
