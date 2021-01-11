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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *Controller) reconcileStaleProjectKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	project, err := c.projectLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[STALE PROJECT RECONCILE] %s - skipping because Project has been deleted", key)
		c.projectStaleQueue.Done(key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[STALE PROJECT RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.staleControl.ReconcileStaleProject(project, metav1.Now); err != nil {
		return err
	}

	c.projectStaleQueue.AddAfter(key, c.config.Controllers.Project.StaleSyncPeriod.Duration)
	return nil
}

// StaleControlInterface implements the control logic for updating Projects. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type StaleControlInterface interface {
	// ReconcileProject implements the control logic for checking and potentially auto-deleting stale Projects.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	ReconcileStaleProject(project *gardencorev1beta1.Project, nowFunc func() metav1.Time) error
}

// NewDefaultStaleControl returns a new instance of the default implementation StaleControlInterface that
// implements the documented semantics for Projects. updater is the UpdaterInterface used
// to update the status of Projects. You should use an instance returned from NewDefaultStaleControl() for any
// scenario other than testing.
func NewDefaultStaleControl(
	clientMap clientmap.ClientMap,
	config *config.ControllerManagerConfiguration,
	shootLister gardencorelisters.ShootLister,
	plantLister gardencorelisters.PlantLister,
	backupEntryLister gardencorelisters.BackupEntryLister,
	secretBindingLister gardencorelisters.SecretBindingLister,
	quotaLister gardencorelisters.QuotaLister,
	namespaceLister kubecorev1listers.NamespaceLister,
	secretLister kubecorev1listers.SecretLister,
) StaleControlInterface {
	return &defaultStaleControl{
		clientMap,
		config,
		shootLister,
		plantLister,
		backupEntryLister,
		secretBindingLister,
		quotaLister,
		namespaceLister,
		secretLister,
	}
}

type defaultStaleControl struct {
	clientMap           clientmap.ClientMap
	config              *config.ControllerManagerConfiguration
	shootLister         gardencorelisters.ShootLister
	plantLister         gardencorelisters.PlantLister
	backupEntryLister   gardencorelisters.BackupEntryLister
	secretBindingLister gardencorelisters.SecretBindingLister
	quotaLister         gardencorelisters.QuotaLister
	namespaceLister     kubecorev1listers.NamespaceLister
	secretLister        kubecorev1listers.SecretLister
}

type projectInUseChecker struct {
	resource  string
	checkFunc func(string) (bool, error)
}

func (c *defaultStaleControl) ReconcileStaleProject(obj *gardencorev1beta1.Project, nowFunc func() metav1.Time) error {
	if obj.DeletionTimestamp != nil || obj.Spec.Namespace == nil {
		return nil
	}

	var (
		ctx           = context.TODO()
		project       = obj.DeepCopy()
		projectLogger = newProjectLogger(project)
	)

	projectLogger.Infof("[STALE PROJECT RECONCILE]")

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	// Skip projects whose namespace is annotated with the skip-stale-check annotation.
	namespace, err := c.namespaceLister.Get(*project.Spec.Namespace)
	if err != nil {
		return err
	}

	var skipStaleCheck bool
	if value, ok := namespace.Annotations[common.ProjectSkipStaleCheck]; ok {
		skipStaleCheck, _ = strconv.ParseBool(value)
	}

	if skipStaleCheck {
		projectLogger.Infof("[STALE PROJECT RECONCILE] Namespace %q is annotated with %s, skipping the check and considering the project as 'not stale'", *project.Spec.Namespace, common.ProjectSkipStaleCheck)
		return c.markProjectAsNotStale(ctx, gardenClient.Client(), project)
	}

	// Skip projects that are not older than the configured minimum lifetime in days. This allows having Projects for a
	// certain period of time until they are checked whether they got stale.
	if project.CreationTimestamp.UTC().Add(time.Hour * 24 * time.Duration(*c.config.Controllers.Project.MinimumLifetimeDays)).After(nowFunc().UTC()) {
		projectLogger.Infof("[STALE PROJECT RECONCILE] Project is not older than the configured minimum %d days lifetime (%v), considering it 'not stale'", *c.config.Controllers.Project.MinimumLifetimeDays, project.CreationTimestamp.UTC())
		return c.markProjectAsNotStale(ctx, gardenClient.Client(), project)
	}

	for _, check := range []projectInUseChecker{
		{"Shoots", c.projectInUseDueToShoots},
		{"Plants", c.projectInUseDueToPlants},
		{"BackupEntries", c.projectInUseDueToBackupEntries},
		{"Secrets", c.projectInUseDueToSecrets},
		{"Quotas", c.projectInUseDueToQuotas},
	} {
		projectInUse, err := check.checkFunc(*project.Spec.Namespace)
		if err != nil {
			return err
		}
		if projectInUse {
			projectLogger.Infof("[STALE PROJECT RECONCILE] Project is being marked as 'not stale' because it is used by %s", check.resource)
			return c.markProjectAsNotStale(ctx, gardenClient.Client(), project)
		}
	}

	projectLogger.Infof("[STALE PROJECT RECONCILE] Project is being marked as 'stale' because it is not being used by any resource")
	if err := c.markProjectAsStale(ctx, gardenClient.Client(), project, nowFunc); err != nil {
		return err
	}

	projectLogger.Infof("[STALE PROJECT RECONCILE] Project is stale since %s", *project.Status.StaleSinceTimestamp)
	if project.Status.StaleAutoDeleteTimestamp != nil {
		projectLogger.Infof("[STALE PROJECT RECONCILE] Project will be deleted at %s", *project.Status.StaleAutoDeleteTimestamp)
	}

	if project.Status.StaleAutoDeleteTimestamp == nil || nowFunc().UTC().Before(project.Status.StaleAutoDeleteTimestamp.UTC()) {
		return nil
	}

	projectLogger.Infof("[STALE PROJECT RECONCILE] Deleting Project now because it's auto-delete timestamp is expired")
	if err := common.ConfirmDeletion(ctx, gardenClient.Client(), project); err != nil {
		return err
	}
	return gardenClient.Client().Delete(ctx, project)
}

func (c *defaultStaleControl) projectInUseDueToShoots(namespace string) (bool, error) {
	shootList, err := c.shootLister.Shoots(namespace).List(labels.Everything())
	return len(shootList) > 0, err
}

func (c *defaultStaleControl) projectInUseDueToPlants(namespace string) (bool, error) {
	plantList, err := c.plantLister.Plants(namespace).List(labels.Everything())
	return len(plantList) > 0, err
}

func (c *defaultStaleControl) projectInUseDueToBackupEntries(namespace string) (bool, error) {
	backupEntryList, err := c.backupEntryLister.BackupEntries(namespace).List(labels.Everything())
	return len(backupEntryList) > 0, err
}

func (c *defaultStaleControl) projectInUseDueToSecrets(namespace string) (bool, error) {
	secretList, err := c.secretLister.Secrets(namespace).List(labels.Everything())
	if err != nil {
		return false, err
	}

	secretNames := computeSecretNames(secretList)
	if secretNames.Len() == 0 {
		return false, nil
	}

	return c.relevantSecretBindingsInUse(func(secretBinding *gardencorev1beta1.SecretBinding) bool {
		return secretBinding.SecretRef.Namespace == namespace && secretNames.Has(secretBinding.SecretRef.Name)
	})
}

func (c *defaultStaleControl) projectInUseDueToQuotas(namespace string) (bool, error) {
	quotaList, err := c.quotaLister.Quotas(namespace).List(labels.Everything())
	if err != nil {
		return false, err
	}

	quotaNames := computeQuotaNames(quotaList)
	if quotaNames.Len() == 0 {
		return false, nil
	}

	return c.relevantSecretBindingsInUse(func(secretBinding *gardencorev1beta1.SecretBinding) bool {
		for _, quota := range secretBinding.Quotas {
			return quota.Namespace == namespace && quotaNames.Has(quota.Name)
		}
		return false
	})
}

func (c *defaultStaleControl) relevantSecretBindingsInUse(isSecretBindingRelevantFunc func(secretBinding *gardencorev1beta1.SecretBinding) bool) (bool, error) {
	secretBindingList, err := c.secretBindingLister.List(labels.Everything())
	if err != nil {
		return false, err
	}

	namespaceToSecretBindingNames := make(map[string]sets.String)
	for _, secretBinding := range secretBindingList {
		if !isSecretBindingRelevantFunc(secretBinding) {
			continue
		}

		if _, ok := namespaceToSecretBindingNames[secretBinding.Namespace]; !ok {
			namespaceToSecretBindingNames[secretBinding.Namespace] = sets.NewString(secretBinding.Name)
		} else {
			namespaceToSecretBindingNames[secretBinding.Namespace].Insert(secretBinding.Name)
		}
	}

	return c.secretBindingInUse(namespaceToSecretBindingNames)
}

func (c *defaultStaleControl) markProjectAsNotStale(ctx context.Context, client client.Client, project *gardencorev1beta1.Project) error {
	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, client, project, func() error {
		project.Status.StaleSinceTimestamp = nil
		project.Status.StaleAutoDeleteTimestamp = nil
		return nil
	})
}

func (c *defaultStaleControl) markProjectAsStale(ctx context.Context, client client.Client, project *gardencorev1beta1.Project, nowFunc func() metav1.Time) error {
	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, client, project, func() error {
		if project.Status.StaleSinceTimestamp == nil {
			now := nowFunc()
			project.Status.StaleSinceTimestamp = &now
		}

		if project.Status.StaleSinceTimestamp.UTC().Add(time.Hour * 24 * time.Duration(*c.config.Controllers.Project.StaleGracePeriodDays)).After(nowFunc().UTC()) {
			// We reset the potentially set auto-delete timestamp here to allow changing the StaleExpirationTimeDays
			// configuration value and correctly applying the changes to all Projects that had already been assigned
			// such a timestamp.
			project.Status.StaleAutoDeleteTimestamp = nil
			return nil
		}

		// If the project got stale we compute an auto delete timestamp only if the configured stale grace period is
		// exceeded. Note that this might update the potentially already set auto-delete timestamp in case the
		// StaleExpirationTimeDays configuration value was changed.
		autoDeleteTimestamp := metav1.Time{Time: project.Status.StaleSinceTimestamp.Add(time.Hour * 24 * time.Duration(*c.config.Controllers.Project.StaleExpirationTimeDays))}

		// Don't allow to shorten the auto-delete timestamp as end-users might depend on the configured time. It may
		// only be extended.
		if project.Status.StaleAutoDeleteTimestamp == nil || autoDeleteTimestamp.After(project.Status.StaleAutoDeleteTimestamp.Time) {
			project.Status.StaleAutoDeleteTimestamp = &autoDeleteTimestamp
		}

		return nil
	})
}

func (c *defaultStaleControl) secretBindingInUse(namespaceToSecretBindingNames map[string]sets.String) (bool, error) {
	if len(namespaceToSecretBindingNames) == 0 {
		return false, nil
	}

	for namespace, secretBindingNames := range namespaceToSecretBindingNames {
		shootList, err := c.shootLister.Shoots(namespace).List(labels.Everything())
		if err != nil {
			return false, err
		}

		for _, shoot := range shootList {
			if secretBindingNames.Has(shoot.Spec.SecretBindingName) {
				return true, nil
			}
		}
	}

	return false, nil
}

// computeSecretNames determines the names of Secrets that are of type Opaque and don't have owner references to a
// Shoot.
func computeSecretNames(secretList []*corev1.Secret) sets.String {
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
func computeQuotaNames(quotaList []*gardencorev1beta1.Quota) sets.String {
	names := sets.NewString()

	for _, quota := range quotaList {
		names.Insert(quota.Name)
	}

	return names
}
