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

package mapper_test

import (
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/mapper"

	extensionshandler "github.com/gardener/gardener/extensions/pkg/handler"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("#ManagedResourceToSecretsMapper", func() {
	var m extensionshandler.Mapper

	BeforeEach(func() {
		m = mapper.ManagedResourceToSecretsMapper()
	})

	It("should do nothing, if Object is nil", func() {
		requests := m.Map(nil)
		Expect(requests).To(BeEmpty())
	})

	It("should do nothing, if Object is not a ManagedResource", func() {
		requests := m.Map(&corev1.Pod{})
		Expect(requests).To(BeEmpty())
	})

	It("should map to all secrets referenced by ManagedResource", func() {
		mr := &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: "secret-one"},
					{Name: "secret-two"},
				},
			},
		}

		requests := m.Map(mr)
		Expect(requests).To(ConsistOf(
			reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      mr.Spec.SecretRefs[0].Name,
				Namespace: mr.Namespace,
			}},
			reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      mr.Spec.SecretRefs[1].Name,
				Namespace: mr.Namespace,
			}},
		))
	})
})
