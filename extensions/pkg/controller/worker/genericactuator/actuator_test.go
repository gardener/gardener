// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://wwr.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genericactuator

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Actuator", func() {
	Describe("listMachineClassSecrets", func() {
		const (
			ns      = "test-ns"
			purpose = "machineclass"
		)

		var (
			existing    *corev1.Secret
			expected    corev1.Secret
			allExisting []runtime.Object
			allExpected []interface{}
		)

		BeforeEach(func() {
			existing = &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      "new",
					Namespace: ns,
					Labels:    map[string]string{},
				},
			}
			allExisting = []runtime.Object{}
			allExpected = []interface{}{}
			expected = *existing.DeepCopy()
		})

		AfterEach(func() {
			a := &genericActuator{client: fake.NewFakeClient(allExisting...)}
			sl, err := a.listMachineClassSecrets(context.TODO(), ns)
			Expect(err).ToNot(HaveOccurred())
			Expect(sl).ToNot(BeNil())
			Expect(sl.Items).To(ConsistOf(allExpected...))
		})

		It("only classes with new label exists", func() {
			existing.Labels["gardener.cloud/purpose"] = purpose
			expected = *existing.DeepCopy()

			allExisting = append(allExisting, existing)
			allExpected = append(allExpected, expected)
		})
		It("only classes with old label exists", func() {
			existing.Labels["garden.sapcloud.io/purpose"] = purpose
			expected := *existing.DeepCopy()

			allExisting = append(allExisting, existing)
			allExpected = append(allExpected, expected)
		})
		It("secret is labeled with both labels", func() {
			existing.Labels["garden.sapcloud.io/purpose"] = purpose
			existing.Labels["gardener.cloud/purpose"] = purpose
			expected := *existing.DeepCopy()

			allExisting = append(allExisting, existing)
			allExpected = append(allExpected, expected)
		})
		It("one old and one new secret exists", func() {
			oldExisting := existing.DeepCopy()
			oldExisting.Name = "old-deprecated"
			oldExisting.Labels["garden.sapcloud.io/purpose"] = purpose

			existing.Labels["gardener.cloud/purpose"] = purpose
			expected := *existing.DeepCopy()
			expectedOld := *oldExisting.DeepCopy()

			allExisting = append(allExisting, existing, oldExisting)
			allExpected = append(allExpected, expected, expectedOld)
		})

	})
})
