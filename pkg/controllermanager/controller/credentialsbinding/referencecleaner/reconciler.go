// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package referencecleaner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Reconciler triggers regularly and cleans not needed labels and finalizers from
// Secrets and WorkloadIdentities that are no longer referenced by CredentialBinding resource.
type Reconciler struct {
	Reader client.Reader
	Writer client.Writer
	Config config.CredentialsBindingReferenceCleanerControllerConfiguration
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(reconcileCtx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(reconcileCtx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(reconcileCtx, r.Config.SyncPeriod.Duration)
	defer cancel()

	log.Info("Starting credentials binding reference cleanup")
	defer log.Info("Credentials binding reference cleanup finished")

	var (
		labels         = client.MatchingLabels{v1beta1constants.LabelCredentialsBindingReference: "true"}
		objectsToClean = map[objectId]client.Object{}
	)

	for _, resource := range []struct {
		schemaGroupVersion schema.GroupVersion
		kind               string
		listKind           string
	}{
		{corev1.SchemeGroupVersion, "Secret", "SecretList"},
		{securityv1alpha1.SchemeGroupVersion, "WorkloadIdentity", "WorkloadIdentityList"},
	} {
		objList := &metav1.PartialObjectMetadataList{}
		objList.SetGroupVersionKind(resource.schemaGroupVersion.WithKind(resource.listKind))
		if err := r.Reader.List(ctx, objList, labels); err != nil {
			return reconcile.Result{}, err
		}

		for _, obj := range objList.Items {
			obj.SetGroupVersionKind(resource.schemaGroupVersion.WithKind(resource.kind))
			objectsToClean[objectId{resource.kind, obj.Namespace, obj.Name}] = &obj
		}
	}

	credentialsBindingList := &securityv1alpha1.CredentialsBindingList{}
	if err := r.Reader.List(ctx, credentialsBindingList); err != nil {
		return reconcile.Result{}, err
	}

	for _, cb := range credentialsBindingList.Items {
		delete(objectsToClean, objectId{cb.CredentialsRef.Kind, cb.CredentialsRef.Namespace, cb.CredentialsRef.Name})
	}

	var (
		results = make(chan error, 1)
		wg      wait.Group
		errList error
	)

	for id, obj := range objectsToClean {
		wg.StartWithContext(ctx, func(ctx context.Context) {
			// Remove shoot provider label and 'referred by a credentials binding' label
			hasProviderLabel, providerLabel := getProviderLabel(obj.GetLabels())
			_, hasCredentialsBindingRefLabel := obj.GetLabels()[v1beta1constants.LabelCredentialsBindingReference]
			_, hasSecretBindingRefLabel := obj.GetLabels()[v1beta1constants.LabelSecretBindingReference]
			gvk := obj.GetObjectKind().GroupVersionKind()
			if hasProviderLabel || hasCredentialsBindingRefLabel {
				patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))

				labels := obj.GetLabels()
				delete(labels, v1beta1constants.LabelCredentialsBindingReference)

				// The credential can be still referenced by a secretbinding so
				// only remove the provider label if there is no secretbinding reference label
				if !hasSecretBindingRefLabel {
					delete(labels, providerLabel)
				}

				obj.SetLabels(labels)
				log.Info("Removing referred label", id.kind, client.ObjectKeyFromObject(obj)) //nolint:logcheck
				if err := r.Writer.Patch(ctx, obj, patch); err != nil {
					results <- fmt.Errorf("failed to remove referred label from %s: %w", id.kind, err)
				}
			}

			// Remove finalizer from referenced credential
			if controllerutil.ContainsFinalizer(obj, gardencorev1beta1.ExternalGardenerName) && !hasSecretBindingRefLabel {
				log.Info("Removing finalizer", id.kind, client.ObjectKeyFromObject(obj)) //nolint:logcheck
				obj.GetObjectKind().SetGroupVersionKind(gvk)
				if err := controllerutils.RemoveFinalizers(ctx, r.Writer, obj, gardencorev1beta1.ExternalGardenerName); err != nil {
					results <- fmt.Errorf("failed to remove finalizer from %s: %w", id.kind, err)
				}
			}
		})
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for err := range results {
		if err != nil {
			errList = errors.Join(errList, err)
		}
	}

	return reconcile.Result{Requeue: true, RequeueAfter: r.Config.SyncPeriod.Duration}, errList
}

type objectId struct {
	kind      string
	namespace string
	name      string
}

func getProviderLabel(labels map[string]string) (bool, string) {
	for label := range labels {
		if strings.HasPrefix(label, v1beta1constants.LabelShootProviderPrefix) {
			return true, label
		}
	}

	return false, ""
}
