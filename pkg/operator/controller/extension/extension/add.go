// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apiextensions "github.com/gardener/gardener/pkg/api/extensions"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension/admission"
	"github.com/gardener/gardener/pkg/operator/controller/extension/extension/controllerregistration"
	extensionruntime "github.com/gardener/gardener/pkg/operator/controller/extension/extension/runtime"
	operatorpredicate "github.com/gardener/gardener/pkg/operator/predicate"
	"github.com/gardener/gardener/pkg/utils/oci"
)

// ControllerName is the name of this controller.
const ControllerName = "extension"

type extension struct {
	objectKind        string
	object            client.Object
	newObjectListFunc func() client.ObjectList
}

var runtimeClusterExtensions = []extension{
	{extensionsv1alpha1.BackupBucketResource, &extensionsv1alpha1.BackupBucket{}, func() client.ObjectList { return &extensionsv1alpha1.BackupBucketList{} }},
	{extensionsv1alpha1.DNSRecordResource, &extensionsv1alpha1.DNSRecord{}, func() client.ObjectList { return &extensionsv1alpha1.DNSRecordList{} }},
	{extensionsv1alpha1.ExtensionResource, &extensionsv1alpha1.Extension{}, func() client.ObjectList { return &extensionsv1alpha1.ExtensionList{} }},
}

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, gardenClientMap clientmap.ClientMap) error {
	var err error

	r.runtimeClientSet, err = kubernetes.NewWithConfig(
		kubernetes.WithRESTConfig(mgr.GetConfig()),
		kubernetes.WithRuntimeAPIReader(mgr.GetAPIReader()),
		kubernetes.WithRuntimeClient(mgr.GetClient()),
		kubernetes.WithRuntimeCache(mgr.GetCache()),
	)
	if err != nil {
		return fmt.Errorf("failed creating runtime clientset: %w", err)
	}

	if gardenClientMap == nil {
		return fmt.Errorf("GardenClientMap must not be nil")
	}
	r.gardenClientMap = gardenClientMap

	r.clock = clock.RealClock{}
	r.recorder = mgr.GetEventRecorderFor(ControllerName + "-controller")

	if r.HelmRegistry == nil {
		r.HelmRegistry, err = oci.NewHelmRegistry()
		if err != nil {
			return fmt.Errorf("failed creating Helm registry: %w", err)
		}
	}

	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	r.lock = &sync.RWMutex{}
	r.kindToRequiredTypes = make(map[string]sets.Set[string])
	r.registeredExtensionResourceWatches = make(sets.Set[string])

	r.admission = admission.New(r.runtimeClientSet, r.recorder, r.GardenNamespace, r.HelmRegistry)
	r.controllerRegistration = controllerregistration.New(r.runtimeClientSet.Client(), r.recorder, r.GardenNamespace)
	r.runtime = extensionruntime.New(r.runtimeClientSet, r.recorder, r.GardenNamespace, r.HelmRegistry)

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&operatorv1alpha1.Extension{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.Extension.ConcurrentSyncs, 0),
		}).
		Watches(
			&operatorv1alpha1.Garden{},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapToAllExtensions), mapper.UpdateWithNew, mgr.GetLogger()),
			builder.WithPredicates(predicate.Or(
				operatorpredicate.GardenCreatedOrReconciledSuccessfully(),
				operatorpredicate.GardenDeletionTriggered(),
			)),
		).
		Build(r)
	if err != nil {
		return err
	}

	r.registerExtensionResourceWatchesFunc = func() error {
		for _, extension := range runtimeClusterExtensions {
			if r.registeredExtensionResourceWatches.Has(extension.objectKind) {
				continue
			}
			eventHandler := mapper.EnqueueRequestsFrom(
				ctx,
				mgr.GetCache(),
				r.MapObjectKindToExtensions(extension.objectKind, extension.newObjectListFunc),
				mapper.UpdateWithNew,
				c.GetLogger(),
			)

			// Execute the mapper function at least once to initialize the `kindToRequiredTypes` map.
			// This is necessary for extension kinds which are registered but for which no extension objects exist in the
			// seed (e.g. when backups are disabled). In such cases, no regular watch event would be triggered. Hence, the
			// mapping function would never be executed. Thus, the extension kind would never be part of the
			// `kindToRequiredTypes` map and the reconciler would not be able to decide whether the
			// ControllerInstallation is required.
			if err = c.Watch(&controllerutils.HandleOnce[client.Object, reconcile.Request]{Handler: eventHandler}); err != nil {
				return err
			}

			if err := c.Watch(source.Kind[client.Object](mgr.GetCache(), extension.object, eventHandler, extensions.ObjectPredicate())); err != nil {
				return err
			}
			r.registeredExtensionResourceWatches.Insert(extension.objectKind)
		}
		return nil
	}

	return nil
}

// MapObjectKindToExtensions returns a mapper function for the given extension kind that lists all existing
// extension resources of the given kind and stores the respective types in the `kindToRequiredTypes` map.
// Afterwards, it enqueue all Extensions for the runtime cluster that responsible for the given kind.
func (r *Reconciler) MapObjectKindToExtensions(objectKind string, newObjectListFunc func() client.ObjectList) mapper.MapFunc {
	return func(ctx context.Context, log logr.Logger, _ client.Reader, _ client.Object) []reconcile.Request {
		log = log.WithValues("extensionKind", objectKind)

		listObj := newObjectListFunc()
		if err := r.runtimeClientSet.Client().List(ctx, listObj); err != nil && !meta.IsNoMatchError(err) {
			// Let's ignore bootstrap situations where extension CRDs were not yet applied. They will be deployed
			// eventually by the garden controller.
			log.Error(err, "Failed to list extension objects")
			return nil
		}

		r.lock.RLock()
		oldRequiredTypes, kindCalculated := r.kindToRequiredTypes[objectKind]
		r.lock.RUnlock()
		newRequiredTypes := sets.New[string]()

		if err := meta.EachListItem(listObj, func(o runtime.Object) error {
			obj, err := apiextensions.Accessor(o)
			if err != nil {
				return err
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

		r.lock.Lock()
		r.kindToRequiredTypes[objectKind] = newRequiredTypes
		r.lock.Unlock()

		// List all extensions and queue those that are supporting resources for the
		// extension kind this particular reconciler is responsible for.
		extensionList := &operatorv1alpha1.ExtensionList{}
		if err := r.runtimeClientSet.Client().List(ctx, extensionList); err != nil {
			log.Error(err, "Failed to list Extensions")
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

// MapToAllExtensions returns reconcile.Request objects for all existing gardens in the system.
func (r *Reconciler) MapToAllExtensions(ctx context.Context, log logr.Logger, reader client.Reader, _ client.Object) []reconcile.Request {
	extensionList := &metav1.PartialObjectMetadataList{}
	extensionList.SetGroupVersionKind(operatorv1alpha1.SchemeGroupVersion.WithKind("ExtensionList"))
	if err := reader.List(ctx, extensionList); err != nil {
		log.Error(err, "Failed to list extensions")
		return nil
	}

	return mapper.ObjectListToRequests(extensionList)
}
