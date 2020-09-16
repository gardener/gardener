// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
