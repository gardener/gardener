// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
