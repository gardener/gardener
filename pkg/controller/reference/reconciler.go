// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package reference

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// Reconciler checks the object in the given request for Secret or ConfiGMap references to further objects in order to
// protect them from deletions as long as they are still referenced.
type Reconciler struct {
	Client                      client.Client
	ConcurrentSyncs             *int
	NewObjectFunc               func() client.Object
	NewObjectListFunc           func() client.ObjectList
	GetNamespace                func(client.Object) string
	GetReferencedSecretNames    func(client.Object) []string
	GetReferencedConfigMapNames func(client.Object) []string
	ReferenceChangedPredicate   func(oldObj, newObj client.Object) bool
}

// Reconcile performs the check.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	obj := r.NewObjectFunc()
	if err := r.Client.Get(ctx, request.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	var (
		referencedSecretNames    = r.GetReferencedSecretNames(obj)
		referencedConfigMapNames = r.GetReferencedConfigMapNames(obj)
	)

	unreferencedSecretNames, err := r.getUnreferencedResources(ctx, obj, &corev1.SecretList{}, referencedSecretNames...)
	if err != nil {
		return reconcile.Result{}, err
	}
	unreferencedConfigMapNames, err := r.getUnreferencedResources(ctx, obj, &corev1.ConfigMapList{}, referencedConfigMapNames...)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := r.releaseUnreferencedResources(ctx, log, append(unreferencedSecretNames, unreferencedConfigMapNames...)...); err != nil {
		return reconcile.Result{}, err
	}

	// Remove finalizer from obj in case it's being deleted and not handled by Gardener anymore.
	if obj.GetDeletionTimestamp() != nil && !controllerutil.ContainsFinalizer(obj, gardencorev1beta1.GardenerName) {
		if controllerutil.ContainsFinalizer(obj, v1beta1constants.ReferenceProtectionFinalizerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.Client, obj, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}
		return reconcile.Result{}, nil
	}

	addedFinalizerToSecret, err := r.handleReferencedResources(ctx, log, "Secret", func() client.Object { return &corev1.Secret{} }, r.GetNamespace(obj), referencedSecretNames...)
	if err != nil {
		return reconcile.Result{}, err
	}
	addedFinalizerToConfigMap, err := r.handleReferencedResources(ctx, log, "ConfigMap", func() client.Object { return &corev1.ConfigMap{} }, r.GetNamespace(obj), referencedConfigMapNames...)
	if err != nil {
		return reconcile.Result{}, err
	}

	var (
		hasFinalizer   = controllerutil.ContainsFinalizer(obj, v1beta1constants.ReferenceProtectionFinalizerName)
		needsFinalizer = addedFinalizerToSecret || addedFinalizerToConfigMap
	)

	if needsFinalizer && !hasFinalizer {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, obj, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
		return reconcile.Result{}, nil
	}

	if !needsFinalizer && hasFinalizer {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.Client, obj, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

var (
	noGardenRole = utils.MustNewRequirement(v1beta1constants.GardenRole, selection.DoesNotExist)

	// UserManagedSelector is a selector for objects which are managed by users and not created by Gardener.
	UserManagedSelector = client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(noGardenRole)}
)

func (r *Reconciler) handleReferencedResources(
	ctx context.Context,
	log logr.Logger,
	kind string,
	newObjectFunc func() client.Object,
	namespace string,
	resourceNames ...string,
) (
	bool,
	error,
) {
	var (
		fns   []flow.TaskFn
		added = uint32(0)
	)

	for _, resourceName := range resourceNames {
		name := resourceName
		fns = append(fns, func(ctx context.Context) error {
			obj := newObjectFunc()
			if err := r.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, obj); err != nil {
				return err
			}

			// Don't handle Gardener managed secrets.
			if kind == "Secret" && obj.GetLabels()[v1beta1constants.GardenRole] != "" {
				return nil
			}

			atomic.StoreUint32(&added, 1)

			if !controllerutil.ContainsFinalizer(obj, v1beta1constants.ReferenceProtectionFinalizerName) {
				log.Info("Adding finalizer to object", "kind", kind, "obj", client.ObjectKeyFromObject(obj))
				if err := controllerutils.AddFinalizers(ctx, r.Client, obj, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
					return fmt.Errorf("failed to add finalizer to %s %s: %w", kind, client.ObjectKeyFromObject(obj), err)
				}
			}

			return nil
		})
	}

	return added != 0, flow.Parallel(fns...)(ctx)
}

func (r *Reconciler) releaseUnreferencedResources(
	ctx context.Context,
	log logr.Logger,
	resources ...client.Object,
) error {
	var fns []flow.TaskFn
	for _, resource := range resources {
		obj := resource

		gvk, err := apiutil.GVKForObject(obj, kubernetesscheme.Scheme)
		if err != nil {
			return fmt.Errorf("failed to identify GVK for object: %w", err)
		}

		fns = append(fns, func(ctx context.Context) error {
			if controllerutil.ContainsFinalizer(obj, v1beta1constants.ReferenceProtectionFinalizerName) {
				log.Info("Removing finalizer from object", "kind", gvk.Kind, "obj", client.ObjectKeyFromObject(obj))
				if err := controllerutils.RemoveFinalizers(ctx, r.Client, obj, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
					return fmt.Errorf("failed to remove finalizer from %s %s: %w", gvk.Kind, client.ObjectKeyFromObject(obj), err)
				}
			}
			return nil
		})
	}
	return flow.Parallel(fns...)(ctx)
}

func (r *Reconciler) getUnreferencedResources(
	ctx context.Context,
	reconciledObj client.Object,
	resourceList client.ObjectList,
	objectNames ...string,
) (
	[]client.Object,
	error,
) {
	var listOptions []client.ListOption
	if namespace := r.GetNamespace(reconciledObj); namespace != "" {
		listOptions = append(listOptions, client.InNamespace(namespace))
	}

	if err := r.Client.List(ctx, resourceList, append([]client.ListOption{UserManagedSelector}, listOptions...)...); err != nil {
		return nil, err
	}

	reconciledObjList := r.NewObjectListFunc()
	if err := r.Client.List(ctx, reconciledObjList, listOptions...); err != nil {
		return nil, err
	}

	referencedObjects := sets.New[string]()
	if err := meta.EachListItem(reconciledObjList, func(o runtime.Object) error {
		obj, ok := o.(client.Object)
		if !ok {
			return fmt.Errorf("failed converting runtime.Object to client.Object: %+v", o)
		}

		// Ignore own references if shoot is in deletion and references are not needed any more by Gardener.
		if obj.GetName() == reconciledObj.GetName() && obj.GetDeletionTimestamp() != nil && !controllerutil.ContainsFinalizer(obj, gardencorev1beta1.GardenerName) {
			return nil
		}

		referencedObjects.Insert(objectNames...)
		return nil
	}); err != nil {
		return nil, err
	}

	var objectsToRelease []client.Object
	if err := meta.EachListItem(resourceList, func(o runtime.Object) error {
		obj, ok := o.(client.Object)
		if !ok {
			return fmt.Errorf("failed converting runtime.Object to client.Object: %+v", o)
		}

		if !controllerutil.ContainsFinalizer(obj, v1beta1constants.ReferenceProtectionFinalizerName) {
			return nil
		}

		if referencedObjects.Has(obj.GetName()) {
			return nil
		}

		objectsToRelease = append(objectsToRelease, obj)
		return nil
	}); err != nil {
		return nil, err
	}

	return objectsToRelease, nil
}
