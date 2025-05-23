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
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

var _ = Describe("SeedManagement", func() {
	var indexer *fakeFieldIndexer

	BeforeEach(func() {
		indexer = &fakeFieldIndexer{}
	})

	DescribeTable("#AddManagedSeedShootName",
		func(obj client.Object, matcher gomegatypes.GomegaMatcher) {
			Expect(AddManagedSeedShootName(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.obj).To(Equal(&seedmanagementv1alpha1.ManagedSeed{}))
			Expect(indexer.field).To(Equal("spec.shoot.name"))
			Expect(indexer.extractValue).NotTo(BeNil())
			Expect(indexer.extractValue(obj)).To(matcher)
		},

		Entry("no ManagedSeed", &corev1.Secret{}, ConsistOf("")),
		Entry("ManagedSeed w/o shoot", &seedmanagementv1alpha1.ManagedSeed{}, ConsistOf("")),
		Entry("ManagedSeed w/ shoot", &seedmanagementv1alpha1.ManagedSeed{Spec: seedmanagementv1alpha1.ManagedSeedSpec{Shoot: &seedmanagementv1alpha1.Shoot{Name: "shoot"}}}, ConsistOf("shoot")),
	)
})
