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
	"strconv"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewProjectStaleReconciler creates a new instance of a reconciler which reconciles stale Projects.
func NewProjectStaleReconciler(
	l logrus.FieldLogger,
	config *config.ProjectControllerConfiguration,
	gardenClient client.Client,
) reconcile.Reconciler {
	return &projectStaleReconciler{
		logger:       l,
		config:       config,
		gardenClient: gardenClient,
	}
}

type projectStaleReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
	config       *config.ProjectControllerConfiguration
}

func (r *projectStaleReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	project := &gardencorev1beta1.Project{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	if err := r.reconcile(ctx, project); err != nil {
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

func (r *projectStaleReconciler) reconcile(ctx context.Context, project *gardencorev1beta1.Project) error {
	if project.DeletionTimestamp != nil || project.Spec.Namespace == nil {
		return nil
	}

	projectLogger := newProjectLogger(project)
	projectLogger.Infof("[STALE PROJECT RECONCILE]")

	// Skip projects whose namespace is annotated with the skip-stale-check annotation.
	namespace := &corev1.Namespace{}
	if err := r.gardenClient.Get(ctx, kutil.Key(*project.Spec.Namespace), namespace); err != nil {
		return err
	}

	var skipStaleCheck bool
	if value, ok := namespace.Annotations[v1beta1constants.ProjectSkipStaleCheck]; ok {
		skipStaleCheck, _ = strconv.ParseBool(value)
	}

	if skipStaleCheck {
		projectLogger.Infof("[STALE PROJECT RECONCILE] Namespace %q is annotated with %s, skipping the check and considering the project as 'not stale'", *project.Spec.Namespace, v1beta1constants.ProjectSkipStaleCheck)
		return r.markProjectAsNotStale(ctx, r.gardenClient, project)
	}

	// Skip projects that are not older than the configured minimum lifetime in days. This allows having Projects for a
	// certain period of time until they are checked whether they got stale.
	if project.CreationTimestamp.UTC().Add(time.Hour * 24 * time.Duration(*r.config.MinimumLifetimeDays)).After(NowFunc().UTC()) {
		projectLogger.Infof("[STALE PROJECT RECONCILE] Project is not older than the configured minimum %d days lifetime (%v), considering it 'not stale'", *r.config.MinimumLifetimeDays, project.CreationTimestamp.UTC())
		return r.markProjectAsNotStale(ctx, r.gardenClient, project)
	}

	//Skip projects that have been used recently
	if project.Status.LastActivityTimestamp != nil && project.Status.LastActivityTimestamp.UTC().Add(time.Hour*24*time.Duration(*r.config.MinimumLifetimeDays)).After(NowFunc().UTC()) {
		projectLogger.Infof("[STALE PROJECT RECONCILE] Project was used recently (%v) and it is not exceeding the minimum configured lifetime %d days, considering it 'not stale'", project.Status.LastActivityTimestamp.UTC(), *r.config.MinimumLifetimeDays)
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
			projectLogger.Infof("[STALE PROJECT RECONCILE] Project is being marked as 'not stale' because it is used by %s", check.resource)
			return r.markProjectAsNotStale(ctx, r.gardenClient, project)
		}
	}

	projectLogger.Infof("[STALE PROJECT RECONCILE] Project is being marked as 'stale' because it is not being used by any resource")
	if err := r.markProjectAsStale(ctx, r.gardenClient, project, NowFunc); err != nil {
		return err
	}

	projectLogger.Infof("[STALE PROJECT RECONCILE] Project is stale since %s", *project.Status.StaleSinceTimestamp)
	if project.Status.StaleAutoDeleteTimestamp != nil {
		projectLogger.Infof("[STALE PROJECT RECONCILE] Project will be deleted at %s", *project.Status.StaleAutoDeleteTimestamp)
	}

	if project.Status.StaleAutoDeleteTimestamp == nil || NowFunc().UTC().Before(project.Status.StaleAutoDeleteTimestamp.UTC()) {
		return nil
	}

	projectLogger.Infof("[STALE PROJECT RECONCILE] Deleting Project now because it's auto-delete timestamp is expired")
	if err := gutil.ConfirmDeletion(ctx, r.gardenClient, project); err != nil {
		if apierrors.IsNotFound(err) {
			projectLogger.Info("Project already gone")
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
	if err := r.gardenClient.List(ctx, secretList, client.InNamespace(namespace), gutil.UncontrolledSecretSelector); err != nil {
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

		for _, ownerRef := range secret.OwnerReferences {
			if ownerRef.APIVersion == gardencorev1beta1.SchemeGroupVersion.String() && ownerRef.Kind == "Shoot" {
				continue
			}
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
