// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
