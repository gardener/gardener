// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	utiltime "github.com/gardener/gardener/pkg/utils/time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/gardener/gardener/pkg/utils/flow"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type objectsRemaining []runtime.Object

var unknownKey = client.ObjectKey{Namespace: "<unknown>", Name: "<unknown>"}

// Error implements error.
func (n *objectsRemaining) Error() string {
	out := make([]string, 0, len(*n))
	for _, obj := range *n {
		key, err := client.ObjectKeyFromObject(obj)
		if err != nil {
			key = unknownKey
		}

		var typeID string
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Empty() {
			typeID = fmt.Sprintf("%T", (*n)[0])
		} else {
			typeID = gvk.String()
		}

		out = append(out, fmt.Sprintf("%s %s", typeID, key.String()))
	}
	return fmt.Sprintf("remaining objects are still present: %v", out)
}

// AreObjectsRemaining checks whether the given error is an 'objects remaining error'.
func AreObjectsRemaining(err error) bool {
	_, ok := err.(*objectsRemaining)
	return ok
}

// NewObjectsRemaining returns a new error with the remaining objects.
func NewObjectsRemaining(remaining runtime.Object) error {
	if meta.IsListType(remaining) {
		items, _ := meta.ExtractList(remaining)
		err := objectsRemaining(items)
		return &err
	}

	err := objectsRemaining{remaining}
	return &err
}

// RemainingObjects retrieves the remaining objects of an `AreObjectsRemaining` error.
//
// If the error does not match `AreObjectsRemaining`, this returns nil.
func RemainingObjects(err error) []runtime.Object {
	if nEmpty, ok := err.(*objectsRemaining); ok {
		return *nEmpty
	}
	return nil
}

// Delete deletes the target with the given options. If CollectionOptions is set, the collection
// matching the options is deleted with the given options.
// TODO: Delete this as soon as the controller-runtime has delete collection support:
// https://github.com/kubernetes-sigs/controller-runtime/pull/324
func Delete(ctx context.Context, c client.Client, obj runtime.Object, opts ...DeleteOptionFunc) error {
	o := &DeleteOptions{}
	o.ApplyOptions(opts)

	if meta.IsListType(obj) {
		return deleteCollection(ctx, c, obj, o)
	}
	return delete(ctx, c, obj, o)
}

func delete(ctx context.Context, c client.Client, obj runtime.Object, o *DeleteOptions) error {
	return c.Delete(ctx, obj, func(options *client.DeleteOptions) {
		*options = o.DeleteOptions
	})
}

func deleteCollection(ctx context.Context, c client.Client, list runtime.Object, o *DeleteOptions) error {
	if err := c.List(ctx, list, useListOptionsIfNotNil(o.CollectionOptions)); err != nil {
		return err
	}

	n := meta.LenList(list)
	if n == 0 {
		return nil
	}

	tasks := make([]flow.TaskFn, 0, n)
	if err := meta.EachListItem(list, func(obj runtime.Object) error {
		return client.IgnoreNotFound(delete(ctx, c, obj, o))
	}); err != nil {
		return err
	}

	return flow.Parallel(tasks...)(ctx)
}

// DeleteOptions are enhanced client.DeleteOptions that allow to delete a collection.
// TODO: Delete this as soon as the controller-runtime has delete collection support:
// https://github.com/kubernetes-sigs/controller-runtime/pull/324
type DeleteOptions struct {
	client.DeleteOptions
	CollectionOptions *client.ListOptions
}

// DeleteOptionFunc is a function that modifies DeleteOptions.
// TODO: Delete this as soon as the controller-runtime has delete collection support:
// https://github.com/kubernetes-sigs/controller-runtime/pull/324
type DeleteOptionFunc func(*DeleteOptions)

// ApplyOptions applies the DeleteOptionFuncs to the DeleteOptions.
// TODO: Delete this as soon as the controller-runtime has delete collection support:
// https://github.com/kubernetes-sigs/controller-runtime/pull/324
func (o *DeleteOptions) ApplyOptions(optFuncs []DeleteOptionFunc) {
	for _, optFunc := range optFuncs {
		optFunc(o)
	}
}

// CollectionMatching modifies the CollectionOptions of the DeleteOptions with the listOpts.
// TODO: Delete this as soon as the controller-runtime has delete collection support:
// https://github.com/kubernetes-sigs/controller-runtime/pull/324
func CollectionMatching(listOpts ...client.ListOptionFunc) DeleteOptionFunc {
	return func(opts *DeleteOptions) {
		if opts.CollectionOptions == nil {
			opts.CollectionOptions = &client.ListOptions{}
		}
		opts.CollectionOptions.ApplyOptions(listOpts)
	}
}

// GracePeriodSeconds sets the GracePeriodSeconds on DeleteOptions to the given amount.
// TODO: Delete this as soon as the controller-runtime has delete collection support:
// https://github.com/kubernetes-sigs/controller-runtime/pull/324
func GracePeriodSeconds(gp int64) DeleteOptionFunc {
	return func(opts *DeleteOptions) {
		client.GracePeriodSeconds(gp)(&opts.DeleteOptions)
	}
}

// TolerateErrorFunc is a function for tolerating errors.
type TolerateErrorFunc func(err error) bool

// CleanOptions are options to clean certain resources.
// If FinalizeGracePeriodSeconds is set, the finalizers of the resources are removed if the resources still
// exist after their targeted termination date plus the FinalizeGracePeriodSeconds amount.
type CleanOptions struct {
	DeleteOptions
	FinalizeGracePeriodSeconds *int64
	ErrorToleration            []TolerateErrorFunc
}

// ApplyOptions applies the OptFuncs to the CleanOptions.
func (o *CleanOptions) ApplyOptions(optFuncs []CleanOptionFunc) {
	for _, optFunc := range optFuncs {
		optFunc(o)
	}
}

// CleanOptionFunc is a function that modifies CleanOptions.
type CleanOptionFunc func(*CleanOptions)

// FinalizeGracePeriodSeconds specifies that a resource shall be finalized if it's been deleting
// without being gone beyond the deletion timestamp for the given seconds.
func FinalizeGracePeriodSeconds(s int64) CleanOptionFunc {
	return func(opts *CleanOptions) {
		opts.FinalizeGracePeriodSeconds = &s
	}
}

// TolerateErrors returns a CleanOptionFunc that adds the given functions
// to tolerate certain errors to the CleanOptions.
func TolerateErrors(fns ...TolerateErrorFunc) CleanOptionFunc {
	return func(opts *CleanOptions) {
		for _, fn := range fns {
			opts.ErrorToleration = append(opts.ErrorToleration, fn)
		}
	}
}

// DeleteWith specifies how to delete resources for cleaning them.
func DeleteWith(optFuncs ...DeleteOptionFunc) CleanOptionFunc {
	return func(opts *CleanOptions) {
		opts.DeleteOptions.ApplyOptions(optFuncs)
	}
}

func useListOptionsIfNotNil(newOpts *client.ListOptions) client.ListOptionFunc {
	return func(options *client.ListOptions) {
		if newOpts != nil {
			client.UseListOptions(newOpts)(options)
		}
	}
}

type finalizer struct{}

// NewFinalizer instantiates a default finalizer.
func NewFinalizer() Finalizer {
	return &finalizer{}
}

var defaultFinalizer = NewFinalizer()

// Finalize removes the finalizers (.meta.finalizers) of given resource.
func (f *finalizer) Finalize(ctx context.Context, c client.Client, obj runtime.Object) error {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	withFinalizers := obj.DeepCopyObject()
	acc.SetFinalizers(nil)
	return c.Patch(ctx, obj, client.MergeFrom(withFinalizers))
}

// HasFinalizers checks whether the given resource has finalizers (.meta.finalizers).
func (f *finalizer) HasFinalizers(obj runtime.Object) (bool, error) {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}

	return len(acc.GetFinalizers()) > 0, nil
}

type namespaceFinalizer struct {
	namespaceInterface typedcorev1.NamespaceInterface
}

// NewNamespaceFinalizer instantiates a namespace finalizer.
func NewNamespaceFinalizer(namespaceInterface typedcorev1.NamespaceInterface) Finalizer {
	return &namespaceFinalizer{namespaceInterface}
}

// Finalize removes the finalizers of given namespace resource.
// Because of legacy reasons namespaces have both .meta.finalizers and .spec.finalizers.
// Both of them are removed. An error is returned when the given resource is not a namespace.
func (f *namespaceFinalizer) Finalize(ctx context.Context, c client.Client, obj runtime.Object) error {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return errors.New("corev1.Namespace is expected")
	}

	namespace.SetFinalizers(nil)
	namespace.Spec.Finalizers = nil

	// TODO (ialidzhikov): Use controller-runtime client once subresources are
	// suported - https://github.com/kubernetes-sigs/controller-runtime/issues/172.
	_, err := f.namespaceInterface.Finalize(namespace)
	return err
}

// HasFinalizers checks whether the given namespace has finalizers
// (.meta.finalizers and .spec.finalizers).
func (f *namespaceFinalizer) HasFinalizers(obj runtime.Object) (bool, error) {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return false, errors.New("corev1.Namespace expected")
	}

	return len(namespace.Finalizers)+len(namespace.Spec.Finalizers) > 0, nil
}

type cleaner struct {
	time      utiltime.Ops
	finalizer Finalizer
}

// NewCleaner instantiates a new Cleaner with the given utiltime.Ops and finalizer.
func NewCleaner(time utiltime.Ops, finalizer Finalizer) Cleaner {
	return &cleaner{time, finalizer}
}

var defaultCleaner = NewCleaner(utiltime.DefaultOps(), defaultFinalizer)

// DefaultCleaner is the default Cleaner.
func DefaultCleaner() Cleaner {
	return defaultCleaner
}

// NewNamespaceCleaner instantiates a new Cleaner with ability to clean namespaces.
func NewNamespaceCleaner(namespaceInterface typedcorev1.NamespaceInterface) Cleaner {
	return NewCleaner(utiltime.DefaultOps(), NewNamespaceFinalizer(namespaceInterface))
}

// Clean deletes and optionally finalizes resources that expired their termination date.
func (o *cleaner) Clean(ctx context.Context, c client.Client, obj runtime.Object, opts ...CleanOptionFunc) error {
	cleanOptions := &CleanOptions{}
	cleanOptions.ApplyOptions(opts)

	if meta.IsListType(obj) {
		return o.cleanCollection(ctx, c, obj, cleanOptions)
	}
	return o.clean(ctx, c, obj, cleanOptions)
}

func (o *cleaner) doClean(ctx context.Context, c client.Client, obj runtime.Object, cleanOptions *CleanOptions) error {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	gracePeriod := time.Second
	if cleanOptions != nil && cleanOptions.FinalizeGracePeriodSeconds != nil {
		gracePeriod *= time.Duration(*cleanOptions.FinalizeGracePeriodSeconds)
	}

	if !acc.GetDeletionTimestamp().IsZero() && acc.GetDeletionTimestamp().Time.Add(gracePeriod).Before(o.time.Now()) {
		hasFinalizers, err := o.finalizer.HasFinalizers(obj)
		if err != nil {
			return err
		}

		if hasFinalizers {
			return o.finalizer.Finalize(ctx, c, obj)
		}
	}

	if err := delete(ctx, c, obj, &cleanOptions.DeleteOptions); err != nil {
		for _, tolerate := range cleanOptions.ErrorToleration {
			if tolerate(err) {
				return nil
			}
		}
		return err
	}
	return nil
}

var defaultGoneEnsurer = GoneEnsurerFunc(EnsureGone)

// EnsureGone implements GoneEnsurer.
func (f GoneEnsurerFunc) EnsureGone(ctx context.Context, c client.Client, obj runtime.Object, opts ...client.ListOptionFunc) error {
	return f(ctx, c, obj, opts...)
}

// DefaultGoneEnsurer is the default GoneEnsurer.
func DefaultGoneEnsurer() GoneEnsurer {
	return defaultGoneEnsurer
}

// EnsureGone ensures that the given object or list is gone.
func EnsureGone(ctx context.Context, c client.Client, obj runtime.Object, opts ...client.ListOptionFunc) error {
	if meta.IsListType(obj) {
		return ensureCollectionGone(ctx, c, obj, opts...)
	}
	return ensureGone(ctx, c, obj)
}

func ensureGone(ctx context.Context, c client.Client, obj runtime.Object) error {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return err
	}

	if err := c.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return NewObjectsRemaining(obj)
}

func ensureCollectionGone(ctx context.Context, c client.Client, list runtime.Object, opts ...client.ListOptionFunc) error {
	if err := c.List(ctx, list, opts...); err != nil {
		return err
	}

	if meta.LenList(list) > 0 {
		return NewObjectsRemaining(list)
	}
	return nil
}

// UseCleanOptions uses the CleanOptions, if they are non-nil.
func UseCleanOptions(newOpts *CleanOptions) CleanOptionFunc {
	return func(opts *CleanOptions) {
		*opts = *newOpts
	}
}

func (o *cleaner) clean(ctx context.Context, c client.Client, obj runtime.Object, cleanOptions *CleanOptions) error {
	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return err
	}

	if err := c.Get(ctx, key, obj); err != nil {
		return err
	}

	return o.doClean(ctx, c, obj, cleanOptions)
}

func (o *cleaner) cleanCollection(
	ctx context.Context,
	c client.Client,
	list runtime.Object,
	cleanOptions *CleanOptions,
) error {
	if err := c.List(ctx, list, useListOptionsIfNotNil(cleanOptions.CollectionOptions)); err != nil {
		return err
	}

	tasks := make([]flow.TaskFn, 0, meta.LenList(list))
	if err := meta.EachListItem(list, func(obj runtime.Object) error {
		tasks = append(tasks, func(ctx context.Context) error {
			return client.IgnoreNotFound(o.doClean(ctx, c, obj, cleanOptions))
		})
		return nil
	}); err != nil {
		return err
	}

	return flow.Parallel(tasks...)(ctx)
}

func (o *cleanerOps) cleanAndEnsureGone(ctx context.Context, c client.Client, obj runtime.Object, cleanOptions *CleanOptions) error {
	if err := o.Clean(ctx, c, obj, UseCleanOptions(cleanOptions)); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	key, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return err
	}

	if err := c.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return NewObjectsRemaining(obj)
}

func (o *cleanerOps) cleanCollectionAndEnsureGone(ctx context.Context, c client.Client, list runtime.Object, cleanOptions *CleanOptions) error {
	if err := o.Clean(ctx, c, list, UseCleanOptions(cleanOptions)); err != nil {
		return err
	}

	if err := c.List(ctx, list, useListOptionsIfNotNil(cleanOptions.CollectionOptions)); err != nil {
		return err
	}

	if meta.LenList(list) > 0 {
		return NewObjectsRemaining(list)
	}
	return nil
}

type cleanerOps struct {
	Cleaner
	GoneEnsurer
}

// CleanAndEnsureGone cleans the target resources. Afterwards, it checks, whether the target resource is still
// present. If yes, it errors with a NewObjectsRemaining error.
func (o *cleanerOps) CleanAndEnsureGone(ctx context.Context, c client.Client, obj runtime.Object, opts ...CleanOptionFunc) error {
	cleanOptions := &CleanOptions{}
	cleanOptions.ApplyOptions(opts)

	if err := o.Clean(ctx, c, obj, UseCleanOptions(cleanOptions)); err != nil {
		return err
	}

	return o.EnsureGone(ctx, c, obj, useListOptionsIfNotNil(cleanOptions.CollectionOptions))
}

// NewCleanOps instantiates new CleanOps with the given Cleaner and GoneEnsurer.
func NewCleanOps(cleaner Cleaner, ensurer GoneEnsurer) CleanOps {
	return &cleanerOps{cleaner, ensurer}
}

var defaultCleanerOps = NewCleanOps(DefaultCleaner(), DefaultGoneEnsurer())

// DefaultCleanOps are the default CleanOps.
func DefaultCleanOps() CleanOps {
	return defaultCleanerOps
}
