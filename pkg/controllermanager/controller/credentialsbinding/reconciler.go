// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package credentialsbinding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
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
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
)

// Reconciler reconciles CredentialsBinding.
type Reconciler struct {
	Client   client.Client
	Config   controllermanagerconfigv1alpha1.CredentialsBindingControllerConfiguration
	Recorder record.EventRecorder
}

// Reconcile reconciles CredentialsBinding.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	credentialsBinding := &securityv1alpha1.CredentialsBinding{}
	if err := r.Client.Get(ctx, request.NamespacedName, credentialsBinding); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// The deletionTimestamp labels a CredentialsBinding as intended to get deleted. Before deletion,
	// it has to be ensured that no Shoots are depending on the CredentialsBinding anymore.
	// When this happens the controller will remove the finalizers from the CredentialsBinding so that it can be garbage collected.
	if credentialsBinding.DeletionTimestamp != nil {
		return reconcile.Result{}, r.delete(ctx, credentialsBinding, log)
	}

	return reconcile.Result{}, r.reconcile(ctx, credentialsBinding, log)
}

func (r *Reconciler) getCredentialsFromRef(ctx context.Context, ref corev1.ObjectReference) (client.Object, error) {
	var obj client.Object
	switch ref.GroupVersionKind() {
	case corev1.SchemeGroupVersion.WithKind("Secret"):
		obj = &corev1.Secret{}
	case securityv1alpha1.SchemeGroupVersion.WithKind("WorkloadIdentity"):
		obj = &securityv1alpha1.WorkloadIdentity{}
	default:
		return nil, fmt.Errorf("unsupported credentials reference: %s, %s", ref.Namespace+"/"+ref.Name, ref.GroupVersionKind().String())
	}

	return obj, r.Client.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, obj)
}

func (r *Reconciler) reconcile(ctx context.Context, credentialsBinding *securityv1alpha1.CredentialsBinding, log logr.Logger) error {
	if !controllerutil.ContainsFinalizer(credentialsBinding, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, credentialsBinding, gardencorev1beta1.GardenerName); err != nil {
			return fmt.Errorf("could not add finalizer: %w", err)
		}
	}

	// Add the Gardener finalizer to the referenced Secret/WorkloadIdentity
	// to protect it from deletion as long as the CredentialsBinding resource exists.
	credential, err := r.getCredentialsFromRef(ctx, credentialsBinding.CredentialsRef)
	if err != nil {
		return err
	}
	kind := credential.GetObjectKind().GroupVersionKind().Kind

	if !controllerutil.ContainsFinalizer(credential, gardencorev1beta1.ExternalGardenerName) {
		log.Info("Adding finalizer", kind, client.ObjectKeyFromObject(credential)) //nolint:logcheck
		if err := controllerutils.AddFinalizers(ctx, r.Client, credential, gardencorev1beta1.ExternalGardenerName); err != nil {
			return fmt.Errorf("could not add finalizer to %s: %w", kind, err)
		}
	}

	providerTypeLabelKey := v1beta1constants.LabelShootProviderPrefix + credentialsBinding.Provider.Type
	labels := credential.GetLabels()

	_, hasProviderKeyLabel := labels[providerTypeLabelKey]
	_, hasCredentialsBindingRefLabel := labels[v1beta1constants.LabelCredentialsBindingReference]
	if !hasProviderKeyLabel || !hasCredentialsBindingRefLabel {
		patch := client.MergeFrom(credential.DeepCopyObject().(client.Object))
		if !hasProviderKeyLabel {
			credential.SetLabels(utils.MergeStringMaps(credential.GetLabels(), map[string]string{
				providerTypeLabelKey: "true",
			}))
		}
		if !hasCredentialsBindingRefLabel {
			credential.SetLabels(utils.MergeStringMaps(credential.GetLabels(), map[string]string{
				v1beta1constants.LabelCredentialsBindingReference: "true",
			}))
		}
		if err := r.Client.Patch(ctx, credential, patch); err != nil {
			return fmt.Errorf("failed to add provider type or/and referred labels to %s referenced in CredentialsBinding: %w", kind, err)
		}
	}

	for _, objRef := range credentialsBinding.Quotas {
		quota := &gardencorev1beta1.Quota{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: objRef.Namespace, Name: objRef.Name}, quota); err != nil {
			return err
		}

		// Add 'referred by a credentials binding' label
		if !metav1.HasLabel(quota.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference) {
			patch := client.MergeFrom(quota.DeepCopy())
			metav1.SetMetaDataLabel(&quota.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference, "true")
			if err := r.Client.Patch(ctx, quota, patch); err != nil {
				return fmt.Errorf("failed to add referred label to the quota referenced in CredentialsBinding, quota: %s , namespace: %s : %w", quota.Name, quota.Namespace, err)
			}
		}
	}

	return nil
}

func (r *Reconciler) delete(ctx context.Context, credentialsBinding *securityv1alpha1.CredentialsBinding, log logr.Logger) error {
	if !controllerutil.ContainsFinalizer(credentialsBinding, gardencorev1beta1.GardenerName) {
		return nil
	}

	associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.Client, credentialsBinding)
	if err != nil {
		return err
	}

	if len(associatedShoots) != 0 {
		message := fmt.Sprintf("Cannot delete CredentialsBinding, because the following Shoots are still referencing it: %+v", associatedShoots)
		r.Recorder.Event(credentialsBinding, corev1.EventTypeWarning, v1beta1constants.EventResourceReferenced, message)
		return errors.New(message)
	}

	log.Info("No Shoots are referencing the CredentialsBinding, deletion accepted")
	mayReleaseCredentials, err := r.mayReleaseCredentials(ctx, credentialsBinding)
	if err != nil {
		return err
	}

	if mayReleaseCredentials {
		credential, err := r.getCredentialsFromRef(ctx, credentialsBinding.CredentialsRef)
		kind := credential.GetObjectKind().GroupVersionKind().Kind
		if err == nil {
			// Remove shoot provider label and 'referred by a credentials binding' label
			hasProviderLabel, providerLabel := getProviderLabel(credential.GetLabels())
			_, hasCredentialsBindingRefLabel := credential.GetLabels()[v1beta1constants.LabelCredentialsBindingReference]
			_, hasSecretBindingRefLabel := credential.GetLabels()[v1beta1constants.LabelSecretBindingReference]
			if hasProviderLabel || hasCredentialsBindingRefLabel {
				patch := client.MergeFrom(credential.DeepCopyObject().(client.Object))

				labels := credential.GetLabels()
				delete(labels, v1beta1constants.LabelCredentialsBindingReference)

				// The secret can be still referenced by a secretbinding so
				// only remove the provider label if there is no secretbinding reference label
				if !hasSecretBindingRefLabel {
					delete(labels, providerLabel)
				}

				credential.SetLabels(labels)
				if err := r.Client.Patch(ctx, credential, patch); err != nil {
					return fmt.Errorf("failed to remove referred label from %s: %w", kind, err)
				}
			}

			// Remove finalizer from referenced secret
			if controllerutil.ContainsFinalizer(credential, gardencorev1beta1.ExternalGardenerName) && !hasSecretBindingRefLabel {
				log.Info("Removing finalizer", kind, client.ObjectKeyFromObject(credential)) //nolint:logcheck
				if err := controllerutils.RemoveFinalizers(ctx, r.Client, credential, gardencorev1beta1.ExternalGardenerName); err != nil {
					return fmt.Errorf("failed to remove finalizer from %s: %w", kind, err)
				}
			}
		} else if !apierrors.IsNotFound(err) {
			return err
		}
	}

	if err := r.removeLabelFromQuotas(ctx, credentialsBinding); err != nil {
		return err
	}

	// Remove finalizer from CredentialsBinding
	log.Info("Removing finalizer")
	if err := controllerutils.RemoveFinalizers(ctx, r.Client, credentialsBinding, gardencorev1beta1.GardenerName); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return nil
}

// We may only release a credential if there is no other credentials binding that references it (maybe in a different namespace).
func (r *Reconciler) mayReleaseCredentials(ctx context.Context, binding *securityv1alpha1.CredentialsBinding) (bool, error) {
	credentialsBindingList := &securityv1alpha1.CredentialsBindingList{}
	if err := r.Client.List(ctx, credentialsBindingList); err != nil {
		return false, err
	}

	for _, cb := range credentialsBindingList.Items {
		// skip if it is one and the same credentials binding
		if cb.Namespace == binding.Namespace && cb.Name == binding.Name {
			continue
		}
		if cb.CredentialsRef.APIVersion == binding.CredentialsRef.APIVersion &&
			cb.CredentialsRef.Kind == binding.CredentialsRef.Kind &&
			cb.CredentialsRef.Namespace == binding.CredentialsRef.Namespace &&
			cb.CredentialsRef.Name == binding.CredentialsRef.Name {
			return false, nil
		}
	}

	return true, nil
}

// Remove the label from the quota only if there is no other credentialsbindings that reference it (maybe in a different namespace).
func (r *Reconciler) removeLabelFromQuotas(ctx context.Context, binding *securityv1alpha1.CredentialsBinding) error {
	credentialsBindingList := &securityv1alpha1.CredentialsBindingList{}
	if err := r.Client.List(ctx, credentialsBindingList); err != nil {
		return err
	}

	for _, q := range binding.Quotas {
		if quotaHasOtherRef(q, credentialsBindingList, binding.Namespace, binding.Name) {
			continue
		}

		quota := &gardencorev1beta1.Quota{}
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: q.Namespace, Name: q.Name}, quota); err != nil {
			return err
		}

		// Remove 'referred by a credentials binding' label
		if metav1.HasLabel(quota.ObjectMeta, v1beta1constants.LabelCredentialsBindingReference) {
			patch := client.MergeFromWithOptions(quota.DeepCopy(), client.MergeFromWithOptimisticLock{})
			delete(quota.Labels, v1beta1constants.LabelCredentialsBindingReference)
			if err := r.Client.Patch(ctx, quota, patch); err != nil {
				return fmt.Errorf("failed to remove referred label from Quota: %w", err)
			}
		}
	}

	return nil
}

func quotaHasOtherRef(
	quota corev1.ObjectReference,
	credentialsBindingList *securityv1alpha1.CredentialsBindingList,
	credentialsBindingNamespace,
	credentialsBindingName string,
) bool {
	for _, cb := range credentialsBindingList.Items {
		if cb.Namespace == credentialsBindingNamespace && cb.Name == credentialsBindingName {
			continue
		}
		for _, q := range cb.Quotas {
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
