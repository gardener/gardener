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

package envtest_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("GardenerTestEnvironment", func() {
	BeforeEach(func() {
		By("ensuring that garden namespace exists")
		Expect(testClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "garden"}})).
			To(Or(Succeed(), BeAlreadyExistsError()))
	})

	It("should be able to manipulate resource from core.gardener.cloud/v1beta1", func() {
		project := &gardencorev1beta1.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
		Expect(testClient.Create(ctx, project)).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
		Expect(gutil.ConfirmDeletion(ctx, testClient, project)).To(Succeed())
		Expect(testClient.Delete(ctx, project)).To(Succeed())
	})

	It("should be able to manipulate resource from core.gardener.cloud/v1alpha1", func() {
		project := &gardencorev1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
		Expect(testClient.Create(ctx, project)).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
		Expect(gutil.ConfirmDeletion(ctx, testClient, project)).To(Succeed())
		Expect(testClient.Delete(ctx, project)).To(Succeed())
	})

	It("should be able to manipulate resource from seedmanagement.gardener.cloud/v1alpha1", func() {
		managedSeed := &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "garden"}}
		Expect(testClient.Create(ctx, managedSeed)).To(MatchError(ContainSubstring("ManagedSeed.seedmanagement.gardener.cloud \"foo\" is invalid")))
	})

	It("should be able to manipulate resource from settings.gardener.cloud/v1alpha1", func() {
		openIDConnectPreset := &settingsv1alpha1.OpenIDConnectPreset{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "garden"}}
		Expect(testClient.Create(ctx, openIDConnectPreset)).To(MatchError(ContainSubstring("OpenIDConnectPreset.settings.gardener.cloud \"foo\" is invalid")))
	})
})
