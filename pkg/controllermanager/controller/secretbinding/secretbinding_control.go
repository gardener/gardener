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
				secret := &corev1.Secret{}
				if err := r.gardenClient.Get(ctx, kutil.Key(secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name), secret); err == nil {
					// Remove 'referred by a secret binding' label
					if metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
						patch := client.MergeFrom(secret.DeepCopy())
						delete(secret.ObjectMeta.Labels, v1beta1constants.LabelSecretBindingReference)
						if err := r.gardenClient.Patch(ctx, secret, patch); err != nil {
							return reconcile.Result{}, fmt.Errorf("failed to remove referred label from Secret: %w", err)
						}
					}
					// Remove finalizer from referenced secret
					if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient, secret.DeepCopy(), gardencorev1beta1.ExternalGardenerName); client.IgnoreNotFound(err) != nil {
						return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from Secret: %w", err)
					}
				} else if !apierrors.IsNotFound(err) {
					return reconcile.Result{}, err
				}
			}

			if err := r.removeLabelfromQuotas(ctx, secretBinding.Quotas, secretBinding.Namespace, secretBinding.Name); err != nil {
				return reconcile.Result{}, err
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
			return reconcile.Result{}, fmt.Errorf("could not add finalizer to SecretBinding: %w", err)
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
			return reconcile.Result{}, fmt.Errorf("could not add finalizer to Secret referenced in SecretBinding: %w", err)
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

	if len(secretBinding.Quotas) != 0 {
		for _, objRef := range secretBinding.Quotas {
			quota := &gardencorev1beta1.Quota{}
			if err := r.gardenClient.Get(ctx, kutil.Key(objRef.Namespace, objRef.Name), quota); err != nil {
				return reconcile.Result{}, err
			}

			// Add 'referred by a secret binding' label
			if !metav1.HasLabel(quota.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
				patch := client.MergeFrom(quota.DeepCopy())
				metav1.SetMetaDataLabel(&quota.ObjectMeta, v1beta1constants.LabelSecretBindingReference, "true")
				if err := r.gardenClient.Patch(ctx, quota, patch); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to add referred label to the quota referenced in SecretBinding, quota: %s , namespace: %s : %w", quota.Name, quota.Namespace, err)
				}
			}
		}
	}

	// Add 'referred by a secret binding' label
	if !metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
		patch := client.MergeFrom(secret.DeepCopy())
		metav1.SetMetaDataLabel(&secret.ObjectMeta, v1beta1constants.LabelSecretBindingReference, "true")
		if err := r.gardenClient.Patch(ctx, secret, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add referred label to Secret referenced in SecretBinding: %w", err)
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

// Remove the label from the quota only if there is no other secretbinding that references it (maybe in a different namespace).
func (r *secretBindingReconciler) removeLabelfromQuotas(ctx context.Context, quotas []corev1.ObjectReference, secretBindingNamespace, secretBindingName string) error {
	secretBindingList := &gardencorev1beta1.SecretBindingList{}
	if err := r.gardenClient.List(ctx, secretBindingList); err != nil {
		return err
	}

	for _, q := range quotas {
		if quotaHasOtherRef(q, secretBindingList, secretBindingNamespace, secretBindingName) {
			continue
		}

		quota := &gardencorev1beta1.Quota{}
		if err := r.gardenClient.Get(ctx, kutil.Key(q.Namespace, q.Name), quota); err != nil {
			return err
		}

		// Remove 'referred by a secret binding' label
		if metav1.HasLabel(quota.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
			patch := client.MergeFromWithOptions(quota.DeepCopy(), client.MergeFromWithOptimisticLock{})
			delete(quota.ObjectMeta.Labels, v1beta1constants.LabelSecretBindingReference)
			if err := r.gardenClient.Patch(ctx, quota, patch); err != nil {
				return fmt.Errorf("failed to remove referred label from Quota: %w", err)
			}
		}
	}

	return nil
}

func quotaHasOtherRef(quota corev1.ObjectReference, secretBindingList *gardencorev1beta1.SecretBindingList, secretBindingNamespace, secretBindingName string) bool {
	for _, secretBinding := range secretBindingList.Items {
		if secretBinding.Namespace == secretBindingNamespace && secretBinding.Name == secretBindingName {
			continue
		}
		for _, q := range secretBinding.Quotas {
			if q.Name == quota.Name && q.Namespace == quota.Namespace {
				return true
			}
		}
	}

	return false
}
