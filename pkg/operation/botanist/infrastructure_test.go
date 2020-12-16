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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	mockshoot "github.com/gardener/gardener/pkg/mock/gardener/operation/shoot"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

var _ = Describe("Infrastructure", func() {
	var (
		ctrl           *gomock.Controller
		infrastructure *mockshoot.MockExtensionInfrastructure
		botanist       *Botanist

		ctx          = context.TODO()
		fakeErr      = fmt.Errorf("fake")
		shootState   = &gardencorev1alpha1.ShootState{}
		sshPublicKey = []byte("key")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		infrastructure = mockshoot.NewMockExtensionInfrastructure(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{
			Secrets: map[string]*corev1.Secret{
				"ssh-keypair": {Data: map[string][]byte{"id_rsa.pub": sshPublicKey}},
			},
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						Infrastructure: infrastructure,
					},
				},
			},
			ShootState: shootState,
		}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployInfrastructure", func() {
		BeforeEach(func() {
			infrastructure.EXPECT().SetSSHPublicKey(sshPublicKey)
		})

		Context("deploy", func() {
			It("should deploy successfully", func() {
				infrastructure.EXPECT().Deploy(ctx)
				Expect(botanist.DeployInfrastructure(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				infrastructure.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployInfrastructure(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			BeforeEach(func() {
				botanist.Shoot.Info = &gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				}
			})

			It("should restore successfully", func() {
				infrastructure.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployInfrastructure(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				infrastructure.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployInfrastructure(ctx)).To(MatchError(fakeErr))
			})
		})
	})

	Describe("#WaitForInfrastructure", func() {
		var (
			kubernetesGardenInterface *mockkubernetes.MockInterface
			kubernetesGardenClient    *mockclient.MockClient
			kubernetesSeedInterface   *mockkubernetes.MockInterface
			kubernetesSeedClient      *mockclient.MockClient

			namespace      = "namespace"
			name           = "name"
			providerStatus = &runtime.RawExtension{Raw: []byte(`{"some": "status"}"`)}
			nodesCIDR      = pointer.StringPtr("1.2.3.4/5")
			shoot          = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		)

		BeforeEach(func() {
			kubernetesGardenInterface = mockkubernetes.NewMockInterface(ctrl)
			kubernetesGardenClient = mockclient.NewMockClient(ctrl)
			kubernetesSeedInterface = mockkubernetes.NewMockInterface(ctrl)
			kubernetesSeedClient = mockclient.NewMockClient(ctrl)

			botanist.K8sGardenClient = kubernetesGardenInterface
			botanist.K8sSeedClient = kubernetesSeedInterface
			botanist.Shoot.Info = shoot
		})

		It("should successfully wait (w/ provider status, w/ nodes cidr)", func() {
			infrastructure.EXPECT().Wait(ctx)
			infrastructure.EXPECT().ProviderStatus().Return(providerStatus)
			infrastructure.EXPECT().NodesCIDR().Return(nodesCIDR)

			kubernetesGardenInterface.EXPECT().DirectClient().Return(kubernetesGardenClient)
			updatedShoot := shoot.DeepCopy()
			updatedShoot.Spec.Networking.Nodes = nodesCIDR
			kubernetesGardenClient.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}))
			kubernetesGardenClient.EXPECT().Update(ctx, updatedShoot)

			kubernetesSeedInterface.EXPECT().Client().Return(kubernetesSeedClient)

			Expect(botanist.WaitForInfrastructure(ctx)).To(Succeed())
			Expect(botanist.Shoot.InfrastructureStatus).To(Equal(providerStatus.Raw))
			Expect(botanist.Shoot.Info).To(Equal(updatedShoot))
		})

		It("should successfully wait (w/o provider status, w/o nodes cidr)", func() {
			infrastructure.EXPECT().Wait(ctx)
			infrastructure.EXPECT().ProviderStatus()
			infrastructure.EXPECT().NodesCIDR()

			Expect(botanist.WaitForInfrastructure(ctx)).To(Succeed())
			Expect(botanist.Shoot.InfrastructureStatus).To(BeNil())
			Expect(botanist.Shoot.Info).To(Equal(shoot))
		})

		It("should return the error during wait", func() {
			infrastructure.EXPECT().Wait(ctx).Return(fakeErr)

			Expect(botanist.WaitForInfrastructure(ctx)).To(MatchError(fakeErr))
			Expect(botanist.Shoot.InfrastructureStatus).To(BeNil())
			Expect(botanist.Shoot.Info).To(Equal(shoot))
		})

		It("should return the error during nodes cidr update", func() {
			infrastructure.EXPECT().Wait(ctx)
			infrastructure.EXPECT().ProviderStatus()
			infrastructure.EXPECT().NodesCIDR().Return(nodesCIDR)

			kubernetesGardenInterface.EXPECT().DirectClient().Return(kubernetesGardenClient)
			updatedShoot := shoot.DeepCopy()
			updatedShoot.Spec.Networking.Nodes = nodesCIDR
			kubernetesGardenClient.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}))
			kubernetesGardenClient.EXPECT().Update(ctx, updatedShoot).Return(fakeErr)

			Expect(botanist.WaitForInfrastructure(ctx)).To(MatchError(fakeErr))
			Expect(botanist.Shoot.InfrastructureStatus).To(BeNil())
			Expect(botanist.Shoot.Info).To(Equal(shoot))
		})
	})
})
