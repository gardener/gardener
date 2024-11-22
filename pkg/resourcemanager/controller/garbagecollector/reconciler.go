// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garbagecollector

import (
	"context"
	"time"

	"github.com/hashicorp/go-multierror"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/resourcemanager/apis/config"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
)

// Reconciler performs garbage collection.
type Reconciler struct {
	TargetClient          client.Client
	Config                config.GarbageCollectorControllerConfig
	Clock                 clock.Clock
	MinimumObjectLifetime *time.Duration
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(reconcileCtx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(reconcileCtx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(reconcileCtx, r.Config.SyncPeriod.Duration)
	defer cancel()

	log.Info("Starting garbage collection")
	defer log.Info("Garbage collection finished")

	var (
		labels                  = client.MatchingLabels{references.LabelKeyGarbageCollectable: references.LabelValueGarbageCollectable}
		objectsToGarbageCollect = map[objectId]struct{}{}
	)

	for _, resource := range []struct {
		kind     string
		listKind string
	}{
		{references.KindSecret, "SecretList"},
		{references.KindConfigMap, "ConfigMapList"},
	} {
		objList := &metav1.PartialObjectMetadataList{}
		objList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(resource.listKind))
		if err := r.TargetClient.List(ctx, objList, labels); err != nil {
			return reconcile.Result{}, err
		}

		for _, obj := range objList.Items {
			if obj.CreationTimestamp.Add(*r.MinimumObjectLifetime).UTC().After(r.Clock.Now().UTC()) {
				// Do not consider recently created objects for garbage collection.
				continue
			}

			objectsToGarbageCollect[objectId{resource.kind, obj.Namespace, obj.Name}] = struct{}{}
		}
	}

	var (
		items             []metav1.PartialObjectMetadata
		groupVersionKinds = []schema.GroupVersionKind{
			appsv1.SchemeGroupVersion.WithKind("DeploymentList"),
			appsv1.SchemeGroupVersion.WithKind("StatefulSetList"),
			appsv1.SchemeGroupVersion.WithKind("DaemonSetList"),
			batchv1.SchemeGroupVersion.WithKind("JobList"),
			corev1.SchemeGroupVersion.WithKind("PodList"),
			batchv1.SchemeGroupVersion.WithKind("CronJobList"),
			resourcesv1alpha1.SchemeGroupVersion.WithKind("ManagedResourceList"),
		}
	)

	for _, gvk := range groupVersionKinds {
		objList := &metav1.PartialObjectMetadataList{}
		objList.SetGroupVersionKind(gvk)
		if err := r.TargetClient.List(ctx, objList); err != nil {
			if !meta.IsNoMatchError(err) {
				return reconcile.Result{}, err
			}
		}
		items = append(items, objList.Items...)
	}

	for _, objectMeta := range items {
		for key, objectName := range objectMeta.Annotations {
			objectKind := references.KindFromAnnotationKey(key)
			if objectKind == "" || objectName == "" {
				continue
			}

			delete(objectsToGarbageCollect, objectId{objectKind, objectMeta.Namespace, objectName})
		}
	}

	var (
		results   = make(chan error, 1)
		wg        wait.Group
		errorList = &multierror.Error{ErrorFormat: errorsutils.NewErrorFormatFuncWithPrefix("Could not delete all unused resources")}
	)

	for id := range objectsToGarbageCollect {
		objId := id

		wg.StartWithContext(ctx, func(ctx context.Context) {
			var (
				meta = metav1.ObjectMeta{Namespace: objId.namespace, Name: objId.name}
				obj  client.Object
			)

			switch objId.kind {
			case references.KindSecret:
				obj = &corev1.Secret{ObjectMeta: meta}
			case references.KindConfigMap:
				obj = &corev1.ConfigMap{ObjectMeta: meta}
			default:
				return
			}

			log.Info("Delete resource",
				"kind", objId.kind,
				"namespace", objId.namespace,
				"name", objId.name,
			)

			if err := r.TargetClient.Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
				results <- err
			}
		})
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for err := range results {
		if err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	return reconcile.Result{Requeue: true, RequeueAfter: r.Config.SyncPeriod.Duration}, errorList.ErrorOrNil()
}

type objectId struct {
	kind      string
	namespace string
	name      string
}
