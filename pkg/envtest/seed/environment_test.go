// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package envtestseed_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SeedTestEnvironment", func() {
	BeforeEach(func() {
		By("ensuring that garden namespace exists")
		Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "garden"}})).
			To(Or(Succeed(), BeAlreadyExistsError()))
	})

	It("should be able to manipulate Gardener Extension resources", func() {
		infrastructure := &extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "garden"}}
		Expect(testClient.Create(ctx, infrastructure)).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(infrastructure), infrastructure)).To(Succeed())
		Expect(testClient.Delete(ctx, infrastructure)).To(Succeed())
	})
})
