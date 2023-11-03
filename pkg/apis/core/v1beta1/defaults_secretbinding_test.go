// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1beta1_test

import (
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SecretBinding defaulting", func() {
	var obj *SecretBinding

	BeforeEach(func() {
		obj = &SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secretbinding",
				Namespace: "test",
			},
			SecretRef: corev1.SecretReference{
				Name: "secret",
			},
		}
	})

	It("should default secretRef namespace", func() {
		SetObjectDefaults_SecretBinding(obj)

		Expect(obj.SecretRef.Namespace).NotTo(BeNil())
		Expect(obj.SecretRef.Namespace).To(Equal("test"))
	})

	It("should not default secretRef namespace if it is already set", func() {
		obj.SecretRef.Namespace = "other"

		SetObjectDefaults_SecretBinding(obj)

		Expect(obj.SecretRef.Namespace).To(Equal("other"))
	})

	It("should default quotas namespace", func() {
		obj.Quotas = []corev1.ObjectReference{
			{
				Name:      "obj1",
				Namespace: "ns1",
			},
			{
				Name: "obj2",
			},
		}

		SetObjectDefaults_SecretBinding(obj)

		Expect(obj.Quotas[0].Namespace).To(Equal("ns1"))
		Expect(obj.Quotas[1].Namespace).To(Equal("test"))
	})
})
