// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package indexer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/api/indexer"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

var _ = Describe("Security", func() {
	var indexer *fakeFieldIndexer

	BeforeEach(func() {
		indexer = &fakeFieldIndexer{}
	})

	DescribeTable("#AddCredentialsBindingCredentialsRefName",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddCredentialsBindingCredentialsRefName(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&securityv1alpha1.CredentialsBinding{}))
			Expect(indexer.field).To(Equal("spec.credentialsRef.name"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no CredentialsBinding", &corev1.Secret{}, ConsistOf("")),
		Entry("CredentialsBinding w/ credentialsRef", &securityv1alpha1.CredentialsBinding{CredentialsRef: corev1.ObjectReference{Name: "cred", Namespace: "ns"}}, ConsistOf("cred")),
	)

	DescribeTable("#AddCredentialsBindingCredentialsRefNamespace",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddCredentialsBindingCredentialsRefNamespace(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&securityv1alpha1.CredentialsBinding{}))
			Expect(indexer.field).To(Equal("spec.credentialsRef.namespace"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no CredentialsBinding", &corev1.Secret{}, ConsistOf("")),
		Entry("CredentialsBinding w/ credentialsRef", &securityv1alpha1.CredentialsBinding{CredentialsRef: corev1.ObjectReference{Name: "cred", Namespace: "ns"}}, ConsistOf("ns")),
	)

	DescribeTable("#AddCredentialsBindingCredentialsRefKind",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddCredentialsBindingCredentialsRefKind(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&securityv1alpha1.CredentialsBinding{}))
			Expect(indexer.field).To(Equal("spec.credentialsRef.kind"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no CredentialsBinding", &corev1.Secret{}, ConsistOf("")),
		Entry("CredentialsBinding w/ credentialsRef", &securityv1alpha1.CredentialsBinding{CredentialsRef: corev1.ObjectReference{Kind: "Secret", Name: "cred", Namespace: "ns"}}, ConsistOf("Secret")),
	)
})
