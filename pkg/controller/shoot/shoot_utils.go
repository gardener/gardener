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

func computeLabelsWithShootHealthiness(healthy bool) func(map[string]string) map[string]string {
	return func(existingLabels map[string]string) map[string]string {
		labels := existingLabels
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
}

func mustIgnoreShoot(annotations map[string]string, respectSyncPeriodOverwrite *bool) bool {
	_, ignore := annotations[common.ShootIgnore]
	return respectSyncPeriodOverwrite != nil && ignore && *respectSyncPeriodOverwrite
}
