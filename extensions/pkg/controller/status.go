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

package controller

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LastOperation creates a new LastOperation from the given parameters.
func LastOperation(t gardencorev1beta1.LastOperationType, state gardencorev1beta1.LastOperationState, progress int32, description string) *gardencorev1beta1.LastOperation {
	return &gardencorev1beta1.LastOperation{
		LastUpdateTime: metav1.Now(),
		Type:           t,
		State:          state,
		Description:    description,
		Progress:       progress,
	}
}

// LastError creates a new LastError from the given parameters.
func LastError(description string, codes ...gardencorev1beta1.ErrorCode) *gardencorev1beta1.LastError {
	now := metav1.Now()

	return &gardencorev1beta1.LastError{
		Description:    description,
		Codes:          codes,
		LastUpdateTime: &now,
	}
}

// ReconcileSucceeded returns a LastOperation with state succeeded at 100 percent and a nil LastError.
func ReconcileSucceeded(t gardencorev1beta1.LastOperationType, description string) (*gardencorev1beta1.LastOperation, *gardencorev1beta1.LastError) {
	return LastOperation(t, gardencorev1beta1.LastOperationStateSucceeded, 100, description), nil
}

// ReconcileError returns a LastOperation with state error and a LastError with the given description and codes.
func ReconcileError(t gardencorev1beta1.LastOperationType, description string, progress int32, codes ...gardencorev1beta1.ErrorCode) (*gardencorev1beta1.LastOperation, *gardencorev1beta1.LastError) {
	return LastOperation(t, gardencorev1beta1.LastOperationStateError, progress, description), LastError(description, codes...)
}
