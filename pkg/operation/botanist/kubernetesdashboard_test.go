// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Kubernetes Dashboard", func() {
	var (
		seedClient client.Client
		botanist   *Botanist
	)

	BeforeEach(func() {
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Addons: &gardencorev1beta1.Addons{
					KubernetesDashboard: &gardencorev1beta1.KubernetesDashboard{},
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.22.1",
				},
			},
		})
	})

	Describe("#DefaultKubernetesDashboard", func() {
		BeforeEach(func() {
			botanist.SeedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build()

			botanist.Shoot.DisableDNS = true
		})

		It("should successfully create a Kubernetes Dashboard interface", func() {
			defer test.WithFeatureGate(gardenletfeatures.FeatureGate, features.APIServerSNI, true)()

			botanist.ImageVector = imagevector.ImageVector{
				{Name: "kubernetes-dashboard"},
				{Name: "kubernetes-dashboard-metrics-scraper"},
			}

			kubernetesDashboard, err := botanist.DefaultKubernetesDashboard()
			Expect(kubernetesDashboard).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			kubernetesDashboard, err := botanist.DefaultKubernetesDashboard()
			Expect(kubernetesDashboard).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})
})
