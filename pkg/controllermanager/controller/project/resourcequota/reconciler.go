// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcequota

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const shootCountResource = "count/shoots.core.gardener.cloud"

var gardenerCreatedResourcesCounts = map[corev1.ResourceName]int{
	"count/configmaps": len(gardenerutils.GetShootProjectConfigMapSuffixes()),
	"count/secrets":    len(gardenerutils.GetShootProjectSecretSuffixes()),
}

// Reconciler reconciles ResourceQuotas in Project namespaces.
// It ensures that ResourceQuota objects in project namespaces allow enough resources for all possible Shoot resources.
// It checks the current quota limits and adjusts them if needed, so that the namespace can accommodate the maximum number of Shoots and their related resources (like ConfigMaps and Secrets) according to the configured limits.
type Reconciler struct {
	Client client.Client
	Config controllermanagerconfigv1alpha1.ProjectControllerConfiguration
}

// Reconcile adjusts the ResourceQuota in a Project namespace to ensure that it can accommodate all Shoots, according to the Shoot limit.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	resourceQuota := &corev1.ResourceQuota{}
	if err := r.Client.Get(ctx, request.NamespacedName, resourceQuota); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if resourceQuota.DeletionTimestamp != nil || resourceQuota.Spec.Hard == nil {
		return reconcile.Result{}, nil
	}

	patch := client.MergeFrom(resourceQuota.DeepCopy())
	var maximumShoots int64

	if shootLimit, hasShootQuota := resourceQuota.Spec.Hard[shootCountResource]; hasShootQuota {
		maximumShoots = shootLimit.Value()
	} else {
		shootList := &metav1.PartialObjectMetadataList{}
		shootList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
		if err := r.Client.List(ctx, shootList, client.InNamespace(request.Namespace)); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed listing Shoots in namespace %s: %w", request.Namespace, err)
		}
		// Adjust to the current Shoot count
		maximumShoots = int64(len(shootList.Items))
	}

	if modified := r.adjustResourceQuota(resourceQuota, maximumShoots); modified {
		if err := r.Client.Patch(ctx, resourceQuota, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed patching ResourceQuota: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

// adjustResourceQuota adjusts the given ResourceQuota's limits for Gardener-created resources and returns true if any adjustments were made.
func (r *Reconciler) adjustResourceQuota(resourceQuota *corev1.ResourceQuota, shootLimit int64) (modified bool) {
	for resourceName, count := range gardenerCreatedResourcesCounts {
		resourceLimit, specified := resourceQuota.Spec.Hard[resourceName]
		if !specified {
			continue
		}
		minRequiredLimit := int64(count) * shootLimit

		if resourceLimit.Value() < minRequiredLimit {
			modified = true
			resourceQuota.Spec.Hard[resourceName] = resource.MustParse(strconv.FormatInt(minRequiredLimit, 10))
		}
	}
	return
}
