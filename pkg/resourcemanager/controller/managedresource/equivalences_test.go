// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Equivalences", func() {

	Describe("#NewEquivalences, #GetEquivalencesFor", func() {
		var (
			additionalEquivalences  [][]metav1.GroupKind
			expectedEquivalenceSets map[metav1.GroupKind]EquivalenceSet
		)

		BeforeEach(func() {
			additionalEquivalences = [][]metav1.GroupKind{}
			expectedEquivalenceSets = map[metav1.GroupKind]EquivalenceSet{}
		})

		AfterEach(func() {
			e := NewEquivalences(additionalEquivalences...)

			for gk, expectedEquivalenceSet := range expectedEquivalenceSets {
				By(fmt.Sprintf("%#v should be equivalent to %+v", gk, expectedEquivalenceSet))
				Expect(e.GetEquivalencesFor(gk)).To(Equal(expectedEquivalenceSet))
			}
		})

		It("no additonal equivalence sets (default equivalence sets)", func() {
			for _, equiList := range defaultEquivalences {
				for _, gk := range equiList {
					expectedEquivalenceSets[gk] = EquivalenceSet{}.Insert(equiList...)
				}
			}
		})

		It("single additional equivalence set", func() {
			equis := []metav1.GroupKind{
				{Group: "groupA", Kind: "kindA"},
				{Group: "groupB", Kind: "kindB"},
				{Group: "groupC", Kind: "kindC"},
			}
			additionalEquivalences = append(additionalEquivalences, equis)

			expectedSet := EquivalenceSet{}.Insert(equis...)
			for _, gk := range equis {
				expectedEquivalenceSets[gk] = expectedSet
			}
		})

		It("multiple additional (disjoint) equivalence sets", func() {
			equis := [][]metav1.GroupKind{
				{
					{Group: "groupA1", Kind: "kindA1"},
					{Group: "groupB1", Kind: "kindB1"},
					{Group: "groupC1", Kind: "kindC1"},
				},
				{
					{Group: "groupA2", Kind: "kindA2"},
					{Group: "groupB2", Kind: "kindB2"},
					{Group: "groupC2", Kind: "kindC2"},
				},
			}
			additionalEquivalences = append(additionalEquivalences, equis...)

			for _, equiSet := range equis {
				expectedSet := EquivalenceSet{}.Insert(equiSet...)
				for _, gk := range equiSet {
					expectedEquivalenceSets[gk] = expectedSet
				}
			}
		})

		It("multiple additional (mixed) equivalence sets", func() {
			equis := [][]metav1.GroupKind{
				{
					{Group: "groupA", Kind: "kindA"},
					{Group: "groupB", Kind: "kindB"},
				},
				{
					{Group: "groupB", Kind: "kindB"},
					{Group: "groupC", Kind: "kindC"},
				},
			}
			additionalEquivalences = append(additionalEquivalences, equis...)

			expectedSet := EquivalenceSet{}.Insert(equis[0]...).Insert(equis[1]...)

			for _, equiSet := range equis {
				for _, gk := range equiSet {
					expectedEquivalenceSets[gk] = expectedSet
				}
			}
		})
	})

})
