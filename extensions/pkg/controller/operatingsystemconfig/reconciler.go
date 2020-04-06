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

package operatingsystemconfig

import (
	"context"
	"fmt"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

// reconciler reconciles OperatingSystemConfig resources of Gardener's `extensions.gardener.cloud`
// API group.
type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	ctx    context.Context
	client client.Client
	scheme *runtime.Scheme
}

var _ reconcile.Reconciler = &reconciler{}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// OperatingSystemConfig resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator) reconcile.Reconciler {
	logger := log.Log.WithName(name)
	return extensionscontroller.OperationAnnotationWrapper(
		&extensionsv1alpha1.OperatingSystemConfig{},
		&reconciler{logger: logger, actuator: actuator},
	)
}

// InjectFunc enables dependency injection into the actuator.
func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

// InjectClient injects the controller runtime client into the reconciler.
func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) InjectScheme(scheme *runtime.Scheme) error {
	r.scheme = scheme
	return nil
}

// InjectStopChannel is an implementation for getting the respective stop channel managed by the controller-runtime.
func (r *reconciler) InjectStopChannel(stopCh <-chan struct{}) error {
	r.ctx = util.ContextFromStopChannel(stopCh)
	return nil
}

// Reconcile is the reconciler function that gets executed in case there are new events for the `OperatingSystemConfig`
// resources.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	osc := &extensionsv1alpha1.OperatingSystemConfig{}
	if err := r.client.Get(r.ctx, request.NamespacedName, osc); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		r.logger.Error(err, "Could not fetch OperatingSystemConfig")
		return reconcile.Result{}, err
	}

	if osc.DeletionTimestamp != nil {
		return r.delete(r.ctx, osc)
	}
	return r.reconcile(r.ctx, osc)
}

func (r *reconciler) reconcile(ctx context.Context, osc *extensionsv1alpha1.OperatingSystemConfig) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, osc); err != nil {
		return reconcile.Result{}, err
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(osc.ObjectMeta, osc.Status.LastOperation)
	if err := r.updateStatusProcessing(ctx, osc, operationType, "Reconciling the operating system config"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the reconciliation of operating system config", "osc", osc.Name)
	userData, command, units, err := r.actuator.Reconcile(ctx, osc)
	if err != nil {
		msg := "Error reconciling operating system config"
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), osc, operationType, msg))
		r.logger.Error(err, msg, "osc", osc.Name)
		return extensionscontroller.ReconcileErr(err)
	}

	secret := &corev1.Secret{ObjectMeta: SecretObjectMetaForConfig(osc)}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.client, secret, func() error {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		secret.Data[extensionsv1alpha1.OperatingSystemConfigSecretDataKey] = userData

		return controllerutil.SetControllerReference(osc, secret, r.scheme)
	}); err != nil {
		msg := "Could not apply secret for generated cloud config"
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), osc, operationType, msg))
		r.logger.Error(err, msg, "osc", osc.Name)
		return extensionscontroller.ReconcileErr(err)
	}

	osc.Status.CloudConfig = &extensionsv1alpha1.CloudConfig{
		SecretRef: corev1.SecretReference{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
	}
	osc.Status.Units = units
	if command != nil {
		osc.Status.Command = command
	}

	msg := "Successfully reconciled operating system config"
	r.logger.Info(msg, "osc", osc.Name)
	if err := r.updateStatusSuccess(ctx, osc, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, osc *extensionsv1alpha1.OperatingSystemConfig) (reconcile.Result, error) {
	hasFinalizer, err := extensionscontroller.HasFinalizer(osc, FinalizerName)
	if err != nil {
		r.logger.Error(err, "Could not instantiate finalizer deletion")
		return reconcile.Result{}, err
	}
	if !hasFinalizer {
		r.logger.Info("Deleting operating system config causes a no-op as there is no finalizer.", "osc", osc.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(osc.ObjectMeta, osc.Status.LastOperation)
	if err := r.updateStatusProcessing(ctx, osc, operationType, "Deleting the operating system config"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of operating system config", "osc", osc.Name)
	if err := r.actuator.Delete(ctx, osc); err != nil {
		msg := "Error deleting operating system config"
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), osc, operationType, msg))
		r.logger.Error(err, msg, "osc", osc.Name)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully deleted operating system config"
	r.logger.Info(msg, "osc", osc.Name)
	if err := r.updateStatusSuccess(ctx, osc, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer.", "osc", osc.Name)
	if err := extensionscontroller.DeleteFinalizer(ctx, r.client, FinalizerName, osc); err != nil {
		r.logger.Error(err, "Error removing finalizer from operating system config", "osc", osc.Name)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, osc *extensionsv1alpha1.OperatingSystemConfig, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	osc.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
	return r.client.Status().Update(ctx, osc)
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, osc *extensionsv1alpha1.OperatingSystemConfig, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	osc.Status.ObservedGeneration = osc.Generation
	osc.Status.LastOperation, osc.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(err)...)
	return r.client.Status().Update(ctx, osc)
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, osc *extensionsv1alpha1.OperatingSystemConfig, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	osc.Status.ObservedGeneration = osc.Generation
	osc.Status.LastOperation, osc.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
	return r.client.Status().Update(ctx, osc)
}
