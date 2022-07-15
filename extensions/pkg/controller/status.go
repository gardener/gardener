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
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// StatusUpdater contains functions for updating statuses of extension resources after a controller operation.
type StatusUpdater interface {
	// InjectClient injects the client into the status updater.
	InjectClient(client.Client)
	// Processing updates the last operation of an extension resource when an operation is started.
	Processing(context.Context, logr.Logger, extensionsv1alpha1.Object, gardencorev1beta1.LastOperationType, string) error
	// Error updates the last operation of an extension resource when an operation was erroneous.
	Error(context.Context, logr.Logger, extensionsv1alpha1.Object, error, gardencorev1beta1.LastOperationType, string) error
	// Success updates the last operation of an extension resource when an operation was successful.
	Success(context.Context, logr.Logger, extensionsv1alpha1.Object, gardencorev1beta1.LastOperationType, string) error
}

// UpdaterFunc is a function to perform additional updates of the status.
type UpdaterFunc func(extensionsv1alpha1.Status) error

// StatusUpdaterCustom contains functions for customized updating statuses of extension resources after a controller operation.
type StatusUpdaterCustom interface {
	// InjectClient injects the client into the status updater.
	InjectClient(client.Client)
	// ProcessingCustom updates the last operation of an extension resource when an operation is started.
	ProcessingCustom(context.Context, logr.Logger, extensionsv1alpha1.Object, gardencorev1beta1.LastOperationType, string, UpdaterFunc) error
	// ErrorCustom updates the last operation of an extension resource when an operation was erroneous.
	ErrorCustom(context.Context, logr.Logger, extensionsv1alpha1.Object, error, gardencorev1beta1.LastOperationType, string, UpdaterFunc) error
	// SuccessCustom updates the last operation of an extension resource when an operation was successful.
	SuccessCustom(context.Context, logr.Logger, extensionsv1alpha1.Object, gardencorev1beta1.LastOperationType, string, UpdaterFunc) error
}

// NewStatusUpdater returns a new status updater.
func NewStatusUpdater() *statusUpdater {
	return &statusUpdater{}
}

type statusUpdater struct {
	client client.Client
}

var _ = StatusUpdater(&statusUpdater{})
var _ = StatusUpdaterCustom(&statusUpdater{})

func (s *statusUpdater) InjectClient(c client.Client) {
	s.client = c
}

func (s *statusUpdater) Processing(
	ctx context.Context,
	log logr.Logger,
	obj extensionsv1alpha1.Object,
	lastOperationType gardencorev1beta1.LastOperationType,
	description string,
) error {
	return s.ProcessingCustom(ctx, log, obj, lastOperationType, description, nil)
}

func (s *statusUpdater) ProcessingCustom(
	ctx context.Context,
	log logr.Logger,
	obj extensionsv1alpha1.Object,
	lastOperationType gardencorev1beta1.LastOperationType,
	description string,
	updater UpdaterFunc,
) error {
	if s.client == nil {
		return fmt.Errorf("client is not set. Call InjectClient() first")
	}

	log.Info(description) //nolint:logcheck

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	lastOp := LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
	obj.GetExtensionStatus().SetLastOperation(lastOp)
	if updater != nil {
		err := updater(obj.GetExtensionStatus())
		if err != nil {
			return err
		}
	}
	return s.client.Status().Patch(ctx, obj, patch)
}

func (s *statusUpdater) Error(
	ctx context.Context,
	log logr.Logger,
	obj extensionsv1alpha1.Object,
	err error,
	lastOperationType gardencorev1beta1.LastOperationType,
	description string,
) error {
	return s.ErrorCustom(ctx, log, obj, err, lastOperationType, description, nil)
}

func (s *statusUpdater) ErrorCustom(
	ctx context.Context,
	log logr.Logger,
	obj extensionsv1alpha1.Object,
	err error,
	lastOperationType gardencorev1beta1.LastOperationType,
	description string,
	updater UpdaterFunc,
) error {
	if s.client == nil {
		return fmt.Errorf("client is not set. Call InjectClient() first")
	}

	var (
		errDescription  = gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err))
		lastOp, lastErr = ReconcileError(lastOperationType, errDescription, 50, gardencorev1beta1helper.ExtractErrorCodes(err)...)
	)

	log.Error(fmt.Errorf(errDescription), "Error") //nolint:logcheck

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	obj.GetExtensionStatus().SetObservedGeneration(obj.GetGeneration())
	obj.GetExtensionStatus().SetLastOperation(lastOp)
	obj.GetExtensionStatus().SetLastError(lastErr)
	if updater != nil {
		err := updater(obj.GetExtensionStatus())
		if err != nil {
			return err
		}
	}
	return s.client.Status().Patch(ctx, obj, patch)
}

func (s *statusUpdater) Success(
	ctx context.Context,
	log logr.Logger,
	obj extensionsv1alpha1.Object,
	lastOperationType gardencorev1beta1.LastOperationType,
	description string,
) error {
	return s.SuccessCustom(ctx, log, obj, lastOperationType, description, nil)
}

func (s *statusUpdater) SuccessCustom(
	ctx context.Context,
	log logr.Logger,
	obj extensionsv1alpha1.Object,
	lastOperationType gardencorev1beta1.LastOperationType,
	description string,
	updater UpdaterFunc,
) error {
	if s.client == nil {
		return fmt.Errorf("client is not set. Call InjectClient() first")
	}

	log.Info(description) //nolint:logcheck

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	lastOp, lastErr := ReconcileSucceeded(lastOperationType, description)
	obj.GetExtensionStatus().SetObservedGeneration(obj.GetGeneration())
	obj.GetExtensionStatus().SetLastOperation(lastOp)
	obj.GetExtensionStatus().SetLastError(lastErr)
	if updater != nil {
		err := updater(obj.GetExtensionStatus())
		if err != nil {
			return err
		}
	}
	return s.client.Status().Patch(ctx, obj, patch)
}
