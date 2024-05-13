// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package envtest_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	settingsv1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("GardenerTestEnvironment", func() {
	It("should be able to manipulate resource from core.gardener.cloud/v1beta1", func() {
		project := &gardencorev1beta1.Project{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-"}}
		Expect(testClient.Create(ctx, project)).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
		Expect(gardenerutils.ConfirmDeletion(ctx, testClient, project)).To(Succeed())
		Expect(testClient.Delete(ctx, project)).To(Succeed())
	})

	It("should be able to manipulate resource from seedmanagement.gardener.cloud/v1alpha1", func() {
		managedSeed := &seedmanagementv1alpha1.ManagedSeed{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: "garden"}}
		Expect(testClient.Create(ctx, managedSeed)).To(MatchError(ContainSubstring("ManagedSeed.seedmanagement.gardener.cloud \"\" is invalid")))
	})

	It("should be able to manipulate resource from settings.gardener.cloud/v1alpha1", func() {
		openIDConnectPreset := &settingsv1alpha1.OpenIDConnectPreset{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: testNamespace.Name}}
		Expect(testClient.Create(ctx, openIDConnectPreset)).To(MatchError(MatchRegexp("OpenIDConnectPreset.settings.gardener.cloud \"test-.+\" is invalid")))
	})

	It("should be able to manipulate resource from operations.gardener.cloud/v1alpha1", func() {
		bastion := &operationsv1alpha1.Bastion{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: testNamespace.Name}}
		Expect(testClient.Create(ctx, bastion)).To(MatchError(ContainSubstring("Bastion.operations.gardener.cloud \"\" is invalid")))
	})

	It("should be able to manipulate resource from security.gardener.cloud/v1alpha1", func() {
		credentialsBinding := &securityv1alpha1.CredentialsBinding{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-", Namespace: testNamespace.Name}}
		Expect(testClient.Create(ctx, credentialsBinding)).To(MatchError(MatchRegexp("CredentialsBinding.security.gardener.cloud \"test-.+\" is invalid")))
	})
})
