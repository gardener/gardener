// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"sort"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ByName returns a comparison function for sorting by name.
func ByName() SortBy {
	return func(o1, o2 client.Object) bool {
		return o1.GetName() < o2.GetName()
	}
}

// ByCreationTimestamp returns a comparison function for sorting by creation timestamp.
func ByCreationTimestamp() SortBy {
	return func(o1, o2 client.Object) bool {
		return o1.GetCreationTimestamp().Time.Before(o2.GetCreationTimestamp().Time)
	}
}

// SortBy the type of a "less" function that defines the ordering of its object arguments.
type SortBy func(o1, o2 client.Object) bool

// Sort sorts the items in the provided list objects according to the sort-by function.
func (sortBy SortBy) Sort(objList runtime.Object) {
	if !meta.IsListType(objList) {
		panic("provided <objList> is not a list type")
	}

	items, err := meta.ExtractList(objList)
	if err != nil {
		panic(err)
	}

	ps := &objectSorter{objects: items, compareFn: sortBy}
	sort.Sort(ps)

	if err := meta.SetList(objList, ps.objects); err != nil {
		panic(err)
	}
}

type objectSorter struct {
	objects   []runtime.Object
	compareFn SortBy
}

func (s *objectSorter) Len() int {
	return len(s.objects)
}

func (s *objectSorter) Swap(i, j int) {
	s.objects[i], s.objects[j] = s.objects[j], s.objects[i]
}

func (s *objectSorter) Less(i, j int) bool {
	return s.compareFn(
		s.objects[i].(client.Object),
		s.objects[j].(client.Object),
	)
}
