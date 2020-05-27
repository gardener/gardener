// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitUntilExtensionCRReady waits until the given extension resource has become ready.
func WaitUntilExtensionCRReady(
	ctx context.Context,
	c client.Client,
	logger *logrus.Entry,
	newObjFunc func() runtime.Object,
	kind string,
	namespace string,
	name string,
	interval time.Duration,
	severeThreshold time.Duration,
	timeout time.Duration,
	postReadyFunc func(runtime.Object) error,
) error {
	return WaitUntilObjectReadyWithHealthFunction(
		ctx,
		c,
		logger,
		health.CheckExtensionObject,
		newObjFunc,
		kind,
		namespace,
		name,
		interval,
		severeThreshold,
		timeout,
		postReadyFunc,
	)
}

// WaitUntilObjectReadyWithHealthFunction waits until the given resource has become ready. It takes the health check
// function that should be executed.
func WaitUntilObjectReadyWithHealthFunction(
	ctx context.Context,
	c client.Client,
	logger *logrus.Entry,
	healthFunc func(obj runtime.Object) error,
	newObjFunc func() runtime.Object,
	kind string,
	namespace string,
	name string,
	interval time.Duration,
	severeThreshold time.Duration,
	timeout time.Duration,
	postReadyFunc func(runtime.Object) error,
) error {
	var (
		errorWithCode         *gardencorev1beta1helper.ErrorWithCodes
		lastObservedError     error
		retryCountUntilSevere int
	)

	if err := retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (bool, error) {
		retryCountUntilSevere++

		obj := newObjFunc()
		if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj); err != nil {
			return retry.SevereError(err)
		}

		if err := healthFunc(obj); err != nil {
			lastObservedError = err
			logger.WithError(err).Errorf("%s did not get ready yet", extensionKey(kind, namespace, name))
			if errors.As(err, &errorWithCode) {
				return retry.MinorOrSevereError(retryCountUntilSevere, int(severeThreshold.Nanoseconds()/interval.Nanoseconds()), err)
			}
			return retry.MinorError(err)
		}

		if postReadyFunc != nil {
			if err := postReadyFunc(obj); err != nil {
				return retry.SevereError(err)
			}
		}

		return retry.Ok()
	}); err != nil {
		message := fmt.Sprintf("Error while waiting for %s to become ready", extensionKey(kind, namespace, name))
		if lastObservedError != nil {
			return gardencorev1beta1helper.NewErrorWithCodes(formatErrorMessage(message, lastObservedError.Error()), gardencorev1beta1helper.ExtractErrorCodes(lastObservedError)...)
		}
		return errors.New(formatErrorMessage(message, err.Error()))
	}

	return nil
}

// DeleteExtensionCR deletes an extension resource.
func DeleteExtensionCR(
	ctx context.Context,
	c client.Client,
	newObjFunc func() extensionsv1alpha1.Object,
	namespace string,
	name string,
	deleteOpts ...client.DeleteOption,
) error {
	obj := newObjFunc()
	obj.SetNamespace(namespace)
	obj.SetName(name)

	if err := ConfirmDeletion(ctx, c, obj); err != nil {
		return err
	}

	if err := client.IgnoreNotFound(c.Delete(ctx, obj, deleteOpts...)); err != nil {
		return err
	}

	return nil
}

// DeleteExtensionCRs lists all extension resources and loops over them. It executes the given <predicateFunc> for each
// of them, and if it evaluates to true then the resource will be deleted.
func DeleteExtensionCRs(
	ctx context.Context,
	c client.Client,
	listObj runtime.Object,
	newObjFunc func() extensionsv1alpha1.Object,
	namespace string,
	predicateFunc func(obj extensionsv1alpha1.Object) bool,
	deleteOpts ...client.DeleteOption,
) error {
	if err := c.List(ctx, listObj, client.InNamespace(namespace)); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, meta.LenList(listObj))

	if err := meta.EachListItem(listObj, func(obj runtime.Object) error {
		o, ok := obj.(extensionsv1alpha1.Object)
		if !ok {
			return fmt.Errorf("expected extensionsv1alpha1.Object but got %T", obj)
		}

		if predicateFunc != nil && !predicateFunc(o) {
			return nil
		}

		fns = append(fns, func(ctx context.Context) error {
			return DeleteExtensionCR(ctx, c, newObjFunc, o.GetNamespace(), o.GetName(), deleteOpts...)
		})

		return nil
	}); err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitUntilExtensionCRsDeleted lists all extension resources and loops over them. It executes the given <predicateFunc>
// for each of them, and if it evaluates to true then it waits for the resource to be deleted.
func WaitUntilExtensionCRsDeleted(
	ctx context.Context,
	c client.Client,
	logger *logrus.Entry,
	listObj runtime.Object,
	newObjFunc func() extensionsv1alpha1.Object,
	kind string,
	namespace string,
	interval time.Duration,
	timeout time.Duration,
	predicateFunc func(obj extensionsv1alpha1.Object) bool,
) error {
	if err := c.List(ctx, listObj, client.InNamespace(namespace)); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, meta.LenList(listObj))

	if err := meta.EachListItem(listObj, func(obj runtime.Object) error {
		o, ok := obj.(extensionsv1alpha1.Object)
		if !ok {
			return fmt.Errorf("expected extensionsv1alpha1.Object but got %T", obj)
		}

		if o.GetDeletionTimestamp() == nil {
			return nil
		}

		if predicateFunc != nil && !predicateFunc(o) {
			return nil
		}

		fns = append(fns, func(ctx context.Context) error {
			return WaitUntilExtensionCRDeleted(
				ctx,
				c,
				logger,
				newObjFunc,
				kind,
				o.GetNamespace(),
				o.GetName(),
				interval,
				timeout,
			)
		})

		return nil
	}); err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitUntilExtensionCRDeleted waits until an extension resource is deleted from the system.
func WaitUntilExtensionCRDeleted(
	ctx context.Context,
	c client.Client,
	logger *logrus.Entry,
	newObjFunc func() extensionsv1alpha1.Object,
	kind string,
	namespace string,
	name string,
	interval time.Duration,
	timeout time.Duration,
) error {
	var lastObservedError error

	if err := retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (bool, error) {
		obj := newObjFunc()
		if err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}

		acc, err := extensions.Accessor(obj)
		if err != nil {
			return retry.SevereError(err)
		}

		if lastErr := acc.GetExtensionStatus().GetLastError(); lastErr != nil {
			logger.Errorf("%s did not get deleted yet, lastError is: %s", extensionKey(kind, namespace, name), lastErr.Description)
			lastObservedError = gardencorev1beta1helper.NewErrorWithCodes(lastErr.Description, lastErr.Codes...)
		}

		var message = fmt.Sprintf("%s is still present", extensionKey(kind, namespace, name))
		if lastObservedError != nil {
			message += fmt.Sprintf(", last observed error: %s", lastObservedError.Error())
		}
		return retry.MinorError(fmt.Errorf(message))
	}); err != nil {
		message := fmt.Sprintf("Failed to delete %s", extensionKey(kind, namespace, name))
		if lastObservedError != nil {
			return gardencorev1beta1helper.NewErrorWithCodes(formatErrorMessage(message, lastObservedError.Error()), gardencorev1beta1helper.ExtractErrorCodes(lastObservedError)...)
		}
		return errors.New(formatErrorMessage(message, err.Error()))
	}

	return nil
}

func extensionKey(kind, namespace, name string) string {
	return fmt.Sprintf("%s %s/%s", kind, namespace, name)
}

func formatErrorMessage(message, description string) string {
	return fmt.Sprintf("%s: %s", message, description)
}
