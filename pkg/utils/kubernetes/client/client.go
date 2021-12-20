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

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/flow"
	utiltime "github.com/gardener/gardener/pkg/utils/time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type objectsRemaining []client.Object

// Error implements error.
func (n objectsRemaining) Error() string {
	out := make([]string, 0, len(n))
	for _, obj := range n {
		var typeID string
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Empty() {
			typeID = fmt.Sprintf("%T", n[0])
		} else {
			typeID = gvk.String()
		}

		out = append(out, fmt.Sprintf("%s %s", typeID, client.ObjectKeyFromObject(obj).String()))
	}
	return fmt.Sprintf("remaining objects are still present: %v", out)
}

// AreObjectsRemaining checks whether the given error is an 'objects remaining error'.
func AreObjectsRemaining(err error) bool {
	_, ok := err.(objectsRemaining)
	return ok
}

// NewObjectsRemaining returns a new error with the remaining objects.
func NewObjectsRemaining(obj runtime.Object) error {
	switch remaining := obj.(type) {
	case client.ObjectList:
		r := make(objectsRemaining, 0, meta.LenList(remaining))
		if err := meta.EachListItem(remaining, func(obj runtime.Object) error {
			r = append(r, obj.(client.Object))
			return nil
		}); err != nil {
			return err
		}
		return r
	case client.Object:
		return objectsRemaining{remaining}
	}
	return fmt.Errorf("type %T does neither implement client.Object nor client.ObjectList", obj)
}

type finalizer struct{}

// NewFinalizer instantiates a default finalizer.
func NewFinalizer() Finalizer {
	return &finalizer{}
}

var defaultFinalizer = NewFinalizer()

// Finalize removes the finalizers (.meta.finalizers) of given resource.
func (f *finalizer) Finalize(ctx context.Context, c client.Client, obj client.Object) error {
	withFinalizers := obj.DeepCopyObject().(client.Object)
	obj.SetFinalizers(nil)
	return c.Patch(ctx, obj, client.MergeFrom(withFinalizers))
}

// HasFinalizers checks whether the given resource has finalizers (.meta.finalizers).
func (f *finalizer) HasFinalizers(obj client.Object) (bool, error) {
	return len(obj.GetFinalizers()) > 0, nil
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
func (f *namespaceFinalizer) Finalize(ctx context.Context, c client.Client, obj client.Object) error {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return errors.New("corev1.Namespace is expected")
	}

	namespace.SetFinalizers(nil)
	namespace.Spec.Finalizers = nil

	// TODO (ialidzhikov): Use controller-runtime client once subresources are
	// supported - https://github.com/kubernetes-sigs/controller-runtime/issues/172.
	_, err := f.namespaceInterface.Finalize(ctx, namespace, kubernetes.DefaultUpdateOptions())
	return err
}

// HasFinalizers checks whether the given namespace has finalizers
// (.meta.finalizers and .spec.finalizers).
func (f *namespaceFinalizer) HasFinalizers(obj client.Object) (bool, error) {
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
func (cl *cleaner) Clean(ctx context.Context, c client.Client, obj runtime.Object, opts ...CleanOption) error {
	cleanOptions := &CleanOptions{}
	cleanOptions.ApplyOptions(opts)

	switch o := obj.(type) {
	case client.ObjectList:
		return cleanCollectionAction(ctx, c, o, cleanOptions, cl.doClean)
	case client.Object:
		return cleanAction(ctx, c, o, cleanOptions, cl.doClean)
	}
	return fmt.Errorf("type %T does neither implement client.Object nor client.ObjectList", obj)
}

func (cl *cleaner) doClean(ctx context.Context, c client.Client, obj client.Object, cleanOptions *CleanOptions) error {
	gracePeriod := time.Second
	if cleanOptions.FinalizeGracePeriodSeconds != nil {
		gracePeriod *= time.Duration(*cleanOptions.FinalizeGracePeriodSeconds)
	}

	if !obj.GetDeletionTimestamp().IsZero() && obj.GetDeletionTimestamp().Time.Add(gracePeriod).Before(cl.time.Now()) {
		hasFinalizers, err := cl.finalizer.HasFinalizers(obj)
		if err != nil {
			return err
		}

		if hasFinalizers {
			return cl.finalizer.Finalize(ctx, c, obj)
		}
	}

	if obj.GetDeletionTimestamp().IsZero() {
		if err := c.Delete(ctx, obj, cleanOptions.DeleteOptions...); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			if meta.IsNoMatchError(err) {
				return nil
			}
			for _, tolerate := range cleanOptions.ErrorToleration {
				if tolerate(err) {
					return nil
				}
			}
			return err
		}
	}
	return nil
}

var _ Cleaner = (*volumeSnapshotContentCleaner)(nil)

type volumeSnapshotContentCleaner struct {
	time utiltime.Ops
}

// Clean annotates the VolumeSnapshotContents so that they are cleaned up by the CSI snapshot controller.
func (v *volumeSnapshotContentCleaner) Clean(ctx context.Context, c client.Client, obj runtime.Object, opts ...CleanOption) error {
	cleanOptions := &CleanOptions{}
	cleanOptions.ApplyOptions(opts)

	switch o := obj.(type) {
	case client.ObjectList:
		return cleanCollectionAction(ctx, c, o, cleanOptions, v.triggerVolumeSnapshotDeletion)
	case client.Object:
		return cleanAction(ctx, c, o, cleanOptions, v.triggerVolumeSnapshotDeletion)
	}
	return fmt.Errorf("type %T does neither implement client.Object nor client.ObjectList", obj)
}

func gracePeriodIsPassed(obj client.Object, ops *CleanOptions, t utiltime.Ops) bool {
	if obj.GetDeletionTimestamp().IsZero() {
		return false
	}

	deleteOp := &client.DeleteOptions{}
	for _, op := range ops.DeleteOptions {
		op.ApplyToDelete(deleteOp)
	}

	gracePeriod := time.Second * time.Duration(pointer.Int64PtrDerefOr(deleteOp.GracePeriodSeconds, 0))
	return obj.GetDeletionTimestamp().Time.Add(gracePeriod).Before(t.Now())
}

const (
	annVolumeSnapshotBeingDeleted = "snapshot.storage.kubernetes.io/volumesnapshot-being-deleted"
	annVolumeSnapshotBeingCreated = "snapshot.storage.kubernetes.io/volumesnapshot-being-created"
)

func (v *volumeSnapshotContentCleaner) triggerVolumeSnapshotDeletion(ctx context.Context, c client.Client, obj client.Object, ops *CleanOptions) error {
	if !gracePeriodIsPassed(obj, ops, v.time) {
		return nil
	}

	_, annDeleted := obj.GetAnnotations()[annVolumeSnapshotBeingDeleted]
	_, annCreated := obj.GetAnnotations()[annVolumeSnapshotBeingCreated]

	if annDeleted && !annCreated {
		return nil
	}

	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))

	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(make(map[string]string))
	}

	// annVolumeSnapshotBeingDeleted triggers the CSI sidecar to delete the snapshot on the infrastructure.
	// annVolumeSnapshotBeingCreated must not exist if we want the controller to delete the snapshot.
	// See https://github.com/kubernetes-csi/external-snapshotter/blob/138d310e5d2d102fcf90df96d7a8aaf6690ce76e/pkg/sidecar-controller/snapshot_controller.go#L563
	obj.GetAnnotations()[annVolumeSnapshotBeingDeleted] = "yes"
	delete(obj.GetAnnotations(), annVolumeSnapshotBeingCreated)

	return client.IgnoreNotFound(c.Patch(ctx, obj, patch))
}

// NewVolumeSnapshotContentCleaner instantiates a new Cleaner with ability to clean VolumeSnapshotContents
// **after** they got deleted and the given deletion grace period is passed.
func NewVolumeSnapshotContentCleaner(time utiltime.Ops) Cleaner {
	return &volumeSnapshotContentCleaner{
		time: time,
	}
}

var defaultVolumeSnapshotContentCleaner = NewVolumeSnapshotContentCleaner(utiltime.DefaultOps())

// DefaultVolumeSnapshotContentCleaner is the default cleaner for VolumeSnapshotContents.
// The VolumeSnapshotCleaner initiates the deletion of VolumeSnapshots **after** they got deleted
// and the given deletion grace period is passed.
func DefaultVolumeSnapshotContentCleaner() Cleaner {
	return defaultVolumeSnapshotContentCleaner
}

var defaultGoneEnsurer = GoneEnsurerFunc(EnsureGone)

// EnsureGone implements GoneEnsurer.
func (f GoneEnsurerFunc) EnsureGone(ctx context.Context, c client.Client, obj runtime.Object, opts ...client.ListOption) error {
	return f(ctx, c, obj, opts...)
}

// DefaultGoneEnsurer is the default GoneEnsurer.
func DefaultGoneEnsurer() GoneEnsurer {
	return defaultGoneEnsurer
}

// GoneBeforeEnsurer returns an implementation of `GoneEnsurer` which is time aware.
// It ensures that only resources created <before> are deleted.
func GoneBeforeEnsurer(before time.Time) GoneEnsurer {
	return &beforeGoneEnsurer{
		time: before,
	}
}

type beforeGoneEnsurer struct {
	time time.Time
}

// EnsureGone ensures that no given object or objects in the list are deleted, if they were created before the given time.
func (b *beforeGoneEnsurer) EnsureGone(ctx context.Context, c client.Client, obj runtime.Object, opts ...client.ListOption) error {
	if err := EnsureGone(ctx, c, obj, opts...); err != nil {
		remainingObjs, ok := err.(objectsRemaining)
		if !ok {
			return err
		}

		var relevants []client.Object
		for _, remaining := range remainingObjs {
			if remaining.GetCreationTimestamp().Time.Before(b.time) {
				relevants = append(relevants, remaining)
			}
		}

		if len(relevants) > 0 {
			return objectsRemaining(relevants)
		}
	}
	return nil
}

// EnsureGone ensures that the given object or list is gone.
func EnsureGone(ctx context.Context, c client.Client, obj runtime.Object, opts ...client.ListOption) error {
	switch o := obj.(type) {
	case client.ObjectList:
		return ensureCollectionGone(ctx, c, o, opts...)
	case client.Object:
		return ensureGone(ctx, c, o)
	}
	return fmt.Errorf("type %T does neither implement client.Object nor client.ObjectList", obj)
}

func ensureGone(ctx context.Context, c client.Client, obj client.Object) error {
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		if meta.IsNoMatchError(err) {
			return nil
		}
		return err
	}
	return NewObjectsRemaining(obj)
}

func ensureCollectionGone(ctx context.Context, c client.Client, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.List(ctx, list, opts...); err != nil && !meta.IsNoMatchError(err) {
		return err
	}

	if meta.LenList(list) > 0 {
		return NewObjectsRemaining(list)
	}
	return nil
}

type actionFunc func(ctx context.Context, c client.Client, obj client.Object, cleanOptions *CleanOptions) error

func cleanAction(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	cleanOptions *CleanOptions,
	action actionFunc,
) error {
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); client.IgnoreNotFound(err) != nil {
		return err
	}

	return action(ctx, c, obj, cleanOptions)
}

func cleanCollectionAction(
	ctx context.Context,
	c client.Client,
	list client.ObjectList,
	cleanOptions *CleanOptions,
	action actionFunc,
) error {
	if err := c.List(ctx, list, cleanOptions.ListOptions...); err != nil {
		if meta.IsNoMatchError(err) {
			return nil
		}
		return err
	}

	tasks := make([]flow.TaskFn, 0, meta.LenList(list))
	if err := meta.EachListItem(list, func(obj runtime.Object) error {
		tasks = append(tasks, func(ctx context.Context) error {
			return action(ctx, c, obj.(client.Object), cleanOptions)
		})
		return nil
	}); err != nil {
		return err
	}

	return flow.Parallel(tasks...)(ctx)
}

type cleanerOps struct {
	cleaners []Cleaner
	GoneEnsurer
}

// CleanAndEnsureGone cleans the target resources. Afterwards, it checks, whether the target resource is still
// present. If yes, it errors with a NewObjectsRemaining error.
func (o *cleanerOps) CleanAndEnsureGone(ctx context.Context, c client.Client, obj runtime.Object, opts ...CleanOption) error {
	cleanOptions := &CleanOptions{}
	cleanOptions.ApplyOptions(opts)

	for _, cle := range o.cleaners {
		if err := cle.Clean(ctx, c, obj, opts...); err != nil {
			return err
		}
	}

	return o.EnsureGone(ctx, c, obj, cleanOptions.ListOptions...)
}

// NewCleanOps instantiates new CleanOps with the given Cleaner and GoneEnsurer.
func NewCleanOps(ensurer GoneEnsurer, cleaner ...Cleaner) CleanOps {
	return &cleanerOps{cleaner, ensurer}
}

var defaultCleanerOps = NewCleanOps(DefaultGoneEnsurer(), DefaultCleaner())

// DefaultCleanOps are the default CleanOps.
func DefaultCleanOps() CleanOps {
	return defaultCleanerOps
}
