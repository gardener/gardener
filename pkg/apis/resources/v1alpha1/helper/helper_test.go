// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
