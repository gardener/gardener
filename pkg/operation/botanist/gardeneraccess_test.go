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
	mockgardeneraccess "github.com/gardener/gardener/pkg/operation/botanist/component/gardeneraccess/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("GardenerAccess", func() {
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

	Describe("#DeployGardenerAccess", func() {
		var (
			gardenerAccess *mockgardeneraccess.MockInterface

			ctx      = context.TODO()
			fakeErr  = fmt.Errorf("fake err")
			caCert   = []byte("cert")
			caSecret = &corev1.Secret{Data: map[string][]byte{"ca.crt": caCert}}
		)

		BeforeEach(func() {
			gardenerAccess = mockgardeneraccess.NewMockInterface(ctrl)

			botanist.StoreSecret("ca", caSecret)

			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					GardenerAccess: gardenerAccess,
				},
			}

			gardenerAccess.EXPECT().SetCACertificate(caCert)
		})

		It("should set the secrets and deploy", func() {
			gardenerAccess.EXPECT().Deploy(ctx)
			Expect(botanist.DeployGardenerAccess(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			gardenerAccess.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployGardenerAccess(ctx)).To(MatchError(fakeErr))
		})
	})
})
