// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretbinding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Reconciler reconciles SecretBindings.
type Reconciler struct {
	Client   client.Client
	Config   controllermanagerconfigv1alpha1.SecretBindingControllerConfiguration
	Recorder record.EventRecorder
}

// Reconcile reconciles SecretBindings.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	secretBinding := &gardencorev1beta1.SecretBinding{}
	if err := r.Client.Get(ctx, request.NamespacedName, secretBinding); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
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

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.Client, secretBinding)
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
				if err := r.Client.Get(ctx, client.ObjectKey{Namespace: secretBinding.SecretRef.Namespace, Name: secretBinding.SecretRef.Name}, secret); err == nil {
					// Remove shoot provider label and 'referred by a secret binding' label
					hasProviderLabel, providerLabel := getProviderLabel(secret.Labels)
					if hasProviderLabel || metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
						patch := client.MergeFrom(secret.DeepCopy())
						delete(secret.Labels, v1beta1constants.LabelSecretBindingReference)

						// The secret can be still referenced by a credentialsbinding so
						// only remove the provider label if there is no credentialsbinding reference label
						if !metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference) {
							delete(secret.Labels, providerLabel)
						}

						if err := r.Client.Patch(ctx, secret, patch); err != nil {
							return reconcile.Result{}, fmt.Errorf("failed to remove referred label from Secret: %w", err)
						}
					}
					// Remove finalizer from referenced secret
					// only if the secret does not have a credentialsbinding reference label
					if controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) && !metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference) {
						log.Info("Removing finalizer from secret", "secret", client.ObjectKeyFromObject(secret))
						if err := controllerutils.RemoveFinalizers(ctx, r.Client, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
							return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from secret: %w", err)
						}
					}
				} else if !apierrors.IsNotFound(err) {
					return reconcile.Result{}, err
				}
			}

			if err := r.removeLabelFromQuotas(ctx, secretBinding.Quotas, secretBinding.Namespace, secretBinding.Name); err != nil {
				return reconcile.Result{}, err
			}

			// Remove finalizer from SecretBinding
			if controllerutil.ContainsFinalizer(secretBinding, gardencorev1beta1.GardenerName) {
				log.Info("Removing finalizer")
				if err := controllerutils.RemoveFinalizers(ctx, r.Client, secretBinding, gardencorev1beta1.GardenerName); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
				}
			}

			return reconcile.Result{}, nil
		}

		message := fmt.Sprintf("Cannot delete SecretBinding, because the following Shoots are still referencing it: %+v", associatedShoots)
		r.Recorder.Event(secretBinding, corev1.EventTypeWarning, v1beta1constants.EventResourceReferenced, message)
		return reconcile.Result{}, errors.New(message)
	}

	if !controllerutil.ContainsFinalizer(secretBinding, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, secretBinding, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not add finalizer: %w", err)
		}
	}

	// Add the Gardener finalizer to the referenced Secret to protect it from deletion as long as the
	// SecretBinding resource exists.
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: secretBinding.SecretRef.Namespace, Name: secretBinding.SecretRef.Name}, secret); err != nil {
		return reconcile.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(secret, gardencorev1beta1.ExternalGardenerName) {
		log.Info("Adding finalizer to secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.AddFinalizers(ctx, r.Client, secret, gardencorev1beta1.ExternalGardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not add finalizer to secret: %w", err)
		}
	}

	if secretBinding.Provider != nil {
		types := v1beta1helper.GetSecretBindingTypes(secretBinding)
		for _, t := range types {
			labelKey := v1beta1constants.LabelShootProviderPrefix + t

			if !metav1.HasLabel(secret.ObjectMeta, labelKey) {
				patch := client.MergeFrom(secret.DeepCopy())
				metav1.SetMetaDataLabel(&secret.ObjectMeta, labelKey, "true")
				if err := r.Client.Patch(ctx, secret, patch); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to add provider type label to Secret referenced in SecretBinding: %w", err)
				}
			}
		}
	}

	if len(secretBinding.Quotas) != 0 {
		for _, objRef := range secretBinding.Quotas {
			quota := &gardencorev1beta1.Quota{}
			if err := r.Client.Get(ctx, client.ObjectKey{Namespace: objRef.Namespace, Name: objRef.Name}, quota); err != nil {
				return reconcile.Result{}, err
			}

			// Add 'referred by a secret binding' label
			if !metav1.HasLabel(quota.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
				patch := client.MergeFrom(quota.DeepCopy())
				metav1.SetMetaDataLabel(&quota.ObjectMeta, v1beta1constants.LabelSecretBindingReference, "true")
				if err := r.Client.Patch(ctx, quota, patch); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to add referred label to the quota referenced in SecretBinding, quota: %s , namespace: %s : %w", quota.Name, quota.Namespace, err)
				}
			}
		}
	}

	// Add 'referred by a secret binding' label
	if !metav1.HasLabel(secret.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
		patch := client.MergeFrom(secret.DeepCopy())
		metav1.SetMetaDataLabel(&secret.ObjectMeta, v1beta1constants.LabelSecretBindingReference, "true")
		if err := r.Client.Patch(ctx, secret, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add referred label to Secret referenced in SecretBinding: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

// We may only release a secret if there is no other secretbinding that references it (maybe in a different namespace).
func (r *Reconciler) mayReleaseSecret(ctx context.Context, secretBindingNamespace, secretBindingName, secretNamespace, secretName string) (bool, error) {
	secretBindingList := &gardencorev1beta1.SecretBindingList{}
	if err := r.Client.List(ctx, secretBindingList); err != nil {
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
func (r *Reconciler) removeLabelFromQuotas(ctx context.Context, quotas []corev1.ObjectReference, secretBindingNamespace, secretBindingName string) error {
	secretBindingList := &gardencorev1beta1.SecretBindingList{}
	if err := r.Client.List(ctx, secretBindingList); err != nil {
		return err
	}

	for _, q := range quotas {
		if quotaHasOtherRef(q, secretBindingList, secretBindingNamespace, secretBindingName) {
			continue
		}

		quota := &gardencorev1beta1.Quota{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: q.Namespace, Name: q.Name}, quota); err != nil {
			return err
		}

		// Remove 'referred by a secret binding' label
		if metav1.HasLabel(quota.ObjectMeta, v1beta1constants.LabelSecretBindingReference) {
			patch := client.MergeFromWithOptions(quota.DeepCopy(), client.MergeFromWithOptimisticLock{})
			delete(quota.Labels, v1beta1constants.LabelSecretBindingReference)
			if err := r.Client.Patch(ctx, quota, patch); err != nil {
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

func getProviderLabel(labels map[string]string) (bool, string) {
	for label := range labels {
		if strings.HasPrefix(label, v1beta1constants.LabelShootProviderPrefix) {
			return true, label
		}
	}

	return false, ""
}
