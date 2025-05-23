// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("DaemonSet", func() {
	DescribeTable("#PodManagedByDaemonSet", func(ownerRefs []metav1.OwnerReference, expectedResult bool) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: ownerRefs,
			},
		}

		Expect(PodManagedByDaemonSet(pod)).To(Equal(expectedResult))
	},
		Entry("OwnerRefs is 'nil'", nil, false),
		Entry("OwnerRefs is empty", []metav1.OwnerReference{}, false),
		Entry("OwnerRefs doesn't contain DaemonSet", []metav1.OwnerReference{{}}, false),
		Entry("DaemonSet is owner but no controller set", []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "DaemonSet"}}, false),
		Entry("DaemonSet is owner but not controller", []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "DaemonSet", Controller: ptr.To(false)}}, false),
		Entry("DaemonSet is owner and controller", []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "DaemonSet", Controller: ptr.To(true)}}, true),
		Entry("DaemonSet and other ref are owners", []metav1.OwnerReference{{}, {APIVersion: "apps/v1", Kind: "DaemonSet", Controller: ptr.To(true)}}, true),
	)
})
