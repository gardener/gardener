// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// Reconciler checks the object in the given request for Secret, ConfigMap, or WorkloadIdentity references
// to further objects in order to protect them from deletions as long as they are still referenced.
type Reconciler struct {
	Client                             client.Client
	ConcurrentSyncs                    *int
	NewObjectFunc                      func() client.Object
	NewObjectListFunc                  func() client.ObjectList
	GetNamespace                       func(client.Object) string
	GetReferencedSecretNames           func(client.Object) []string
	GetReferencedConfigMapNames        func(client.Object) []string
	GetReferencedWorkloadIdentityNames func(client.Object) []string
	ReferenceChangedPredicate          func(oldObj, newObj client.Object) bool
}

// Reconcile performs the check.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	obj := r.NewObjectFunc()
	if err := r.Client.Get(ctx, request.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	var unreferencedResources []client.Object

	unreferencedSecrets, err := r.getUnreferencedResources(ctx, obj, &corev1.SecretList{}, r.GetReferencedSecretNames)
	if err != nil {
		return reconcile.Result{}, err
	}
	unreferencedResources = append(unreferencedResources, unreferencedSecrets...)

	unreferencedConfigMaps, err := r.getUnreferencedResources(ctx, obj, &corev1.ConfigMapList{}, r.GetReferencedConfigMapNames)
	if err != nil {
		return reconcile.Result{}, err
	}
	unreferencedResources = append(unreferencedResources, unreferencedConfigMaps...)

	unreferencedWorkloadIdentities := []client.Object{}
	if r.GetReferencedWorkloadIdentityNames != nil {
		unreferencedWorkloadIdentities, err = r.getUnreferencedResources(ctx, obj, &securityv1alpha1.WorkloadIdentityList{}, r.GetReferencedWorkloadIdentityNames)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	unreferencedResources = append(unreferencedResources, unreferencedWorkloadIdentities...)

	if err := r.releaseUnreferencedResources(ctx, log, unreferencedResources...); err != nil {
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

	addedFinalizerToSecret, err := r.handleReferencedResources(ctx, log, "Secret", func() client.Object { return &corev1.Secret{} }, r.GetNamespace(obj), r.GetReferencedSecretNames(obj)...)
	if err != nil {
		return reconcile.Result{}, err
	}
	addedFinalizerToConfigMap, err := r.handleReferencedResources(ctx, log, "ConfigMap", func() client.Object { return &corev1.ConfigMap{} }, r.GetNamespace(obj), r.GetReferencedConfigMapNames(obj)...)
	if err != nil {
		return reconcile.Result{}, err
	}
	addedFinalizerToWorkloadIdentity := false
	if r.GetReferencedWorkloadIdentityNames != nil {
		addedFinalizerToWorkloadIdentity, err = r.handleReferencedResources(ctx, log, "WorkloadIdentity", func() client.Object { return &securityv1alpha1.WorkloadIdentity{} }, r.GetNamespace(obj), r.GetReferencedWorkloadIdentityNames(obj)...)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	var (
		hasFinalizer   = controllerutil.ContainsFinalizer(obj, v1beta1constants.ReferenceProtectionFinalizerName)
		needsFinalizer = addedFinalizerToSecret || addedFinalizerToConfigMap || addedFinalizerToWorkloadIdentity
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

		fns = append(fns, func(ctx context.Context) error {
			if controllerutil.ContainsFinalizer(obj, v1beta1constants.ReferenceProtectionFinalizerName) {
				log.Info("Removing finalizer from object", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "obj", client.ObjectKeyFromObject(obj))
				if err := controllerutils.RemoveFinalizers(ctx, r.Client, obj, v1beta1constants.ReferenceProtectionFinalizerName); err != nil {
					return fmt.Errorf("failed to remove finalizer from %s %s: %w", obj.GetObjectKind().GroupVersionKind().Kind, client.ObjectKeyFromObject(obj), err)
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
	getReferencedObjectsFunc func(client.Object) []string,
) (
	[]client.Object,
	error,
) {
	var (
		reconciledObjListOptions []client.ListOption
		resourceListOptions      = []client.ListOption{UserManagedSelector}
		reconciledObjList        = r.NewObjectListFunc()
	)

	if namespace := r.GetNamespace(reconciledObj); namespace != "" {
		resourceListOptions = append(resourceListOptions, client.InNamespace(namespace))
	}

	if err := r.Client.List(ctx, resourceList, resourceListOptions...); err != nil {
		return nil, err
	}

	if namespace := reconciledObj.GetNamespace(); namespace != "" {
		reconciledObjListOptions = append(reconciledObjListOptions, client.InNamespace(namespace))
	}

	if err := r.Client.List(ctx, reconciledObjList, reconciledObjListOptions...); err != nil {
		return nil, err
	}

	referencedObjects := sets.New[string]()
	if err := meta.EachListItem(reconciledObjList, func(o runtime.Object) error {
		obj, ok := o.(client.Object)
		if !ok {
			return fmt.Errorf("failed converting runtime.Object to client.Object: %+v", o)
		}

		// Ignore own references if the object is in deletion and references are not needed anymore.
		if obj.GetName() == reconciledObj.GetName() && obj.GetDeletionTimestamp() != nil && !controllerutil.ContainsFinalizer(obj, gardencorev1beta1.GardenerName) {
			return nil
		}

		referencedObjects.Insert(getReferencedObjectsFunc(obj)...)
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
