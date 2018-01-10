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
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// operationFinal returns true if the .status.lastOperation.state field has a value which indicates
// a final state (like succeeded, failed, ...), and false otherwise.
func operationFinal(shoot *gardenv1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation
	if lastOperation == nil {
		return false
	}
	return lastOperation.State == gardenv1beta1.ShootLastOperationStateSucceeded || lastOperation.State == gardenv1beta1.ShootLastOperationStateError || lastOperation.State == gardenv1beta1.ShootLastOperationStateFailed
}

// operationStarted returns true if the .status.phase field switched to an ongoing state.
func operationStarted(old, new *gardenv1beta1.Shoot) bool {
	return (operationFinal(old) || old.Status.LastOperation == nil) && operationOngoing(new)
}

// operationEnded returns true if the .status.phase field switched to a final state.
func operationEnded(old, new *gardenv1beta1.Shoot) bool {
	return operationOngoing(old) && operationFinal(new)
}

// checkConfirmationDeletionTimestamp checks whether an annotation with the key of the constant <common.ConfirmationDeletionTimestamp>
// variable exists on the provided <shoot> object. In that case, it returns true, otherwise false.
func checkConfirmationDeletionTimestamp(shoot *gardenv1beta1.Shoot) bool {
	return metav1.HasAnnotation(shoot.ObjectMeta, common.ConfirmationDeletionTimestamp)
}

// checkConfirmationDeletionTimestampValid checks whether an annotation with the key of the constant <common.ConfirmationDeletionTimestamp>
// variable exists on the provided <shoot> object and if yes, whether its value is equal to the Shoot's
// '.metadata.deletionTimestamp' value. In that case, it returns true, otherwise false.
func checkConfirmationDeletionTimestampValid(shoot *gardenv1beta1.Shoot) bool {
	deletionTimestamp := shoot.ObjectMeta.DeletionTimestamp
	if !checkConfirmationDeletionTimestamp(shoot) || deletionTimestamp == nil {
		return false
	}
	timestamp, err := time.Parse(time.RFC3339, shoot.ObjectMeta.Annotations[common.ConfirmationDeletionTimestamp])
	if err != nil {
		return false
	}
	confirmationDeletionTimestamp := metav1.NewTime(timestamp)
	return confirmationDeletionTimestamp.Equal(deletionTimestamp)
}

// checkIfShootStillOperated checks whether another (older version) Pod of the Gardener in the <gardenerNamespace>
// does still exist (its name must match <oldGardener.Name>). It returns true if <currentGardener.Name> equals
// <oldGardener.Name> AND <currentGardener.ID> equals <oldGardener.ID> or if it found a Pod with <oldGardener.Name>. Otherwise
// it will return false. In order to perform the check, it requires a Kubernetes client <k8sClient>.
func checkIfShootStillOperated(k8sClient kubernetes.Client, gardenerNamespace string, currentGardener, oldGardener *gardenv1beta1.Gardener) bool {
	if currentGardener.Name == oldGardener.Name {
		if currentGardener.ID == oldGardener.ID {
			return true
		}
		return false
	}
	_, err := k8sClient.GetPod(gardenerNamespace, oldGardener.Name)
	if err != nil && apierrors.IsNotFound(err) {
		return false
	}
	return true
}

func formatError(message string, err error) *gardenv1beta1.LastError {
	return &gardenv1beta1.LastError{
		Description: fmt.Sprintf("%s (%s)", message, err.Error()),
	}
}

// shootUpdateValidationRequired checks whether it must be verified that no forbidden fields have been changed on update events.
func shootUpdateValidationRequired(lastOperation *gardenv1beta1.LastOperation) bool {
	if lastOperation == nil {
		return false
	}
	if lastOperation.Type != gardenv1beta1.ShootLastOperationTypeCreate {
		return true
	}
	if lastOperation.State == gardenv1beta1.ShootLastOperationStateError || lastOperation.State == gardenv1beta1.ShootLastOperationStateFailed {
		return false
	}
	return true
}
