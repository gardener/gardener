// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	"sort"

	"helm.sh/helm/v3/pkg/releaseutil"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

var _ = sort.Interface(referenceSorter{})

type referenceSorter struct {
	keys []string
	refs []resourcesv1alpha1.ObjectReference
}

func newReferenceSorter(refs []resourcesv1alpha1.ObjectReference) sort.Interface {
	// compute keys only once
	keys := make([]string, len(refs))
	for i, ref := range refs {
		keys[i] = objectKeyByReference(ref)
	}

	return referenceSorter{
		keys: keys,
		refs: refs,
	}
}

func sortObjectReferences(refs []resourcesv1alpha1.ObjectReference) {
	s := newReferenceSorter(refs)
	sort.Sort(s)
}

func (r referenceSorter) Len() int {
	return len(r.refs)
}

func (r referenceSorter) Less(i, j int) bool {
	return r.keys[i] < r.keys[j]
}

func (r referenceSorter) Swap(i, j int) {
	r.keys[i], r.keys[j] = r.keys[j], r.keys[i]
	r.refs[i], r.refs[j] = r.refs[j], r.refs[i]
}

type kindSorter struct {
	ordering map[string]int
	objects  []object
}

func newKindSorter(obj []object, s releaseutil.KindSortOrder) *kindSorter {
	o := make(map[string]int, len(s))
	for v, k := range s {
		o[k] = v
	}

	return &kindSorter{
		objects:  obj,
		ordering: o,
	}
}

func (k *kindSorter) Len() int { return len(k.objects) }

func (k *kindSorter) Swap(i, j int) { k.objects[i], k.objects[j] = k.objects[j], k.objects[i] }

func (k *kindSorter) Less(i, j int) bool {
	a := k.objects[i]
	b := k.objects[j]
	first, aok := k.ordering[a.obj.GetKind()]
	second, bok := k.ordering[b.obj.GetKind()]

	if !aok && !bok {
		// if both are unknown then sort alphabetically by kind and name
		if a.obj.GetKind() != b.obj.GetKind() {
			return a.obj.GetKind() < b.obj.GetKind()
		}
		return a.obj.GetName() < b.obj.GetName()
	}

	// unknown kind is last
	if !aok {
		return false
	}
	if !bok {
		return true
	}

	// if same kind sub sort alphanumeric
	if first == second {
		return a.obj.GetName() < b.obj.GetName()
	}
	// sort different kinds
	return first < second
}
func sortByKind(resourceObject []object) []object {
	ordering := releaseutil.InstallOrder
	ks := newKindSorter(resourceObject, ordering)
	sort.Sort(ks)
	return ks.objects
}
