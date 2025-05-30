// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apiextensions "github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
)

// ControllerName is the name of this controller.
const ControllerName = "extension-required-runtime"

type extension struct {
	objectKind        string
	object            client.Object
	newObjectListFunc func() client.ObjectList
}

var runtimeClusterExtensions = []extension{
	{objectKind: extensionsv1alpha1.BackupBucketResource, object: &extensionsv1alpha1.BackupBucket{}, newObjectListFunc: func() client.ObjectList { return &extensionsv1alpha1.BackupBucketList{} }},
	{objectKind: extensionsv1alpha1.DNSRecordResource, object: &extensionsv1alpha1.DNSRecord{}, newObjectListFunc: func() client.ObjectList { return &extensionsv1alpha1.DNSRecordList{} }},
	{objectKind: extensionsv1alpha1.ExtensionResource, object: &extensionsv1alpha1.Extension{}, newObjectListFunc: func() client.ObjectList { return &extensionsv1alpha1.ExtensionList{} }},
}

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.KindToRequiredTypes == nil {
		r.KindToRequiredTypes = map[string]sets.Set[string]{}
	}
	if r.Lock == nil {
		r.Lock = &sync.RWMutex{}
	}

	r.clock = clock.RealClock{}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(
			&operatorv1alpha1.Extension{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&operatorv1alpha1.Garden{},
			handler.EnqueueRequestsFromMapFunc(r.MapGardenToExtensions(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Build(r)
	if err != nil {
		return err
	}

	for _, extension := range runtimeClusterExtensions {
		eventHandler := handler.EnqueueRequestsFromMapFunc(r.MapObjectKindToExtensions(
			mgr.GetLogger().WithValues("controller", ControllerName),
			extension.objectKind,
			extension.newObjectListFunc,
		))

		// Execute the mapper function at least once to initialize the `KindToRequiredTypes` map.
		// This is necessary for extension kinds which are registered but for which no extension objects exist in the
		// garden runtime cluster (e.g. when backups are disabled).
		// In such cases, no regular watch event is triggered, and the mapping function will not be executed.
		// Thus, the extension kind would never be part of the `KindToRequiredTypes` map
		// and the reconciler would not be able to decide whether the  Extension is required.
		if err = c.Watch(&controllerutils.HandleOnce[client.Object, reconcile.Request]{Handler: eventHandler}); err != nil {
			return err
		}

		if err := c.Watch(source.Kind[client.Object](mgr.GetCache(), extension.object, eventHandler, extensions.ObjectPredicate(), predicateutils.HasClass(extensionsv1alpha1.ExtensionClassGarden))); err != nil {
			return err
		}
	}

	return nil
}

// MapGardenToExtensions returns a mapping function that maps a given garden resource to all related extensions.
func (r *Reconciler) MapGardenToExtensions(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		garden, ok := obj.(*operatorv1alpha1.Garden)
		if !ok {
			log.Error(fmt.Errorf("expected Garden but got %#v", obj), "Unable to convert to Garden")
			return nil
		}

		extensionList := &operatorv1alpha1.ExtensionList{}
		if err := r.Client.List(ctx, extensionList); err != nil {
			log.Error(err, "Failed to list extensions")
			return nil
		}

		var (
			requests           []reconcile.Request
			requiredExtensions = operator.ComputeRequiredExtensionsForGarden(garden, extensionList)
		)

		for _, extension := range extensionList.Items {
			if slices.ContainsFunc(extension.Spec.Resources, func(resource gardencorev1beta1.ControllerResource) bool {
				return requiredExtensions.Has(gardenerutils.ExtensionsID(resource.Kind, resource.Type))
			}) {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: extension.Name, Namespace: extension.Namespace}})
			}
		}

		return requests
	}
}

// MapObjectKindToExtensions returns a mapper function for the given 'extensions.gardener.cloud' extension kind
// that lists all existing resources of the given kind and stores the respective types in the `KindToRequiredTypes` map.
// Afterwards, it returns all 'operator.gardener.cloud' Extensions that responsible for the given kind.
func (r *Reconciler) MapObjectKindToExtensions(log logr.Logger, objectKind string, newObjectListFunc func() client.ObjectList) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		log = log.WithValues("extensionKind", objectKind)

		listObj := newObjectListFunc()
		if err := r.Client.List(ctx, listObj); err != nil && !meta.IsNoMatchError(err) {
			// Let's ignore bootstrap situations where extension CRDs were not yet applied. They will be deployed
			// eventually by the garden controller.
			log.Error(err, "Failed to list extension objects")
			return nil
		}

		r.Lock.RLock()
		oldRequiredTypes, kindCalculated := r.KindToRequiredTypes[objectKind]
		r.Lock.RUnlock()
		newRequiredTypes := sets.New[string]()

		if err := meta.EachListItem(listObj, func(o runtime.Object) error {
			obj, err := apiextensions.Accessor(o)
			if err != nil {
				return err
			}

			if ptr.Deref(obj.GetExtensionSpec().GetExtensionClass(), extensionsv1alpha1.ExtensionClassShoot) != extensionsv1alpha1.ExtensionClassGarden {
				return nil
			}

			newRequiredTypes.Insert(obj.GetExtensionSpec().GetExtensionType())
			return nil
		}); err != nil {
			log.Error(err, "Failed while iterating over extension objects")
			return nil
		}

		// if there is no difference compared to before then exit early
		if kindCalculated && oldRequiredTypes.Equal(newRequiredTypes) {
			return nil
		}

		r.Lock.Lock()
		r.KindToRequiredTypes[objectKind] = newRequiredTypes
		r.Lock.Unlock()

		// List all extensions and queue those that are supporting resources for the
		// extension kind this particular reconciler is responsible for.
		extensionList := &operatorv1alpha1.ExtensionList{}
		if err := r.Client.List(ctx, extensionList); err != nil {
			log.Error(err, "Failed to list extensions")
			return nil
		}

		var requests []reconcile.Request
		for _, extension := range extensionList.Items {
			for _, resource := range extension.Spec.Resources {
				if resource.Kind == objectKind {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: extension.Name}})
					break
				}
			}
		}

		return requests
	}
}
