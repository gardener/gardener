// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
