// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
)

var _ = Describe("Operations", func() {
	var indexer *fakeFieldIndexer

	BeforeEach(func() {
		indexer = &fakeFieldIndexer{}
	})

	DescribeTable("#AddBastionShootName",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddBastionShootName(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&operationsv1alpha1.Bastion{}))
			Expect(indexer.field).To(Equal("spec.shootRef.name"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no Bastion", &corev1.Secret{}, ConsistOf("")),
		Entry("Bastion w/o shootRef", &operationsv1alpha1.Bastion{}, ConsistOf("")),
		Entry("Bastion w/ shootRef", &operationsv1alpha1.Bastion{Spec: operationsv1alpha1.BastionSpec{ShootRef: corev1.LocalObjectReference{Name: "shoot"}}}, ConsistOf("shoot")),
	)
})
