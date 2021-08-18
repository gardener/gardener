// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubescheduler"
	mockkubescheduler "github.com/gardener/gardener/pkg/operation/botanist/component/kubescheduler/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("KubeScheduler", func() {
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

	Describe("#DefaultKubeScheduler", func() {
		var kubernetesClient *mockkubernetes.MockInterface

		BeforeEach(func() {
			kubernetesClient = mockkubernetes.NewMockInterface(ctrl)
			kubernetesClient.EXPECT().Version()

			botanist.K8sSeedClient = kubernetesClient
			botanist.Shoot = &shootpkg.Shoot{}
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
		})

		It("should successfully create a kube-scheduler interface", func() {
			kubernetesClient.EXPECT().Client()
			botanist.ImageVector = imagevector.ImageVector{{Name: "kube-scheduler"}}

			kubeScheduler, err := botanist.DefaultKubeScheduler()
			Expect(kubeScheduler).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because the image cannot be found", func() {
			botanist.ImageVector = imagevector.ImageVector{}

			kubeScheduler, err := botanist.DefaultKubeScheduler()
			Expect(kubeScheduler).To(BeNil())
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#DeployKubeScheduler", func() {
		var (
			kubeScheduler *mockkubescheduler.MockInterface

			ctx              = context.TODO()
			fakeErr          = fmt.Errorf("fake err")
			secretName       = "kube-scheduler"
			secretNameServer = "kube-scheduler-server"
			checksum         = "1234"
			checksumServer   = "5678"
		)

		BeforeEach(func() {
			kubeScheduler = mockkubescheduler.NewMockInterface(ctrl)

			botanist.StoreCheckSum(secretName, checksum)
			botanist.StoreCheckSum(secretNameServer, checksumServer)
			botanist.Shoot = &shootpkg.Shoot{
				Components: &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						KubeScheduler: kubeScheduler,
					},
				},
			}

			kubeScheduler.EXPECT().SetSecrets(kubescheduler.Secrets{
				Kubeconfig: component.Secret{Name: secretName, Checksum: checksum},
				Server:     component.Secret{Name: secretNameServer, Checksum: checksumServer},
			})
		})

		It("should set the secrets and deploy", func() {
			kubeScheduler.EXPECT().Deploy(ctx)
			Expect(botanist.DeployKubeScheduler(ctx)).To(Succeed())
		})

		It("should fail when the deploy function fails", func() {
			kubeScheduler.EXPECT().Deploy(ctx).Return(fakeErr)
			Expect(botanist.DeployKubeScheduler(ctx)).To(Equal(fakeErr))
		})
	})
})
