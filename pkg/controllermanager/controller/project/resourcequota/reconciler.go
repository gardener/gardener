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
)

const (
	shootCountResource = "count/shoots.core.gardener.cloud"
)

var (
	GardenerCreatedResourcesCounts = map[corev1.ResourceName]int{
		"count/configmaps": 2,
		"count/secrets":    4,
	}
)

// Reconciler reconciles ResourceQuotas in Project namespaces.
// It adapts them to allow all Shoots resources (like ConfigMaps and Secrets) to fit in the ResourceQuota.
type Reconciler struct {
	Client client.Client
	Config controllermanagerconfigv1alpha1.ProjectControllerConfiguration
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)
	log.Info(fmt.Sprintf("Reconciling ResourceQuota %s/%s", request.Namespace, request.Name))

	resourceQuota := &corev1.ResourceQuota{}
	if err := r.Client.Get(ctx, request.NamespacedName, resourceQuota); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if resourceQuota.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	if resourceQuota.Spec.Hard == nil {
		return reconcile.Result{}, nil
	}

	oldResourceQuota := resourceQuota.DeepCopy()
	patch := client.MergeFrom(oldResourceQuota)
	var maximumShoots int64

	if shootLimit, hasShootQuota := resourceQuota.Spec.Hard[shootCountResource]; hasShootQuota {
		maximumShoots = shootLimit.Value()
	} else {
		shootList := &metav1.PartialObjectMetadataList{}
		shootList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
		if err := r.Client.List(ctx, shootList, client.InNamespace(request.Namespace)); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed listing Shoots in namespace %s: %w", request.Namespace, err)
		}
		// Adjust to the current Shoot count + 1 to allow room for new Shoots
		maximumShoots = int64(len(shootList.Items) + 1)
	}

	if modified := r.adjustResourceQuota(resourceQuota, maximumShoots); modified {
		if err := r.Client.Patch(ctx, resourceQuota, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed updating ResourceQuota: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

// adjustResourceQuota adjusts the given ResourceQuota's limits for Gardener-created resources and returns true if any adjustments were made.
func (r *Reconciler) adjustResourceQuota(resourceQuota *corev1.ResourceQuota, shootLimit int64) (modified bool) {
	for resourceName, count := range GardenerCreatedResourcesCounts {
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
