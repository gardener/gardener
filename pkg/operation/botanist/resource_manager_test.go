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

package botanist_test

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager"
	mockresourcemanager "github.com/gardener/gardener/pkg/operation/botanist/component/resourcemanager/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceManager", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployGardenerResourceManager", func() {
		var (
			resourceManager *mockresourcemanager.MockInterface

			ctx           = context.TODO()
			fakeErr       = fmt.Errorf("fake err")
			secretName    = "gardener-resource-manager"
			seedNamespace = "fake-seed-ns"
			checksum      = "1234"
		)

		BeforeEach(func() {
			resourceManager = mockresourcemanager.NewMockInterface(ctrl)

			botanist.StoreCheckSum(secretName, checksum)
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						ResourceManager: resourceManager,
					},
				},
				SeedNamespace: seedNamespace,
			}

			resourceManager.EXPECT().SetSecrets(resourcemanager.Secrets{
				Kubeconfig: component.Secret{Name: secretName, Checksum: checksum}})
		})

		It("should set the secrets and deploy", func() {
			resourceManager.EXPECT().Deploy(ctx)
			Expect(botanist.DeployGardenerResourceManager(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			resourceManager.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployGardenerResourceManager(ctx)).To(Equal(fakeErr))
		})
	})
})
