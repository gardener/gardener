// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
)

var _ = Describe("Networkpolicies", func() {
	var (
		ctrl            *gomock.Controller
		clientInterface *kubernetesmock.MockInterface
		c               *mockclient.MockClient
		botanist        *Botanist

		seedNamespace = "shoot--foo--bar"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		clientInterface = kubernetesmock.NewMockInterface(ctrl)
		c = mockclient.NewMockClient(ctrl)
		botanist = &Botanist{
			Operation: &operation.Operation{
				SeedClientSet: clientInterface,
				Seed:          &seedpkg.Seed{},
				Shoot: &shootpkg.Shoot{
					SeedNamespace: seedNamespace,
				},
			},
		}
	})

	DescribeTable("#DefaultNetworkPolicies",
		func(sniPhase component.Phase, prepTestValues func(), expectations func(client.Client, string)) {
			prepTestValues()
			validator := &newNetworkPoliciesFuncValidator{expectations: expectations}

			oldNewNetworkPoliciesDeployerFn := NewNetworkPoliciesDeployer
			defer func() { NewNetworkPoliciesDeployer = oldNewNetworkPoliciesDeployerFn }()
			NewNetworkPoliciesDeployer = validator.new

			clientInterface.EXPECT().Client().Return(c)

			botanist.DefaultNetworkPolicies()
		},

		Entry(
			"w/o networks",
			component.PhaseUnknown,
			func() {},
			func(client client.Client, namespace string) {
				Expect(client).To(Equal(c))
				Expect(namespace).To(Equal(seedNamespace))
			},
		),
	)
})

type newNetworkPoliciesFuncValidator struct {
	expectations func(client.Client, string)
}

func (n *newNetworkPoliciesFuncValidator) new(client client.Client, namespace string) component.Deployer {
	n.expectations(client, namespace)
	return nil
}
