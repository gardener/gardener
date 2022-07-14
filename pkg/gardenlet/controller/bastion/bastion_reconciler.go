// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package bastion

import (
	"context"
	"errors"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// finalizerName is the Kubernetes finalizerName that is used to control the cleanup of
	// Bastion resources in the seed cluster.
	finalizerName = gardencorev1alpha1.GardenerName

	defaultTimeout         = 30 * time.Second
	defaultSevereThreshold = 15 * time.Second
	defaultInterval        = 5 * time.Second
)

// reconciler implements the reconcile.Reconcile interface for bastion reconciliation.
type reconciler struct {
	clientMap clientmap.ClientMap
	config    *config.GardenletConfiguration
}

// newReconciler returns the new bastion reconciler.
func newReconciler(clientMap clientmap.ClientMap, config *config.GardenletConfiguration) reconcile.Reconciler {
	return &reconciler{
		clientMap: clientMap,
		config:    config,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	bastion := &operationsv1alpha1.Bastion{}
	if err := gardenClient.Client().Get(ctx, request.NamespacedName, bastion); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if !IsBastionManagedByThisGardenlet(bastion, r.config) {
		log.V(1).Info("Skipping because Bastion is not managed by this gardenlet", "seedName", *bastion.Spec.SeedName)
		return reconcile.Result{}, nil
	}

	// retrieve Kubernetes client for Seed cluster
	seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(*bastion.Spec.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get seed client: %w", err)
	}

	// get Shoot for the bastion
	shoot := gardencorev1beta1.Shoot{}
	shootKey := kutil.Key(bastion.Namespace, bastion.Spec.ShootRef.Name)
	if err := gardenClient.Client().Get(ctx, shootKey, &shoot); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not get shoot %v: %w", shootKey, err)
	}

	if bastion.DeletionTimestamp != nil {
		err = r.cleanupBastion(ctx, log, gardenClient.Client(), seedClient.Client(), bastion, &shoot)
	} else {
		err = r.reconcileBastion(ctx, log, gardenClient.Client(), seedClient.Client(), bastion, &shoot)
	}

	if cause := reconcilerutils.ReconcileErrCause(err); cause != nil {
		log.Error(cause, "Reconciling failed")
	}

	return reconcilerutils.ReconcileErr(err)
}

func (r *reconciler) reconcileBastion(
	ctx context.Context,
	log logr.Logger,
	gardenClient client.Client,
	seedClient client.Client,
	bastion *operationsv1alpha1.Bastion,
	shoot *gardencorev1beta1.Shoot,
) error {
	// ensure finalizer is set
	if !controllerutil.ContainsFinalizer(bastion, finalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, gardenClient, bastion, finalizerName); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}
		return nil
	}

	// prepare extension resource
	extBastion := newBastionExtension(bastion, shoot)
	extensionsIngress := make([]extensionsv1alpha1.BastionIngressPolicy, len(bastion.Spec.Ingress))
	for i, ingress := range bastion.Spec.Ingress {
		extensionsIngress[i] = extensionsv1alpha1.BastionIngressPolicy{
			IPBlock: ingress.IPBlock,
		}
	}

	// create or patch the bastion in the seed cluster
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, seedClient, extBastion, func() error {
		metav1.SetMetaDataAnnotation(&extBastion.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&extBastion.ObjectMeta, v1beta1constants.GardenerTimestamp, time.Now().UTC().String())
		extBastion.Spec.UserData = createUserData(bastion)
		extBastion.Spec.Ingress = extensionsIngress
		extBastion.Spec.Type = *bastion.Spec.ProviderType
		return nil
	}); err != nil {
		if patchErr := patchReadyCondition(ctx, gardenClient, bastion, gardencorev1alpha1.ConditionFalse, "FailedReconciling", err.Error()); patchErr != nil {
			log.Error(patchErr, "Failed patching ready condition")
		}

		return fmt.Errorf("failed to ensure bastion extension resource: %w", err)
	}

	// wait for the extension controller to reconcile possible changes
	if err := extensions.WaitUntilExtensionObjectReady(
		ctx,
		seedClient,
		log,
		extBastion,
		extensionsv1alpha1.BastionResource,
		defaultInterval,
		defaultSevereThreshold,
		defaultTimeout,
		nil,
	); err != nil {
		if patchErr := patchReadyCondition(ctx, gardenClient, bastion, gardencorev1alpha1.ConditionFalse, "FailedReconciling", err.Error()); patchErr != nil {
			log.Error(patchErr, "Failed patching ready condition")
		}

		return fmt.Errorf("failed wait for bastion extension resource to be reconciled: %w", err)
	}

	// copy over the extension's status to the garden and set the condition
	patch := client.MergeFrom(bastion.DeepCopy())
	setReadyCondition(bastion, gardencorev1alpha1.ConditionTrue, "SuccessfullyReconciled", "The bastion has been reconciled successfully.")
	bastion.Status.Ingress = extBastion.Status.Ingress.DeepCopy()
	bastion.Status.ObservedGeneration = &bastion.Generation
	if err := gardenClient.Status().Patch(ctx, bastion, patch); err != nil {
		return fmt.Errorf("failed patching ready condition of Bastion: %w", err)
	}

	return nil
}

func (r *reconciler) cleanupBastion(
	ctx context.Context,
	log logr.Logger,
	gardenClient client.Client,
	seedClient client.Client,
	bastion *operationsv1alpha1.Bastion,
	shoot *gardencorev1beta1.Shoot,
) error {
	if !sets.NewString(bastion.Finalizers...).Has(finalizerName) {
		return nil
	}

	if err := patchReadyCondition(ctx, gardenClient, bastion, gardencorev1alpha1.ConditionFalse, "DeletionInProgress", "The bastion is being deleted."); err != nil {
		return fmt.Errorf("failed patching ready condition of Bastion: %w", err)
	}

	// delete bastion extension resource in seed cluster
	extBastion := newBastionExtension(bastion, shoot)

	if err := seedClient.Delete(ctx, extBastion); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Successfully deleted")

			if controllerutil.ContainsFinalizer(bastion, finalizerName) {
				log.Info("Removing finalizer")
				if err := controllerutils.RemoveFinalizers(ctx, gardenClient, bastion, finalizerName); err != nil {
					return fmt.Errorf("failed to remove finalizer: %w", err)
				}
			}

			return nil
		}

		return fmt.Errorf("failed to delete bastion extension resource: %w", err)
	}

	// cleanup is now triggered on the seed, requeue to wait for it to happen
	return &reconcilerutils.RequeueAfterError{
		RequeueAfter: 5 * time.Second,
		Cause:        errors.New("bastion extension cleanup has not completed yet"),
	}
}

func newBastionExtension(bastion *operationsv1alpha1.Bastion, shoot *gardencorev1beta1.Shoot) *extensionsv1alpha1.Bastion {
	return &extensionsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bastion.Name,
			Namespace: shoot.Status.TechnicalID,
		},
	}
}

func setReadyCondition(bastion *operationsv1alpha1.Bastion, status gardencorev1alpha1.ConditionStatus, reason string, message string) {
	condition := gardencorev1alpha1helper.GetOrInitCondition(bastion.Status.Conditions, operationsv1alpha1.BastionReady)
	condition = gardencorev1alpha1helper.UpdatedCondition(condition, status, reason, message)

	bastion.Status.Conditions = gardencorev1alpha1helper.MergeConditions(bastion.Status.Conditions, condition)
}

func patchReadyCondition(ctx context.Context, c client.StatusClient, bastion *operationsv1alpha1.Bastion, status gardencorev1alpha1.ConditionStatus, reason string, message string) error {
	patch := client.MergeFrom(bastion.DeepCopy())
	setReadyCondition(bastion, status, reason, message)
	return c.Status().Patch(ctx, bastion, patch)
}

func createUserData(bastion *operationsv1alpha1.Bastion) []byte {
	userData := fmt.Sprintf(`#!/bin/bash -eu

id gardener || useradd gardener -mU
mkdir -p /home/gardener/.ssh
echo "%s" > /home/gardener/.ssh/authorized_keys
chown gardener:gardener /home/gardener/.ssh/authorized_keys
echo "gardener ALL=(ALL) NOPASSWD:ALL" >/etc/sudoers.d/99-gardener-user
`, bastion.Spec.SSHPublicKey)

	return []byte(userData)
}
