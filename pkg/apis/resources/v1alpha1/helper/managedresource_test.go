// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/resources/v1alpha1/helper"
)

var _ = Describe("Origin", func() {
	const (
		clusterID = "test"
		name      = "bar"
		namespace = "foo"
	)
	var managedResource *resourcesv1alpha1.ManagedResource

	BeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	})

	Describe("#OriginForManagedResource", func() {
		It("should return the ManagedResource key without clusterID", func() {
			Expect(OriginForManagedResource("", managedResource)).To(Equal(namespace + "/" + name))
		})

		It("should return the ManagedResource key with clusterID", func() {
			Expect(OriginForManagedResource(clusterID, managedResource)).To(Equal(clusterID + ":" + namespace + "/" + name))
		})
	})

	Describe("#SplitOrigin", func() {
		It("should complain about invalid format", func() {
			_, _, err := SplitOrigin("id:foo")
			Expect(err).To(MatchError(ContainSubstring("unexpected origin format")))
			_, _, err = SplitOrigin("id:foo:bar")
			Expect(err).To(MatchError(ContainSubstring("unexpected origin format")))
			_, _, err = SplitOrigin("foo")
			Expect(err).To(MatchError(ContainSubstring("unexpected origin format")))
			_, _, err = SplitOrigin("id/foo/bar")
			Expect(err).To(MatchError(ContainSubstring("unexpected origin format")))
		})

		It("should return the ManagedResource key and clusterID", func() {
			id, key, err := SplitOrigin(clusterID + ":" + namespace + "/" + name)
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal(clusterID))
			Expect(key).To(Equal(types.NamespacedName{Namespace: namespace, Name: name}))
		})

		It("should return the ManagedResource key and empty clusterID", func() {
			id, key, err := SplitOrigin(namespace + "/" + name)
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(BeEmpty())
			Expect(key).To(Equal(types.NamespacedName{Namespace: namespace, Name: name}))
		})
	})
})
