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

package project

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const staleReconcilerName = "stale"

// NewProjectStaleReconciler creates a new instance of a reconciler which reconciles stale Projects.
func NewProjectStaleReconciler(config *config.ProjectControllerConfiguration, gardenClient client.Client) reconcile.Reconciler {
	return &projectStaleReconciler{
		config:       config,
		gardenClient: gardenClient,
	}
}

type projectStaleReconciler struct {
	gardenClient client.Client
	config       *config.ProjectControllerConfiguration
}

func (r *projectStaleReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	project := &gardencorev1beta1.Project{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if err := r.reconcile(ctx, log, project); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.config.StaleSyncPeriod.Duration}, nil
}

type projectInUseChecker struct {
	resource  string
	checkFunc func(context.Context, string) (bool, error)
}

// NowFunc is the same like metav1.Now.
// Exposed for testing.
var NowFunc = metav1.Now

func (r *projectStaleReconciler) reconcile(ctx context.Context, log logr.Logger, project *gardencorev1beta1.Project) error {
	if project.DeletionTimestamp != nil || project.Spec.Namespace == nil {
		return nil
	}

	// Skip projects whose namespace is annotated with the skip-stale-check annotation.
	namespace := &corev1.Namespace{}
	if err := r.gardenClient.Get(ctx, kutil.Key(*project.Spec.Namespace), namespace); err != nil {
		return err
	}

	log = log.WithValues("namespaceName", namespace.Name)

	var skipStaleCheck bool
	if value, ok := namespace.Annotations[v1beta1constants.ProjectSkipStaleCheck]; ok {
		skipStaleCheck, _ = strconv.ParseBool(value)
	}

	if skipStaleCheck {
		log.Info("Namespace is marked to skip the stale check, marking Project as not stale")
		return r.markProjectAsNotStale(ctx, r.gardenClient, project)
	}

	// Skip projects that are not older than the configured minimum lifetime in days. This allows having Projects for a
	// certain period of time until they are checked whether they got stale.
	if project.CreationTimestamp.UTC().Add(time.Hour * 24 * time.Duration(*r.config.MinimumLifetimeDays)).After(NowFunc().UTC()) {
		log.Info("Project is not older than the configured minimum lifetime, marking Project as not stale", "minimumLifetimeDays", *r.config.MinimumLifetimeDays, "creationTimestamp", project.CreationTimestamp.UTC())
		return r.markProjectAsNotStale(ctx, r.gardenClient, project)
	}

	// Skip projects that have been used recently
	if project.Status.LastActivityTimestamp != nil && project.Status.LastActivityTimestamp.UTC().Add(time.Hour*24*time.Duration(*r.config.MinimumLifetimeDays)).After(NowFunc().UTC()) {
		log.Info("Project was used recently and it is not exceeding the configured minimum lifetime, marking Project as not stale", "minimumLifetimeDays", *r.config.MinimumLifetimeDays, "lastActivityTimestamp", project.Status.LastActivityTimestamp.UTC())
		return r.markProjectAsNotStale(ctx, r.gardenClient, project)
	}

	for _, check := range []projectInUseChecker{
		{"Shoots", r.projectInUseDueToShoots},
		{"Plants", r.projectInUseDueToPlants},
		{"BackupEntries", r.projectInUseDueToBackupEntries},
		{"Secrets", r.projectInUseDueToSecrets},
		{"Quotas", r.projectInUseDueToQuotas},
	} {
		projectInUse, err := check.checkFunc(ctx, *project.Spec.Namespace)
		if err != nil {
			return err
		}
		if projectInUse {
			log.Info("Project is in use by resource, marking Project as not stale", "resource", check.resource)
			return r.markProjectAsNotStale(ctx, r.gardenClient, project)
		}
	}

	log.Info("Project is not in use by any resource, marking Project as stale")
	if err := r.markProjectAsStale(ctx, r.gardenClient, project, NowFunc); err != nil {
		return err
	}

	log = log.WithValues("staleSinceTimestamp", (*project.Status.StaleSinceTimestamp).Time)
	if project.Status.StaleAutoDeleteTimestamp != nil {
		log = log.WithValues("staleAutoDeleteTimestamp", (*project.Status.StaleAutoDeleteTimestamp).Time)
	}

	if project.Status.StaleAutoDeleteTimestamp == nil || NowFunc().UTC().Before(project.Status.StaleAutoDeleteTimestamp.UTC()) {
		log.Info("Project is stale, but will not be deleted now")
		return nil
	}

	log.Info("Deleting Project now because its auto-delete timestamp is exceeded")
	if err := gutil.ConfirmDeletion(ctx, r.gardenClient, project); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Project already gone")
			return nil
		}
		return err
	}
	return client.IgnoreNotFound(r.gardenClient.Delete(ctx, project))
}

func (r *projectStaleReconciler) projectInUseDueToShoots(ctx context.Context, namespace string) (bool, error) {
	return kutil.IsNamespaceInUse(ctx, r.gardenClient, namespace, gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
}

func (r *projectStaleReconciler) projectInUseDueToPlants(ctx context.Context, namespace string) (bool, error) {
	return kutil.IsNamespaceInUse(ctx, r.gardenClient, namespace, gardencorev1beta1.SchemeGroupVersion.WithKind("PlantList"))
}

func (r *projectStaleReconciler) projectInUseDueToBackupEntries(ctx context.Context, namespace string) (bool, error) {
	return kutil.IsNamespaceInUse(ctx, r.gardenClient, namespace, gardencorev1beta1.SchemeGroupVersion.WithKind("BackupEntryList"))
}

func (r *projectStaleReconciler) projectInUseDueToSecrets(ctx context.Context, namespace string) (bool, error) {
	secretList := &corev1.SecretList{}
	if err := r.gardenClient.List(
		ctx,
		secretList,
		client.InNamespace(namespace),
		gutil.UncontrolledSecretSelector,
		client.MatchingLabels{v1beta1constants.LabelSecretBindingReference: "true"},
	); err != nil {
		return false, err
	}

	secretNames := computeSecretNames(secretList.Items)
	if secretNames.Len() == 0 {
		return false, nil
	}

	return r.relevantSecretBindingsInUse(ctx, func(secretBinding gardencorev1beta1.SecretBinding) bool {
		return secretBinding.SecretRef.Namespace == namespace && secretNames.Has(secretBinding.SecretRef.Name)
	})
}

func (r *projectStaleReconciler) projectInUseDueToQuotas(ctx context.Context, namespace string) (bool, error) {
	quotaList := &metav1.PartialObjectMetadataList{}
	quotaList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("QuotaList"))

	if err := r.gardenClient.List(ctx, quotaList, client.InNamespace(namespace)); err != nil {
		return false, err
	}

	quotaNames := computeQuotaNames(quotaList.Items)
	if quotaNames.Len() == 0 {
		return false, nil
	}

	return r.relevantSecretBindingsInUse(ctx, func(secretBinding gardencorev1beta1.SecretBinding) bool {
		for _, quota := range secretBinding.Quotas {
			return quota.Namespace == namespace && quotaNames.Has(quota.Name)
		}
		return false
	})
}

func (r *projectStaleReconciler) relevantSecretBindingsInUse(ctx context.Context, isSecretBindingRelevantFunc func(secretBinding gardencorev1beta1.SecretBinding) bool) (bool, error) {
	secretBindingList := &gardencorev1beta1.SecretBindingList{}
	if err := r.gardenClient.List(ctx, secretBindingList); err != nil {
		return false, err
	}

	namespaceToSecretBindingNames := make(map[string]sets.String)
	for _, secretBinding := range secretBindingList.Items {
		if !isSecretBindingRelevantFunc(secretBinding) {
			continue
		}

		if _, ok := namespaceToSecretBindingNames[secretBinding.Namespace]; !ok {
			namespaceToSecretBindingNames[secretBinding.Namespace] = sets.NewString(secretBinding.Name)
		} else {
			namespaceToSecretBindingNames[secretBinding.Namespace].Insert(secretBinding.Name)
		}
	}

	return r.secretBindingInUse(ctx, namespaceToSecretBindingNames)
}

func (r *projectStaleReconciler) markProjectAsNotStale(ctx context.Context, client client.Client, project *gardencorev1beta1.Project) error {
	return updateStatus(ctx, client, project, func() {
		project.Status.StaleSinceTimestamp = nil
		project.Status.StaleAutoDeleteTimestamp = nil
	})
}

func (r *projectStaleReconciler) markProjectAsStale(ctx context.Context, client client.Client, project *gardencorev1beta1.Project, nowFunc func() metav1.Time) error {
	return updateStatus(ctx, client, project, func() {
		if project.Status.StaleSinceTimestamp == nil {
			now := nowFunc()
			project.Status.StaleSinceTimestamp = &now
		}

		if project.Status.StaleSinceTimestamp.UTC().Add(time.Hour * 24 * time.Duration(*r.config.StaleGracePeriodDays)).After(nowFunc().UTC()) {
			// We reset the potentially set auto-delete timestamp here to allow changing the StaleExpirationTimeDays
			// configuration value and correctly applying the changes to all Projects that had already been assigned
			// such a timestamp.
			project.Status.StaleAutoDeleteTimestamp = nil
			return
		}

		// If the project got stale we compute an auto delete timestamp only if the configured stale grace period is
		// exceeded. Note that this might update the potentially already set auto-delete timestamp in case the
		// StaleExpirationTimeDays configuration value was changed.
		autoDeleteTimestamp := metav1.Time{Time: project.Status.StaleSinceTimestamp.Add(time.Hour * 24 * time.Duration(*r.config.StaleExpirationTimeDays))}

		// Don't allow to shorten the auto-delete timestamp as end-users might depend on the configured time. It may
		// only be extended.
		if project.Status.StaleAutoDeleteTimestamp == nil || autoDeleteTimestamp.After(project.Status.StaleAutoDeleteTimestamp.Time) {
			project.Status.StaleAutoDeleteTimestamp = &autoDeleteTimestamp
		}
	})
}

func (r *projectStaleReconciler) secretBindingInUse(ctx context.Context, namespaceToSecretBindingNames map[string]sets.String) (bool, error) {
	if len(namespaceToSecretBindingNames) == 0 {
		return false, nil
	}

	for namespace, secretBindingNames := range namespaceToSecretBindingNames {
		shootList := &gardencorev1beta1.ShootList{}
		if err := r.gardenClient.List(ctx, shootList, client.InNamespace(namespace)); err != nil {
			return false, err
		}

		for _, shoot := range shootList.Items {
			if secretBindingNames.Has(shoot.Spec.SecretBindingName) {
				return true, nil
			}
		}
	}

	return false, nil
}

// computeSecretNames determines the names of Secrets that are of type Opaque and don't have owner references to a
// Shoot.
func computeSecretNames(secretList []corev1.Secret) sets.String {
	names := sets.NewString()

	for _, secret := range secretList {
		if secret.Type != corev1.SecretTypeOpaque {
			continue
		}

		hasOwnerRef := false
		for _, ownerRef := range secret.OwnerReferences {
			if ownerRef.APIVersion == gardencorev1beta1.SchemeGroupVersion.String() && ownerRef.Kind == "Shoot" {
				hasOwnerRef = true
				break
			}
		}
		if hasOwnerRef {
			continue
		}

		names.Insert(secret.Name)
	}

	return names
}

// computeQuotaNames determines the names of Quotas from the given slice.
func computeQuotaNames(quotaList []metav1.PartialObjectMetadata) sets.String {
	names := sets.NewString()

	for _, quota := range quotaList {
		names.Insert(quota.Name)
	}

	return names
}
