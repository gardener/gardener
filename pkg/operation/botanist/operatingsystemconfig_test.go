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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/operation/botanist/extensions/operatingsystemconfig/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("operatingsystemconfig", func() {
	var (
		ctrl                  *gomock.Controller
		operatingSystemConfig *mockoperatingsystemconfig.MockInterface
		botanist              *Botanist

		ctx        = context.TODO()
		fakeErr    = fmt.Errorf("fake")
		shootState = &gardencorev1alpha1.ShootState{}

		ca             = []byte("ca")
		caKubelet      = []byte("ca-kubelet")
		caCloudProfile = "ca-cloud-profile"
		sshPublicKey   = []byte("ssh-public-key")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		operatingSystemConfig = mockoperatingsystemconfig.NewMockInterface(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{
			Secrets: map[string]*corev1.Secret{
				"ca":          {Data: map[string][]byte{"ca.crt": ca}},
				"ca-kubelet":  {Data: map[string][]byte{"ca.crt": caKubelet}},
				"ssh-keypair": {Data: map[string][]byte{"id_rsa.pub": sshPublicKey}},
			},
			Shoot: &shootpkg.Shoot{
				CloudProfile: &gardencorev1beta1.CloudProfile{},
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						OperatingSystemConfig: operatingSystemConfig,
					},
				},
			},
			ShootState: shootState,
		}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployOperatingSystemConfig", func() {
		BeforeEach(func() {
			operatingSystemConfig.EXPECT().SetKubeletCACertificate(string(caKubelet))
			operatingSystemConfig.EXPECT().SetSSHPublicKey(string(sshPublicKey))
		})

		Context("deploy", func() {
			It("should deploy successfully (no CA)", func() {
				botanist.Secrets["ca"].Data["ca.crt"] = nil
				operatingSystemConfig.EXPECT().SetCABundle(nil)

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should deploy successfully (only cluster CA)", func() {
				operatingSystemConfig.EXPECT().SetCABundle(pointer.StringPtr("\n" + string(ca)))

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should deploy successfully (only CloudProfile CA)", func() {
				botanist.Shoot.CloudProfile.Spec.CABundle = &caCloudProfile
				botanist.Secrets["ca"].Data["ca.crt"] = nil
				operatingSystemConfig.EXPECT().SetCABundle(&caCloudProfile)

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should deploy successfully (both cluster and CloudProfile CA)", func() {
				botanist.Shoot.CloudProfile.Spec.CABundle = &caCloudProfile
				operatingSystemConfig.EXPECT().SetCABundle(pointer.StringPtr(caCloudProfile + "\n" + string(ca)))

				operatingSystemConfig.EXPECT().Deploy(ctx)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				operatingSystemConfig.EXPECT().SetCABundle(pointer.StringPtr("\n" + string(ca)))

				operatingSystemConfig.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(MatchError(fakeErr))
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

				operatingSystemConfig.EXPECT().SetCABundle(pointer.StringPtr("\n" + string(ca)))
			})

			It("should restore successfully", func() {
				operatingSystemConfig.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				operatingSystemConfig.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployOperatingSystemConfig(ctx)).To(MatchError(fakeErr))
			})
		})
	})
})
