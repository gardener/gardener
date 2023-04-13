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

package index_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/extensions/pkg/util/index"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func TestIndex(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extensions Util Index Suite")
}

var _ = Describe("Index", func() {
	Context("#SecretRefNamespaceIndexerFunc", func() {
		It("should return empty slice for non SecretBinding", func() {
			actual := index.SecretRefNamespaceIndexerFunc(&corev1.Secret{})
			Expect(actual).To(Equal([]string{}))
		})

		It("should return secretRef.namespace for SecretBinding", func() {
			secretBinding := &gardencorev1beta1.SecretBinding{
				SecretRef: corev1.SecretReference{
					Namespace: "garden-dev",
				},
			}

			actual := index.SecretRefNamespaceIndexerFunc(secretBinding)
			Expect(actual).To(Equal([]string{"garden-dev"}))
		})
	})

	Context("#SecretBindingNameIndexerFunc", func() {
		It("should return empty slice for non Shoot", func() {
			actual := index.SecretBindingNameIndexerFunc(&corev1.Pod{})
			Expect(actual).To(BeEmpty())
		})

		It("should return empty slice for nil secretBindingName", func() {
			shoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{},
			}

			actual := index.SecretBindingNameIndexerFunc(shoot)
			Expect(actual).To(BeEmpty())
		})

		It("should return spec.secretBindingName for Shoot", func() {
			shoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					SecretBindingName: pointer.String("foo"),
				},
			}

			actual := index.SecretBindingNameIndexerFunc(shoot)
			Expect(actual).To(Equal([]string{"foo"}))
		})
	})
})
