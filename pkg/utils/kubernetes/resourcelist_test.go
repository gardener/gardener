// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Node", func() {
	Describe("IsNodeLabelAllowedForKubelet", func() {
		var (
			v1  = resource.MustParse("1")
			v2G = resource.MustParse("2G")
			v3  = resource.MustParse("3")
			v4G = resource.MustParse("4G")
		)

		It("should merge the values correctly into the main list", func() {
			var (
				listMain = corev1.ResourceList{
					"inMain": v1,
					"inBoth": v2G,
				}
				listExtra = corev1.ResourceList{
					"inExtra": v3,
					"inBoth":  v4G,
				}
			)

			MergeMinValuesIntoResourceList(listExtra, listMain)

			Expect(listMain).To(HaveLen(3))
			Expect(listMain["inMain"]).To(Equal(v1))
			Expect(listMain["inExtra"]).To(Equal(v3))
			Expect(listMain["inBoth"]).To(Equal(v2G))
		})

		It("should use the smaller value", func() {
			var (
				smallMainList = corev1.ResourceList{
					"cpu": v1,
				}
				largeExtraList = corev1.ResourceList{
					"cpu": v3,
				}
				largeMainList = corev1.ResourceList{
					"cpu": v1,
				}
				smallExtraList = corev1.ResourceList{
					"cpu": v3,
				}
			)

			MergeMinValuesIntoResourceList(smallExtraList, largeMainList)
			MergeMinValuesIntoResourceList(largeExtraList, smallMainList)

			Expect(smallMainList["cpu"]).To(Equal(v1))
			Expect(largeMainList["cpu"]).To(Equal(v1))
		})

		It("should not modify the read-only list", func() {
			var (
				listMain = corev1.ResourceList{
					"inMain": v1,
					"inBoth": v2G,
				}
				listExtra = corev1.ResourceList{
					"inExtra": v3,
					"inBoth":  v4G,
				}
			)

			MergeMinValuesIntoResourceList(listExtra, listMain)

			Expect(listExtra).To(HaveLen(2))
			Expect(listExtra["inExtra"]).To(Equal(v3))
			Expect(listExtra["inBoth"]).To(Equal(v4G))
		})

		It("should work correctly if the read-only list is nil", func() {
			var (
				listMain = corev1.ResourceList{
					"inMain": v1,
					"inBoth": v2G,
				}
			)

			MergeMinValuesIntoResourceList(nil, listMain)

			Expect(listMain).To(HaveLen(2))
		})
	})
})
