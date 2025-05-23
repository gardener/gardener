// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mocknetwork "github.com/gardener/gardener/pkg/component/extensions/network/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Network", func() {
	var (
		ctrl     *gomock.Controller
		network  *mocknetwork.MockInterface
		botanist *Botanist

		ctx        = context.TODO()
		fakeErr    = errors.New("fake")
		shootState = &gardencorev1beta1.ShootState{}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		network = mocknetwork.NewMockInterface(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						Network: network,
					},
				},
				Networks: &shootpkg.Networks{
					Pods:     []net.IPNet{{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}, {IP: net.ParseIP("2001:db8:1::"), Mask: net.CIDRMask(64, 128)}},
					Services: []net.IPNet{{IP: net.ParseIP("10.0.1.0"), Mask: net.CIDRMask(24, 32)}, {IP: net.ParseIP("2001:db8:2::"), Mask: net.CIDRMask(64, 128)}},
				},
			},
		}}
		botanist.Shoot.SetShootState(shootState)
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployNetwork", func() {
		BeforeEach(func() {
			network.EXPECT().SetPodCIDRs(botanist.Shoot.Networks.Pods)
			network.EXPECT().SetServiceCIDRs(botanist.Shoot.Networks.Services)
		})

		Context("deploy", func() {
			It("should deploy successfully", func() {
				network.EXPECT().Deploy(ctx)
				Expect(botanist.DeployNetwork(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				network.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployNetwork(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			BeforeEach(func() {
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				})
			})

			It("should restore successfully", func() {
				network.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployNetwork(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				network.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployNetwork(ctx)).To(MatchError(fakeErr))
			})
		})
	})
})
