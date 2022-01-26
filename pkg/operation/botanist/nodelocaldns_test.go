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
	"net"

	"github.com/gardener/gardener/charts"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("NodeLocalDNS", func() {
	var (
		ctrl     *gomock.Controller
		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					v1beta1constants.AnnotationNodeLocalDNS: "true",
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultNodeLocalDNS", func() {
		var kubernetesClient *mockkubernetes.MockInterface

		BeforeEach(func() {
			kubernetesClient = mockkubernetes.NewMockInterface(ctrl)

			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot.Networks = &shootpkg.Networks{
				CoreDNS: net.ParseIP("18.19.20.21"),
			}
		})

		It("should successfully create a node-local-dns interface", func() {
			kubernetesClient.EXPECT().Client()
			botanist.ImageVector = imagevector.ImageVector{{Name: charts.ImageNameNodeLocalDns}}

			nodeLocalDNS, err := botanist.DefaultNodeLocalDNS()
			Expect(nodeLocalDNS).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			nodeLocalDNS, err := botanist.DefaultNodeLocalDNS()
			Expect(nodeLocalDNS).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})
})
