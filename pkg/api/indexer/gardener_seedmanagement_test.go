// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
