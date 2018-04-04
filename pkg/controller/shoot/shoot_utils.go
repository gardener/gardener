// Copyright 2018 The Gardener Authors.
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

package shoot

import (
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
)

// operationOngoing returns true if the .status.phase field has a value which indicates that an operation
// is still running (like creating, updating, ...), and false otherwise.
func operationOngoing(shoot *gardenv1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation
	if lastOperation == nil {
		return false
	}
	return lastOperation.State == gardenv1beta1.ShootLastOperationStateProcessing
}

func formatError(message string, err error) *gardenv1beta1.LastError {
	return &gardenv1beta1.LastError{
		Description: fmt.Sprintf("%s (%s)", message, err.Error()),
	}
}

func computeLabelsWithShootHealthiness(shoot *gardenv1beta1.Shoot, healthy bool) map[string]string {
	labels := shoot.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	if !healthy {
		labels[common.ShootUnhealthy] = "true"
	} else {
		delete(labels, common.ShootUnhealthy)
	}

	return labels
}

func computeOperationType(lastOperation *gardenv1beta1.LastOperation) gardenv1beta1.ShootLastOperationType {
	if lastOperation == nil || (lastOperation.Type == gardenv1beta1.ShootLastOperationTypeCreate && lastOperation.State != gardenv1beta1.ShootLastOperationStateSucceeded) {
		return gardenv1beta1.ShootLastOperationTypeCreate
	}
	return gardenv1beta1.ShootLastOperationTypeReconcile
}
