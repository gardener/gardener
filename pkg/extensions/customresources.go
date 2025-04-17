// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	unstructuredutils "github.com/gardener/gardener/pkg/utils/kubernetes/unstructured"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// WaitUntilExtensionObjectReady waits until the given extension object has become ready.
// Passed objects are expected to be filled with the latest state the controller/component
// applied/observed/retrieved, but at least namespace and name.
func WaitUntilExtensionObjectReady(
	ctx context.Context,
	c client.Client,
	log logr.Logger,
	obj extensionsv1alpha1.Object,
	kind string,
	interval time.Duration,
	severeThreshold time.Duration,
	timeout time.Duration,
	postReadyFunc func() error,
) error {
	return WaitUntilObjectReadyWithHealthFunction(ctx, c, log, health.CheckExtensionObject, obj, kind, interval, severeThreshold, timeout, postReadyFunc)
}

// WaitUntilObjectReadyWithHealthFunction waits until the given object has become ready. It takes the health check
// function that should be executed.
// Passed objects are expected to be filled with the latest state the controller/component
// observed/retrieved, but at least namespace and name.
func WaitUntilObjectReadyWithHealthFunction(
	ctx context.Context,
	c client.Client,
	log logr.Logger,
	healthFunc health.Func,
	obj client.Object,
	kind string,
	interval time.Duration,
	severeThreshold time.Duration,
	timeout time.Duration,
	postReadyFunc func() error,
) error {
	var (
		lastObservedError     error
		retryCountUntilSevere int

		name      = obj.GetName()
		namespace = obj.GetNamespace()
	)

	// If the object has been reconciled successfully before we triggered a new reconciliation and our cache
	// is not updated fast enough with our reconciliation trigger (i.e. adding the reconcile annotation), we might
	// falsy return early from waiting for the object to be ready (as the last state already was "ready").
	// Use the timestamp annotation on the object as an ensurance, that once we see it in our cache, we are observing
	// a version of the object that is fresh enough.
	if expectedTimestamp, ok := obj.GetAnnotations()[v1beta1constants.GardenerTimestamp]; ok {
		healthFunc = health.And(health.ObjectHasAnnotationWithValue(v1beta1constants.GardenerTimestamp, expectedTimestamp), healthFunc)
	}

	var objectKind string
	if err := retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (bool, error) {
		retryCountUntilSevere++

		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.MinorError(err)
			}
			return retry.SevereError(err)
		}

		if objectKind == "" {
			if objectKind = obj.GetObjectKind().GroupVersionKind().Kind; objectKind != "" {
				log = log.WithValues("kind", objectKind)
			}
		}

		if err := healthFunc(obj); err != nil {
			lastObservedError = err
			log.Info("Object did not get ready yet", "reason", err.Error())

			if retry.IsRetriable(err) {
				return retry.MinorOrSevereError(retryCountUntilSevere, int(severeThreshold.Nanoseconds()/interval.Nanoseconds()), err)
			}
			return retry.MinorError(err)
		}

		if postReadyFunc != nil {
			if err := postReadyFunc(); err != nil {
				lastObservedError = err
				return retry.SevereError(err)
			}
		}

		return retry.Ok()
	}); err != nil {
		message := fmt.Sprintf("Error while waiting for %s to become ready", extensionKey(kind, namespace, name))
		if lastObservedError != nil {
			return fmt.Errorf("%s: %w", message, lastObservedError)
		}
		return fmt.Errorf("%s: %w", message, err)
	}

	return nil
}

// DeleteExtensionObject deletes a given extension object.
// Passed objects are expected to be filled with the latest state the controller/component
// observed/retrieved, but at least namespace and name.
func DeleteExtensionObject(
	ctx context.Context,
	c client.Writer,
	obj extensionsv1alpha1.Object,
	deleteOpts ...client.DeleteOption,
) error {
	if err := gardenerutils.ConfirmDeletion(ctx, c, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return client.IgnoreNotFound(c.Delete(ctx, obj, deleteOpts...))
}

// DeleteExtensionObjects lists all extension objects and loops over them. It executes the given predicateFunc for
// each of them, and if it evaluates to true then the object will be deleted.
func DeleteExtensionObjects(
	ctx context.Context,
	c client.Client,
	listObj client.ObjectList,
	namespace string,
	predicateFunc func(obj extensionsv1alpha1.Object) bool,
	deleteOpts ...client.DeleteOption,
) error {
	fns, err := applyFuncToExtensionObjects(ctx, c, listObj, namespace, predicateFunc, func(ctx context.Context, obj extensionsv1alpha1.Object) error {
		return DeleteExtensionObject(ctx, c, obj, deleteOpts...)
	})
	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitUntilExtensionObjectsDeleted lists all extension objects and loops over them. It executes the given
// predicateFunc for each of them, and if it evaluates to true, then it waits for the object to be deleted.
// If the component needs to wait for a given subset of all extension objects to be deleted (e.g. after deleting
// unwanted objects), it should pass a predicateFunc that filters objects to wait for by name.
func WaitUntilExtensionObjectsDeleted(
	ctx context.Context,
	c client.Client,
	log logr.Logger,
	listObj client.ObjectList,
	kind string,
	namespace string,
	interval time.Duration,
	timeout time.Duration,
	predicateFunc func(obj extensionsv1alpha1.Object) bool,
) error {
	fns, err := applyFuncToExtensionObjects(ctx, c, listObj, namespace, predicateFunc, func(ctx context.Context, obj extensionsv1alpha1.Object) error {
		return WaitUntilExtensionObjectDeleted(ctx, c, log, obj, kind, interval, timeout)
	})
	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitUntilExtensionObjectDeleted waits until an extension object is deleted from the system.
// Passed objects are expected to be filled with the latest state the controller/component
// observed/retrieved, but at least namespace and name.
func WaitUntilExtensionObjectDeleted(
	ctx context.Context,
	c client.Client,
	log logr.Logger,
	obj extensionsv1alpha1.Object,
	kind string,
	interval time.Duration,
	timeout time.Duration,
) error {
	var (
		lastObservedError error

		name      = obj.GetName()
		namespace = obj.GetNamespace()
	)

	if err := retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (bool, error) {
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}

		if lastErr := obj.GetExtensionStatus().GetLastError(); lastErr != nil {
			log.Info("Object did not get deleted yet", "description", lastErr.Description)
			lastObservedError = v1beta1helper.NewErrorWithCodes(errors.New(lastErr.Description), lastErr.Codes...)
		}

		var message = extensionKey(kind, namespace, name) + " is still present"
		if lastObservedError != nil {
			message += fmt.Sprintf(", last observed error: %s", lastObservedError.Error())
		}
		return retry.MinorError(errors.New(message))
	}); err != nil {
		message := "Failed to delete " + extensionKey(kind, namespace, name)
		if lastObservedError != nil {
			return fmt.Errorf("%s: %w", message, lastObservedError)
		}
		return fmt.Errorf("%s: %w", message, err)
	}

	return nil
}

// RestoreExtensionWithDeployFunction deploys the extension object with the passed in deployFunc and sets its operation annotation to wait-for-state.
// It then restores the state of the extension object from the ShootState, creates any required state object and sets the operation annotation to restore.
func RestoreExtensionWithDeployFunction(
	ctx context.Context,
	c client.Client,
	shootState *gardencorev1beta1.ShootState,
	kind string,
	deployFunc func(ctx context.Context, operationAnnotation string) (extensionsv1alpha1.Object, error),
) error {
	extensionObj, err := deployFunc(ctx, v1beta1constants.GardenerOperationWaitForState)
	if err != nil {
		return err
	}

	if err := RestoreExtensionObjectState(ctx, c, shootState, extensionObj, kind); err != nil {
		return err
	}

	return AnnotateObjectWithOperation(ctx, c, extensionObj, v1beta1constants.GardenerOperationRestore)
}

// RestoreExtensionObjectState restores the status.state field of the extension objects and deploys any required objects from the provided shoot state
func RestoreExtensionObjectState(
	ctx context.Context,
	c client.Client,
	shootState *gardencorev1beta1.ShootState,
	extensionObj extensionsv1alpha1.Object,
	kind string,
) error {
	var resourceRefs []autoscalingv1.CrossVersionObjectReference
	if shootState.Spec.Extensions != nil {
		resourceName := extensionObj.GetName()
		purpose := extensionObj.GetExtensionSpec().GetExtensionPurpose()
		list := v1beta1helper.ExtensionResourceStateList(shootState.Spec.Extensions)
		if extensionResourceState := list.Get(kind, &resourceName, purpose); extensionResourceState != nil {
			patch := client.MergeFrom(extensionObj.DeepCopyObject().(client.Object))
			extensionStatus := extensionObj.GetExtensionStatus()
			extensionStatus.SetState(extensionResourceState.State)
			extensionStatus.SetResources(extensionResourceState.Resources)

			if err := c.Status().Patch(ctx, extensionObj, patch); err != nil {
				return err
			}

			for _, r := range extensionResourceState.Resources {
				resourceRefs = append(resourceRefs, r.ResourceRef)
			}
		}
	}
	if shootState.Spec.Resources != nil {
		list := v1beta1helper.ResourceDataList(shootState.Spec.Resources)
		for _, resourceRef := range resourceRefs {
			resourceData := list.Get(&resourceRef)
			if resourceData != nil {
				obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&resourceData.Data)
				if err != nil {
					return err
				}
				if err := unstructuredutils.CreateOrPatchObjectByRef(ctx, c, &resourceRef, extensionObj.GetNamespace(), obj); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// MigrateExtensionObject adds the migrate operation annotation to the extension object.
// Passed objects are expected to be filled with the latest state the controller/component
// observed/retrieved, but at least namespace and name.
func MigrateExtensionObject(
	ctx context.Context,
	c client.Writer,
	obj extensionsv1alpha1.Object,
) error {
	return client.IgnoreNotFound(AnnotateObjectWithOperation(ctx, c, obj, v1beta1constants.GardenerOperationMigrate))
}

// MigrateExtensionObjects lists all extension objects of a given kind and annotates them with the Migrate operation.
// It executes the given predicateFunc for each of them, and if it evaluates to true, then it migrates the extension object.
// If predicateFunc is nil then migrates all extension objects.
func MigrateExtensionObjects(
	ctx context.Context,
	c client.Client,
	listObj client.ObjectList,
	namespace string,
	predicateFunc func(obj extensionsv1alpha1.Object) bool,
) error {
	fns, err := applyFuncToExtensionObjects(ctx, c, listObj, namespace, predicateFunc, func(ctx context.Context, obj extensionsv1alpha1.Object) error {
		return MigrateExtensionObject(ctx, c, obj)
	})
	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitUntilExtensionObjectMigrated waits until the migrate operation for the extension object is successful.
// Passed objects are expected to be filled with the latest state the controller/component
// observed/retrieved, but at least namespace and name.
func WaitUntilExtensionObjectMigrated(
	ctx context.Context,
	c client.Client,
	obj extensionsv1alpha1.Object,
	kind string,
	interval time.Duration,
	timeout time.Duration,
) error {
	return retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			if client.IgnoreNotFound(err) == nil {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}

		if extensionObjStatus := obj.GetExtensionStatus(); extensionObjStatus != nil {
			if lastOperation := extensionObjStatus.GetLastOperation(); lastOperation != nil {
				if lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate && lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
					return retry.Ok()
				}
			}
		}

		return retry.MinorError(fmt.Errorf("error while waiting for %s to be successfully migrated", extensionKey(kind, obj.GetNamespace(), obj.GetName())))
	})
}

// WaitUntilExtensionObjectsMigrated lists all extension objects of a given kind and waits until they are migrated.
// It executes the given predicateFunc for each of them, and if it evaluates to true, then it waits for the extension object to be migrated.
// If predicateFunc is nil then waits for all extension objects to be migrated.
func WaitUntilExtensionObjectsMigrated(
	ctx context.Context,
	c client.Client,
	listObj client.ObjectList,
	kind string,
	namespace string,
	interval time.Duration,
	timeout time.Duration,
	predicateFunc func(obj extensionsv1alpha1.Object) bool,
) error {
	fns, err := applyFuncToExtensionObjects(ctx, c, listObj, namespace, predicateFunc, func(ctx context.Context, obj extensionsv1alpha1.Object) error {
		return WaitUntilExtensionObjectMigrated(ctx, c, obj, kind, interval, timeout)
	})
	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// AnnotateObjectWithOperation annotates the object with the provided operation annotation value.
func AnnotateObjectWithOperation(ctx context.Context, w client.Writer, obj client.Object, operation string) error {
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	kubernetesutils.SetMetaDataAnnotation(obj, v1beta1constants.GardenerOperation, operation)
	kubernetesutils.SetMetaDataAnnotation(obj, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))
	return w.Patch(ctx, obj, patch)
}

func applyFuncToExtensionObjects(
	ctx context.Context,
	c client.Reader,
	listObj client.ObjectList,
	namespace string,
	predicateFunc func(obj extensionsv1alpha1.Object) bool,
	applyFunc func(ctx context.Context, object extensionsv1alpha1.Object) error,
) ([]flow.TaskFn, error) {
	if err := c.List(ctx, listObj, client.InNamespace(namespace)); err != nil {
		return nil, err
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
			return applyFunc(ctx, o)
		})

		return nil
	}); err != nil {
		return nil, err
	}

	return fns, nil
}

func extensionKey(kind, namespace, name string) string {
	return fmt.Sprintf("%s %s/%s", kind, namespace, name)
}
