// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apiserver/registry/seedmanagement/managedseedset"
)

// REST implements a RESTStorage for ManagedSeedSet.
type REST struct {
	*genericregistry.Store
}

// ManagedSeedSetStorage implements the storage for ManagedSeedSets and their status subresource.
type ManagedSeedSetStorage struct {
	ManagedSeedSet *REST
	Status         *StatusREST
	Scale          *ScaleREST
}

// NewStorage creates a new ManagedSeedSetStorage object.
func NewStorage(optsGetter generic.RESTOptionsGetter) ManagedSeedSetStorage {
	managedSeedSetRest, managedSeedSetStatusRest := NewREST(optsGetter)

	return ManagedSeedSetStorage{
		ManagedSeedSet: managedSeedSetRest,
		Status:         managedSeedSetStatusRest,
		Scale:          &ScaleREST{store: managedSeedSetRest.Store},
	}
}

// NewREST returns a RESTStorage object that will work with ManagedSeedSet objects.
func NewREST(optsGetter generic.RESTOptionsGetter) (*REST, *StatusREST) {
	strategy := managedseedset.NewStrategy()
	statusStrategy := managedseedset.NewStatusStrategy()

	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &seedmanagement.ManagedSeedSet{} },
		NewListFunc:               func() runtime.Object { return &seedmanagement.ManagedSeedSetList{} },
		DefaultQualifiedResource:  seedmanagement.Resource("managedseedsets"),
		SingularQualifiedResource: seedmanagement.Resource("managedseedset"),
		EnableGarbageCollection:   true,

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,

		TableConvertor: newTableConvertor(),
	}
	options := &generic.StoreOptions{
		RESTOptions: optsGetter,
		AttrFunc:    managedseedset.GetAttrs,
	}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err)
	}

	statusStore := *store
	statusStore.UpdateStrategy = statusStrategy

	return &REST{store}, &StatusREST{store: &statusStore}
}

// StatusREST implements the REST endpoint for changing the status of a ManagedSeedSet.
type StatusREST struct {
	store *genericregistry.Store
}

var (
	_ rest.Storage = &StatusREST{}
	_ rest.Getter  = &StatusREST{}
	_ rest.Updater = &StatusREST{}
)

// New creates a new (empty) internal ManagedSeedSet object.
func (r *StatusREST) New() runtime.Object {
	return &seedmanagement.ManagedSeedSet{}
}

// Destroy cleans up its resources on shutdown.
func (r *StatusREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// Get retrieves the object from the storage. It is required to support Patch.
func (r *StatusREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return r.store.Get(ctx, name, options)
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	return r.store.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}

// Implement ShortNamesProvider
var _ rest.ShortNamesProvider = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"mss"}
}

// ScaleREST implements a Scale for ManagedSeedSet.
type ScaleREST struct {
	store *genericregistry.Store
}

// ScaleREST implements Patcher
var _ = rest.Patcher(&ScaleREST{})
var _ = rest.GroupVersionKindProvider(&ScaleREST{})

// GroupVersionKind returns GroupVersionKind for ManagedSeedSet Scale object.
func (r *ScaleREST) GroupVersionKind(_ schema.GroupVersion) schema.GroupVersionKind {
	return autoscalingv1.SchemeGroupVersion.WithKind("Scale")
}

// New creates a new (empty) Scale object.
func (r *ScaleREST) New() runtime.Object {
	return &autoscalingv1.Scale{}
}

// Destroy cleans up its resources on shutdown.
func (r *ScaleREST) Destroy() {
	// Given that underlying store is shared with REST,
	// we don't destroy it here explicitly.
}

// Get retrieves object from Scale storage.
func (r *ScaleREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	obj, err := r.store.Get(ctx, name, options)
	if err != nil {
		return nil, errors.NewNotFound(seedmanagement.Resource("managedseedsets/scale"), name)
	}
	mss := obj.(*seedmanagement.ManagedSeedSet)
	scale, err := scaleFromManagedSeedSet(mss)
	if err != nil {
		return nil, errors.NewBadRequest(fmt.Sprintf("%v", err))
	}
	return scale, nil
}

// Update alters scale subset of ManagedSeedSet object.
func (r *ScaleREST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, _ bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	obj, _, err := r.store.Update(
		ctx,
		name,
		&scaleUpdatedObjectInfo{name, objInfo},
		toScaleCreateValidation(createValidation),
		toScaleUpdateValidation(updateValidation),
		false,
		options,
	)
	if err != nil {
		return nil, false, err
	}
	mss := obj.(*seedmanagement.ManagedSeedSet)
	newScale, err := scaleFromManagedSeedSet(mss)
	if err != nil {
		return nil, false, errors.NewBadRequest(fmt.Sprintf("%v", err))
	}
	return newScale, false, nil
}

func toScaleCreateValidation(f rest.ValidateObjectFunc) rest.ValidateObjectFunc {
	return func(ctx context.Context, obj runtime.Object) error {
		scale, err := scaleFromManagedSeedSet(obj.(*seedmanagement.ManagedSeedSet))
		if err != nil {
			return err
		}
		return f(ctx, scale)
	}
}

func toScaleUpdateValidation(f rest.ValidateObjectUpdateFunc) rest.ValidateObjectUpdateFunc {
	return func(ctx context.Context, obj, old runtime.Object) error {
		newScale, err := scaleFromManagedSeedSet(obj.(*seedmanagement.ManagedSeedSet))
		if err != nil {
			return err
		}
		oldScale, err := scaleFromManagedSeedSet(old.(*seedmanagement.ManagedSeedSet))
		if err != nil {
			return err
		}
		return f(ctx, newScale, oldScale)
	}
}

// scaleFromManagedSeedSet returns a scale subresource for a ManagedSeedSet.
func scaleFromManagedSeedSet(mss *seedmanagement.ManagedSeedSet) (*autoscalingv1.Scale, error) {
	selector, err := metav1.LabelSelectorAsSelector(&mss.Spec.Selector)
	if err != nil {
		return nil, err
	}
	return &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{
			Name:              mss.Name,
			Namespace:         mss.Namespace,
			UID:               mss.UID,
			ResourceVersion:   mss.ResourceVersion,
			CreationTimestamp: mss.CreationTimestamp,
		},
		Spec: autoscalingv1.ScaleSpec{
			Replicas: ptr.Deref(mss.Spec.Replicas, 0),
		},
		Status: autoscalingv1.ScaleStatus{
			Replicas: mss.Status.Replicas,
			Selector: selector.String(),
		},
	}, nil
}

// scaleUpdatedObjectInfo transforms an existing ManagedSeedSet into an existing scale, then to a new scale,
// and finally to a new ManagedSeedSet.
type scaleUpdatedObjectInfo struct {
	name    string
	objInfo rest.UpdatedObjectInfo
}

func (i *scaleUpdatedObjectInfo) Preconditions() *metav1.Preconditions {
	return i.objInfo.Preconditions()
}

func (i *scaleUpdatedObjectInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error) {
	mss, ok := oldObj.DeepCopyObject().(*seedmanagement.ManagedSeedSet)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("expected existing object type to be ManagedSeedSet, got %T", mss))
	}
	if len(mss.ResourceVersion) == 0 {
		return nil, errors.NewNotFound(seedmanagement.Resource("managedseedsets/scale"), i.name)
	}

	// Transform the ManagedSeedSet into the old scale
	oldScale, err := scaleFromManagedSeedSet(mss)
	if err != nil {
		return nil, err
	}

	// Transform the old scale to a new scale
	newScaleObj, err := i.objInfo.UpdatedObject(ctx, oldScale)
	if err != nil {
		return nil, err
	}
	if newScaleObj == nil {
		return nil, errors.NewBadRequest("nil update passed to Scale")
	}
	scale, ok := newScaleObj.(*autoscalingv1.Scale)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("expected input object type to be Scale, got %T", newScaleObj))
	}

	// Validate precondition if specified (resourceVersion matching is handled by storage)
	if len(scale.UID) > 0 && scale.UID != mss.UID {
		return nil, errors.NewConflict(seedmanagement.Resource("managedseedsets/scale"), mss.Name,
			fmt.Errorf("precondition failed: UID in precondition: %v, UID in object meta: %v", scale.UID, mss.UID))
	}

	// Move replicas/resourceVersion fields to object and return
	mss.Spec.Replicas = ptr.To(scale.Spec.Replicas)
	mss.ResourceVersion = scale.ResourceVersion
	return mss, nil
}
