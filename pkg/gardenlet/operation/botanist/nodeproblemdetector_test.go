// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("NodeProblemDetector", func() {
	var (
		botanist *Botanist
	)

	BeforeEach(func() {
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.35.0",
				},
			},
		})
		botanist.Garden = &garden.Garden{}
	})

	Describe("#DefaultNodeProblemDetector", func() {
		BeforeEach(func() {
			fakeClient := fakeclient.NewClientBuilder().Build()
			botanist.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()
		})

		It("should successfully create a nodeproblemdetector interface", func() {
			nodeProblemDetector, err := botanist.DefaultNodeProblemDetector()
			Expect(nodeProblemDetector).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
