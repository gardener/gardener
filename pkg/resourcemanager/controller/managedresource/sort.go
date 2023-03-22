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

package managedresource

import (
	"sort"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	chartrenderer "github.com/gardener/gardener/pkg/chartrenderer"
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

func newKindSorter(obj []object, s chartrenderer.SortOrder) *kindSorter {
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
	ordering := chartrenderer.InstallOrder
	ks := newKindSorter(resourceObject, ordering)
	sort.Sort(ks)
	return ks.objects
}
