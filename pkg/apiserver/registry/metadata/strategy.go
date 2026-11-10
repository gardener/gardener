// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"
)

// strategy is the update strategy for the metadata subresource. It wraps the parent resource's update strategy
// and restricts updates to object metadata: `.spec`, `.status`, and any other non-metadata fields are restored
// from the stored object, so a request against the metadata subresource can only change `.metadata` (labels,
// annotations, finalizers, owner references, managed fields).
type strategy struct {
	rest.RESTUpdateStrategy
}

// newStrategy returns an update strategy for the metadata subresource wrapping the given parent update strategy.
// The parent strategy is reused for namespace scope, name generation, and object typing.
func newStrategy(parent rest.RESTUpdateStrategy) strategy {
	return strategy{RESTUpdateStrategy: parent}
}

// PrepareForUpdate restores everything but the object metadata from the stored object, so that only `.metadata`
// can be changed via the metadata subresource. It runs the parent strategy's `PrepareForUpdate` on the incoming
// metadata so that metadata-driven canonicalization (e.g. stripping the operation annotation and the resulting
// generation increment) behaves exactly as on a main-resource write. Any `.spec`/`.status` mutation the parent
// performs (e.g. field syncing, `Bastion` heartbeat) is discarded, because those must not change via this subresource.
func (s strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	// Overlay the incoming metadata onto the stored object so that `.spec`/`.status` equal the stored values. The
	// parent sees an object whose spec is unchanged, hence no spurious spec-diff generation increment fires. The
	// generation stems from the stored object (not the request), so it cannot be spoofed via the metadata subresource.
	canonicalized := overlayMetadata(obj, old)
	if canonicalized == nil {
		return
	}
	s.RESTUpdateStrategy.PrepareForUpdate(ctx, canonicalized, old)

	// The parent may have mutated `.spec`/`.status`. Discard those by overlaying the now-canonicalized metadata onto
	// another copy of the stored object, keeping only the parent's metadata changes. A generation increment the parent
	// performed (e.g. in response to the operation annotation) lives in `.metadata` and is carried over explicitly.
	result := overlayMetadata(canonicalized, old)
	if result == nil {
		return
	}
	if canonicalizedMeta, err := meta.Accessor(canonicalized); err == nil {
		mustSetGeneration(result, canonicalizedMeta.GetGeneration())
	}

	reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(result).Elem())
}

// mustSetGeneration sets the given generation on the object's metadata, ignoring objects without object metadata.
func mustSetGeneration(obj runtime.Object, generation int64) {
	if objMeta, err := meta.Accessor(obj); err == nil {
		objMeta.SetGeneration(generation)
	}
}

// overlayMetadata returns a deep copy of `old` with its metadata (except the generation) replaced by the metadata of
// `from`. We cannot reach `.spec`/`.status` through the `metav1.Object` accessor (they are not part of the interface
// and their types differ per resource), so copying metadata onto a copy of `old` keeps the subresource generic for all
// resource types without knowing their concrete `.spec`/`.status` fields. The generation is intentionally kept from
// `old` (not copied from `from`) so that it can only be changed by the parent strategy, never by the request. It
// returns `nil` if either object does not expose object metadata.
func overlayMetadata(from, old runtime.Object) runtime.Object {
	fromMeta, err := meta.Accessor(from)
	if err != nil {
		return nil
	}

	merged := old.DeepCopyObject()
	mergedMeta, err := meta.Accessor(merged)
	if err != nil {
		return nil
	}

	mergedMeta.SetLabels(fromMeta.GetLabels())
	mergedMeta.SetAnnotations(fromMeta.GetAnnotations())
	mergedMeta.SetFinalizers(fromMeta.GetFinalizers())
	mergedMeta.SetOwnerReferences(fromMeta.GetOwnerReferences())
	mergedMeta.SetManagedFields(fromMeta.GetManagedFields())

	return merged
}

// ValidateUpdate performs no resource-specific validation: object metadata is already validated by the generic
// update path (`rest.BeforeUpdate`), and `.spec`/`.status` cannot change through this subresource.
func (strategy) ValidateUpdate(_ context.Context, _, _ runtime.Object) field.ErrorList {
	return nil
}

// WarningsOnUpdate returns no warnings; warnings about `.spec` do not apply to a metadata-only update.
func (strategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}
