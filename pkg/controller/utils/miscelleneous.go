// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
)

// ComputeOperationType checksthe <lastOperation> and determines whether is it is Create operation or reconcile operation
func ComputeOperationType(lastOperation *gardenv1beta1.LastOperation) gardenv1beta1.ShootLastOperationType {
	if lastOperation == nil || (lastOperation.Type == gardenv1beta1.ShootLastOperationTypeCreate && lastOperation.State != gardenv1beta1.ShootLastOperationStateSucceeded) {
		return gardenv1beta1.ShootLastOperationTypeCreate
	}
	return gardenv1beta1.ShootLastOperationTypeReconcile
}
