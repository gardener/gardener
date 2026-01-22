// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcequota

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	annotationKeyConfigMapsPerShoot = "quota.gardener.cloud/configmaps-per-shoot"
	annotationKeySecretsPerShoot    = "quota.gardener.cloud/secrets-per-shoot"

	shootCountResource = "count/shoots.core.gardener.cloud"
)

// ResourceQuotaUsages describes the resource quota usages per shoot cluster in project namespaces.
type ResourceQuotaUsages struct {
	Annotation       string
	QuotaKey         corev1.ResourceName
	ExpectedPerShoot int
}

var (
	// PerShootQuotaDescriptors describes resources that Gardener creates per Shoot Cluster in the project namespace.
	// Exposed for testing.
	PerShootQuotaDescriptors = []ResourceQuotaUsages{
		{
			annotationKeyConfigMapsPerShoot,
			"count/configmaps",
			len(gardenerutils.GetShootProjectConfigMapSuffixes()),
		},
		{
			annotationKeySecretsPerShoot,
			"count/secrets",
			len(gardenerutils.GetShootProjectSecretSuffixes()),
		},
	}
)

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

	for _, resourceQuotaUsage := range PerShootQuotaDescriptors {
		usageAnnotation := resourceQuotaUsage.Annotation
		usageSpecKey := resourceQuotaUsage.QuotaKey
		expectedCount := resourceQuotaUsage.ExpectedPerShoot

		annotationCountString, ok := resourceQuota.Annotations[usageAnnotation]
		if !ok {
			if err := r.handleMissingAnnotation(ctx, log, resourceQuota, usageAnnotation, usageSpecKey, expectedCount); err != nil {
				return reconcile.Result{}, fmt.Errorf("error aligning resource quota %q in namespace %q: %w", resourceQuota.Name, resourceQuota.Namespace, err)
			}
			continue
		}

		annotationCount, err := strconv.Atoi(annotationCountString)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed converting resource quota annotation %q to int: %w", usageAnnotation, err)
		}

		if annotationCount != expectedCount {
			if err := r.handleAnnotationMismatch(ctx, log, resourceQuota, usageAnnotation, usageSpecKey, annotationCount, expectedCount); err != nil {
				return reconcile.Result{}, fmt.Errorf("error aligning resource quota %q in namespace %q: %w", resourceQuota.Name, resourceQuota.Namespace, err)
			}
		}
	}

	if err := r.Client.Patch(ctx, resourceQuota, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed updating resource quota %q in namespace %q: %w", resourceQuota.Name, resourceQuota.Namespace, err)
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) handleMissingAnnotation(ctx context.Context, log logr.Logger, resourceQuota *corev1.ResourceQuota, usageAnnotation string, usageSpecKey corev1.ResourceName, expectedCount int) error {
	maximumShootsInProject, err := r.getMaximumShootsInProject(ctx, *resourceQuota)
	if err != nil {
		return err
	}

	currentQuotaResource, ok := resourceQuota.Spec.Hard[usageSpecKey]
	if !ok {
		metav1.SetMetaDataAnnotation(&resourceQuota.ObjectMeta, usageAnnotation, strconv.Itoa(expectedCount))
		return nil
	}

	currentQuota := ptr.To(currentQuotaResource).Value()
	requiredQuota := int64(expectedCount) * maximumShootsInProject

	if currentQuota < requiredQuota {
		log.Info("Current quota is less than required quota, bumping up", "currentQuota", currentQuota, "requiredQuota", requiredQuota, "quotaType", usageSpecKey.String())
		resourceQuota.Spec.Hard[usageSpecKey] = resource.MustParse(strconv.Itoa(int(requiredQuota)))
	} else {
		log.Info("Current quota is sufficient for required quota, not changing quota", "currentQuota", currentQuota, "requiredQuota", requiredQuota, "quotaType", usageSpecKey.String())
	}

	metav1.SetMetaDataAnnotation(&resourceQuota.ObjectMeta, usageAnnotation, strconv.Itoa(expectedCount))
	return nil
}

func (r *Reconciler) handleAnnotationMismatch(ctx context.Context, log logr.Logger, resourceQuota *corev1.ResourceQuota, usageAnnotation string, usageSpecKey corev1.ResourceName, annotationCount, expectedCount int) error {
	log.Info("Bumping resource quota per shoot", "quotaType", usageSpecKey.String(), "from", annotationCount, "to", expectedCount)
	countDiff := int64(max(expectedCount-annotationCount, 0))

	if countDiff > 0 {
		maximum, err := r.getMaximumShootsInProject(ctx, *resourceQuota)
		if err != nil {
			return err
		}

		newQuota := ptr.To(resourceQuota.Spec.Hard[usageSpecKey]).Value() + maximum*countDiff
		newVal := strconv.Itoa(int(newQuota))
		log.Info("Updating resource quota with value", "quotaType", usageSpecKey.String(), "value", newVal)
		resourceQuota.Spec.Hard[usageSpecKey] = resource.MustParse(newVal)
	}

	metav1.SetMetaDataAnnotation(&resourceQuota.ObjectMeta, usageAnnotation, strconv.Itoa(expectedCount))
	return nil
}

func (r *Reconciler) getMaximumShootsInProject(ctx context.Context, resourceQuota corev1.ResourceQuota) (int64, error) {
	if limit, hasQuota := resourceQuota.Spec.Hard[corev1.ResourceName(shootCountResource)]; hasQuota {
		return limit.Value(), nil
	}
	shootList := &metav1.PartialObjectMetadataList{}
	shootList.SetGroupVersionKind(gardencorev1beta1.SchemeGroupVersion.WithKind("ShootList"))
	if err := r.Client.List(ctx, shootList, client.InNamespace(resourceQuota.Namespace)); err != nil {
		return 0, fmt.Errorf("could not list shoots in project namespace %q: %w", resourceQuota.Namespace, err)
	}
	return int64(len(shootList.Items)), nil
}
