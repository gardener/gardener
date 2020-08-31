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
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SyncClusterResourceToSeed creates or updates the `extensions.gardener.cloud/v1alpha1.Cluster` resource in the seed
// cluster by adding the shoot, seed, and cloudprofile specification.
func SyncClusterResourceToSeed(ctx context.Context, client client.Client, clusterName string, shoot *gardencorev1beta1.Shoot, cloudProfile *gardencorev1beta1.CloudProfile, seed *gardencorev1beta1.Seed) error {
	if shoot.Spec.SeedName == nil {
		return nil
	}

	var (
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
		}

		cloudProfileObj *gardencorev1beta1.CloudProfile
		seedObj         *gardencorev1beta1.Seed
		shootObj        *gardencorev1beta1.Shoot
	)

	if cloudProfile != nil {
		cloudProfileObj = cloudProfile.DeepCopy()
		cloudProfileObj.TypeMeta = metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "CloudProfile",
		}
	}

	if seed != nil {
		seedObj = seed.DeepCopy()
		seedObj.TypeMeta = metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "Seed",
		}
	}

	if shoot != nil {
		shootObj = shoot.DeepCopy()
		shootObj.TypeMeta = metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "Shoot",
		}

		// TODO: Workaround for the issue that was fixed with https://github.com/gardener/gardener/pull/2265. It adds a
		//       fake "observed generation" and a fake "last operation" and in case it is not set yet. This prevents the
		//       ShootNotFailed predicate in the extensions library from reacting false negatively. This fake status is only
		//       internally and will not be reported in the Shoot object in the garden cluster.
		//       This code can be removed in a future version after giving extension controllers enough time to revendor
		//       Gardener's extensions library.
		shootObj.Status.ObservedGeneration = shootObj.Generation
		if shootObj.Status.LastOperation == nil {
			shootObj.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeCreate,
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}
		}
	}

	_, err := controllerutil.CreateOrUpdate(ctx, client, cluster, func() error {
		if cloudProfileObj != nil {
			cluster.Spec.CloudProfile = runtime.RawExtension{Object: cloudProfileObj}
		}
		if seedObj != nil {
			cluster.Spec.Seed = runtime.RawExtension{Object: seedObj}
		}
		if shootObj != nil {
			cluster.Spec.Shoot = runtime.RawExtension{Object: shootObj}
		}
		return nil
	})
	return err
}

// WaitUntilExtensionCRReady waits until the given extension resource has become ready.
func WaitUntilExtensionCRReady(
	ctx context.Context,
	c client.Client,
	logger logrus.FieldLogger,
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
	logger logrus.FieldLogger,
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
			if apierrors.IsNotFound(err) {
				return retry.MinorError(err)
			}
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
	fns, err := applyFuncToExtensionResources(ctx, c, listObj, namespace, predicateFunc, func(ctx context.Context, obj extensionsv1alpha1.Object) error {
		return DeleteExtensionCR(
			ctx,
			c,
			newObjFunc,
			obj.GetNamespace(),
			obj.GetName(),
			deleteOpts...,
		)
	})

	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitUntilExtensionCRsDeleted lists all extension resources and loops over them. It executes the given <predicateFunc>
// for each of them, and if it evaluates to true then it waits for the resource to be deleted.
func WaitUntilExtensionCRsDeleted(
	ctx context.Context,
	c client.Client,
	logger logrus.FieldLogger,
	listObj runtime.Object,
	newObjFunc func() extensionsv1alpha1.Object,
	kind string,
	namespace string,
	interval time.Duration,
	timeout time.Duration,
	predicateFunc func(obj extensionsv1alpha1.Object) bool,
) error {
	fns, err := applyFuncToExtensionResources(
		ctx,
		c,
		listObj,
		namespace,
		func(obj extensionsv1alpha1.Object) bool {
			if obj.GetDeletionTimestamp() == nil {
				return false
			}
			if predicateFunc != nil && !predicateFunc(obj) {
				return false
			}
			return true
		},
		func(ctx context.Context, obj extensionsv1alpha1.Object) error {
			return WaitUntilExtensionCRDeleted(
				ctx,
				c,
				logger,
				newObjFunc,
				kind,
				obj.GetNamespace(),
				obj.GetName(),
				interval,
				timeout,
			)
		},
	)

	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitUntilExtensionCRDeleted waits until an extension resource is deleted from the system.
func WaitUntilExtensionCRDeleted(
	ctx context.Context,
	c client.Client,
	logger logrus.FieldLogger,
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

// RestoreExtensionWithDeployFunction deploys the extension resource with the passed in deployFunc and sets its operation annotation to wait-for-state.
// It then restores the state of the extension resource from the ShootState, creates any required state resources and sets the operation annotation to restore.
func RestoreExtensionWithDeployFunction(
	ctx context.Context,
	shootState *gardencorev1alpha1.ShootState,
	c client.Client,
	resourceKind string,
	namespace string,
	deployFunc func(ctx context.Context, operationAnnotation string) (extensionsv1alpha1.Object, error),
) error {
	extensionObj, err := deployFunc(ctx, v1beta1constants.GardenerOperationWaitForState)
	if err != nil {
		return err
	}

	if err := RestoreExtensionObjectState(ctx, c, shootState, namespace, extensionObj, resourceKind); err != nil {
		return err
	}

	return AnnotateExtensionObjectWithOperation(ctx, c, extensionObj, v1beta1constants.GardenerOperationRestore)
}

//RestoreExtensionObjectState restores the status.state field of the extension resources and deploys any required resources from the provided shoot state
func RestoreExtensionObjectState(
	ctx context.Context,
	c client.Client,
	shootState *gardencorev1alpha1.ShootState,
	namespace string,
	extensionObj extensionsv1alpha1.Object,
	resourceKind string,
) error {
	var resourceRefs []autoscalingv1.CrossVersionObjectReference
	if shootState.Spec.Extensions != nil {
		resourceName := extensionObj.GetName()
		purpose := extensionObj.GetExtensionSpec().GetExtensionPurpose()
		list := gardencorev1alpha1helper.ExtensionResourceStateList(shootState.Spec.Extensions)
		if extensionResourceState := list.Get(resourceKind, &resourceName, purpose); extensionResourceState != nil {
			extensionStatus := extensionObj.GetExtensionStatus()
			extensionStatus.SetState(extensionResourceState.State)
			extensionStatus.SetResources(extensionResourceState.Resources)

			if err := c.Status().Update(ctx, extensionObj); err != nil {
				return err
			}

			for _, r := range extensionResourceState.Resources {
				resourceRefs = append(resourceRefs, r.ResourceRef)
			}
		}
	}
	if shootState.Spec.Resources != nil {
		list := gardencorev1alpha1helper.ResourceDataList(shootState.Spec.Resources)
		for _, resourceRef := range resourceRefs {
			resourceData := list.Get(&resourceRef)
			if resourceData != nil {
				obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&resourceData.Data)
				if err != nil {
					return err
				}
				if err := utils.CreateOrUpdateObjectByRef(ctx, c, &resourceRef, namespace, obj); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// MigrateExtensionCR adds the migrate operation annotation to the extension CR.
func MigrateExtensionCR(
	ctx context.Context,
	c client.Client,
	newObjFunc func() extensionsv1alpha1.Object,
	namespace string,
	name string,
) error {
	obj := newObjFunc()
	obj.SetNamespace(namespace)
	obj.SetName(name)

	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return err
	}

	return AnnotateExtensionObjectWithOperation(ctx, c, obj, v1beta1constants.GardenerOperationMigrate)
}

// MigrateExtensionCRs lists all extension resources of a given kind and annotates them with the Migrate operation.
func MigrateExtensionCRs(
	ctx context.Context,
	c client.Client,
	listObj runtime.Object,
	newObjFunc func() extensionsv1alpha1.Object,
	namespace string,
) error {
	fns, err := applyFuncToExtensionResources(ctx, c, listObj, namespace, nil, func(ctx context.Context, o extensionsv1alpha1.Object) error {
		return MigrateExtensionCR(ctx, c, newObjFunc, o.GetNamespace(), o.GetName())
	})

	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// WaitUntilExtensionCRMigrated waits until the migrate operation for the extension resource is successful.
func WaitUntilExtensionCRMigrated(
	ctx context.Context,
	c client.Client,
	newObjFunc func() extensionsv1alpha1.Object,
	namespace string,
	name string,
	interval time.Duration,
	timeout time.Duration,
) error {
	obj := newObjFunc()
	obj.SetNamespace(namespace)
	obj.SetName(name)

	return retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
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

		var extensionType string
		if extensionSpec := obj.GetExtensionSpec(); extensionSpec != nil {
			extensionType = extensionSpec.GetExtensionType()
		}
		return retry.MinorError(fmt.Errorf("lastOperation for extension CR %s with name %s and type %s is not Migrate=Succeeded", obj.GetObjectKind().GroupVersionKind().Kind, name, extensionType))
	})
}

// WaitUntilExtensionCRsMigrated lists all extension resources of a given kind and waits until they are migrated
func WaitUntilExtensionCRsMigrated(
	ctx context.Context,
	c client.Client,
	listObj runtime.Object,
	newObjFunc func() extensionsv1alpha1.Object,
	namespace string,
	interval time.Duration,
	timeout time.Duration,
) error {
	fns, err := applyFuncToExtensionResources(ctx, c, listObj, namespace, nil, func(ctx context.Context, object extensionsv1alpha1.Object) error {
		return WaitUntilExtensionCRMigrated(
			ctx,
			c,
			newObjFunc,
			object.GetNamespace(),
			object.GetName(),
			interval,
			timeout,
		)
	})

	if err != nil {
		return err
	}

	return flow.Parallel(fns...)(ctx)
}

// AnnotateExtensionObjectWithOperation annotates the extension resource with the provided operation annotation value.
func AnnotateExtensionObjectWithOperation(ctx context.Context, c client.Client, extensionObj extensionsv1alpha1.Object, operation string) error {
	extensionObjCopy := extensionObj.DeepCopyObject()
	kutil.SetMetaDataAnnotation(extensionObj, v1beta1constants.GardenerOperation, operation)
	kutil.SetMetaDataAnnotation(extensionObj, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())
	return c.Patch(ctx, extensionObj, client.MergeFrom(extensionObjCopy))
}

func applyFuncToExtensionResources(
	ctx context.Context,
	c client.Client,
	listObj runtime.Object,
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

func formatErrorMessage(message, description string) string {
	return fmt.Sprintf("%s: %s", message, description)
}
