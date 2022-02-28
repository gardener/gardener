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

package secretbinding

import (
	"context"
	"fmt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) secretBindingAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.secretBindingQueue.Add(key)
}

func (c *Controller) secretBindingUpdate(oldObj, newObj interface{}) {
	c.secretBindingAdd(newObj)
}

func (c *Controller) secretBindingDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.secretBindingQueue.Add(key)
}

// NewSecretBindingReconciler creates a new instance of a reconciler which reconciles SecretBindings.
func NewSecretBindingReconciler(gardenClient client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &secretBindingReconciler{
		gardenClient: gardenClient,
		recorder:     recorder,
	}
}

type secretBindingReconciler struct {
	gardenClient client.Client
	recorder     record.EventRecorder
}

func (r *secretBindingReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	secretBinding := &gardencorev1beta1.SecretBinding{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, secretBinding); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// The deletionTimestamp labels a SecretBinding as intended to get deleted. Before deletion,
	// it has to be ensured that no Shoots are depending on the SecretBinding anymore.
	// When this happens the controller will remove the finalizers from the SecretBinding so that it can be garbage collected.
	if secretBinding.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(secretBinding, gardencorev1beta1.GardenerName) {
			return reconcile.Result{}, nil
		}

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.gardenClient, secretBinding)
		if err != nil {
			return reconcile.Result{}, err
		}

		if len(associatedShoots) == 0 {
			log.Info("No Shoots are referencing the SecretBinding, deletion accepted")

			mayReleaseSecret, err := r.mayReleaseSecret(ctx, secretBinding.Namespace, secretBinding.Name, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name)
			if err != nil {
				return reconcile.Result{}, err
			}

			if mayReleaseSecret {
				// Remove finalizer from referenced secret
				secret := &corev1.Secret{}
				if err := r.gardenClient.Get(ctx, kutil.Key(secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name), secret); err == nil {
					if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient, secret.DeepCopy(), gardencorev1beta1.ExternalGardenerName); client.IgnoreNotFound(err) != nil {
						return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from Secret: %w", err)
					}
				} else if !apierrors.IsNotFound(err) {
					return reconcile.Result{}, err
				}
			}

			// Remove finalizer from SecretBinding
			if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient, secretBinding, gardencorev1beta1.GardenerName); client.IgnoreNotFound(err) != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from SecretBinding: %w", err)
			}

			return reconcile.Result{}, nil
		}

		message := fmt.Sprintf("Cannot delete SecretBinding, because the following Shoots are still referencing it: %+v", associatedShoots)
		r.recorder.Event(secretBinding, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)
		return reconcile.Result{}, fmt.Errorf(message)
	}

	if !controllerutil.ContainsFinalizer(secretBinding, gardencorev1beta1.GardenerName) {
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, r.gardenClient, secretBinding, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to SecretBinding: %w", err)
		}
	}

	// Add the Gardener finalizer to the referenced SecretBinding secret to protect it from deletion as long as
	// the SecretBinding resource does exist.
	secret := &corev1.Secret{}
	if err := r.gardenClient.Get(ctx, kutil.Key(secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name), secret); err != nil {
		return reconcile.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, r.gardenClient, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to Secret referenced in SecretBinding: %w", err)
		}
	}

	if secretBinding.Provider != nil {
		types := gardencorev1beta1helper.GetSecretBindingTypes(secretBinding)
		for _, t := range types {
			labelKey := v1beta1constants.LabelShootProviderPrefix + t

			if !metav1.HasLabel(secret.ObjectMeta, labelKey) {
				patch := client.MergeFrom(secret.DeepCopy())
				metav1.SetMetaDataLabel(&secret.ObjectMeta, labelKey, "true")
				if err := r.gardenClient.Patch(ctx, secret, patch); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to add provider type label to Secret referenced in SecretBinding: %w", err)
				}
			}
		}
	}

	return reconcile.Result{}, nil
}

// We may only release a secret if there is no other secretbinding that references it (maybe in a different namespace).
func (r *secretBindingReconciler) mayReleaseSecret(ctx context.Context, secretBindingNamespace, secretBindingName, secretNamespace, secretName string) (bool, error) {
	secretBindingList := &gardencorev1beta1.SecretBindingList{}
	if err := r.gardenClient.List(ctx, secretBindingList); err != nil {
		return false, err
	}

	for _, secretBinding := range secretBindingList.Items {
		if secretBinding.Namespace == secretBindingNamespace && secretBinding.Name == secretBindingName {
			continue
		}
		if secretBinding.SecretRef.Namespace == secretNamespace && secretBinding.SecretRef.Name == secretName {
			return false, nil
		}
	}

	return true, nil
}
