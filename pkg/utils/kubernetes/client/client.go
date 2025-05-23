// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	timeutils "github.com/gardener/gardener/pkg/utils/time"
)

type objectsRemaining []client.Object

// Error implements error.
func (n objectsRemaining) Error() string {
	out := make([]string, 0, len(n))
	for _, obj := range n {
		var typeID string
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gvk.Empty() {
			typeID = fmt.Sprintf("%T", obj)
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

type namespaceFinalizer struct{}

// NewNamespaceFinalizer instantiates a namespace finalizer.
func NewNamespaceFinalizer() Finalizer {
	return &namespaceFinalizer{}
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

	return c.SubResource("finalize").Update(ctx, namespace)
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
	time      timeutils.Ops
	finalizer Finalizer
}

// NewCleaner instantiates a new Cleaner with the given timeutils.Ops and finalizer.
func NewCleaner(time timeutils.Ops, finalizer Finalizer) Cleaner {
	return &cleaner{time, finalizer}
}

var defaultCleaner = NewCleaner(timeutils.DefaultOps(), defaultFinalizer)

// DefaultCleaner is the default Cleaner.
func DefaultCleaner() Cleaner {
	return defaultCleaner
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
	time timeutils.Ops
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

func gracePeriodIsPassed(obj client.Object, ops *CleanOptions, t timeutils.Ops) bool {
	if obj.GetDeletionTimestamp().IsZero() {
		return false
	}

	deleteOp := &client.DeleteOptions{}
	for _, op := range ops.DeleteOptions {
		op.ApplyToDelete(deleteOp)
	}

	gracePeriod := time.Second * time.Duration(ptr.Deref(deleteOp.GracePeriodSeconds, 0))
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
func NewVolumeSnapshotContentCleaner(time timeutils.Ops) Cleaner {
	return &volumeSnapshotContentCleaner{
		time: time,
	}
}

var defaultVolumeSnapshotContentCleaner = NewVolumeSnapshotContentCleaner(timeutils.DefaultOps())

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
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return NewObjectsRemaining(obj)
}

func ensureCollectionGone(ctx context.Context, c client.Client, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.List(ctx, list, opts...); err != nil {
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
			return nil
		}
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
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
			return nil
		}
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
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
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

// ApplyToObjects applies the passed function to all the objects in the list.
func ApplyToObjects(ctx context.Context, objectList client.ObjectList, fn func(ctx context.Context, object client.Object) error) error {
	taskFns := make([]flow.TaskFn, 0, meta.LenList(objectList))
	if err := meta.EachListItem(objectList, func(obj runtime.Object) error {
		object, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("expected client.Object but got %T", obj)
		}
		taskFns = append(taskFns, func(ctx context.Context) error {
			return fn(ctx, object)
		})
		return nil
	}); err != nil {
		return err
	}
	return flow.Parallel(taskFns...)(ctx)
}

// ApplyToObjectKinds applies the passed function to all the object lists for the passed kinds.
func ApplyToObjectKinds(ctx context.Context, fn func(kind string, objectList client.ObjectList) flow.TaskFn, kindToObjectList map[string]client.ObjectList) error {
	var taskFns []flow.TaskFn
	for kind, objectList := range kindToObjectList {
		taskFns = append(taskFns, fn(kind, objectList))
	}

	return flow.Parallel(taskFns...)(ctx)
}

// ForceDeleteObjects lists and finalizes all the objects in the passed namespace and deletes them.
func ForceDeleteObjects(c client.Client, namespace string, objectList client.ObjectList, opts ...client.ListOption) flow.TaskFn {
	return func(ctx context.Context) error {
		listOpts := &client.ListOptions{Namespace: namespace}
		listOpts.ApplyOptions(opts)
		if err := c.List(ctx, objectList, listOpts); err != nil {
			return err
		}

		return ApplyToObjects(ctx, objectList, func(ctx context.Context, object client.Object) error {
			if err := c.Delete(ctx, object); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			return controllerutils.RemoveAllFinalizers(ctx, c, object)
		})
	}
}
