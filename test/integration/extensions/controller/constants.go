// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package integration

const (
	// Type is the extension type for this integration test controller.
	Type = "integrationtest"

	// AnnotationKeyTimeIn is a constant for a key of an annotation in an extension object which contains the current
	// time set by the integration test itself.
	AnnotationKeyTimeIn = "time-in"
	// AnnotationKeyTimeOut is a constant for a key of an annotation in an extension object which contains the value of
	// the time-in annotation observed by the integration test controller.
	AnnotationKeyTimeOut = "time-out"
	// AnnotationKeyDesiredOperation is a constant for a key of an annotation describing the desired operation.
	AnnotationKeyDesiredOperation = "desired-operation"
	// AnnotationValueOperationDelete is a constant for a value of an annotation describing the delete operation.
	AnnotationValueOperationDelete = "delete"
	// AnnotationKeyDesiredOperationState is a constant for a key of an annotation describing the desired operation
	// state.
	AnnotationKeyDesiredOperationState = "desired-operation-state"
	// AnnotationValueDesiredOperationStateError is a constant for a value of an annotation describing the that the
	// desired operation should error.
	AnnotationValueDesiredOperationStateError = "error"
)
