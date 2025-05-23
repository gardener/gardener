// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockcontainerruntime "github.com/gardener/gardener/pkg/component/extensions/containerruntime/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("ContainerRuntime", func() {
	var (
		ctrl             *gomock.Controller
		containerRuntime *mockcontainerruntime.MockInterface
		botanist         *Botanist

		ctx        = context.TODO()
		fakeErr    = errors.New("fake")
		shootState = &gardencorev1beta1.ShootState{}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		containerRuntime = mockcontainerruntime.NewMockInterface(ctrl)
		botanist = &Botanist{Operation: &operation.Operation{
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						ContainerRuntime: containerRuntime,
					},
				},
			},
		}}
		botanist.Shoot.SetShootState(shootState)
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployContainerRuntime", func() {
		Context("deploy", func() {
			It("should deploy successfully", func() {
				containerRuntime.EXPECT().Deploy(ctx)
				Expect(botanist.DeployContainerRuntime(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				containerRuntime.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployContainerRuntime(ctx)).To(MatchError(fakeErr))
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
				containerRuntime.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployContainerRuntime(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				containerRuntime.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployContainerRuntime(ctx)).To(MatchError(fakeErr))
			})
		})
	})
})
