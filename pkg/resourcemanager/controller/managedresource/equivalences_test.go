// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

		It("no additional equivalence sets (default equivalence sets)", func() {
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
