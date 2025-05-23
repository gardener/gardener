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
	"k8s.io/apimachinery/pkg/runtime"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockcontrolplane "github.com/gardener/gardener/pkg/component/extensions/controlplane/mock"
	mockdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord/mock"
	mockinfrastructure "github.com/gardener/gardener/pkg/component/extensions/infrastructure/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("controlplane", func() {
	var (
		ctrl *gomock.Controller

		infrastructure       *mockinfrastructure.MockInterface
		controlPlane         *mockcontrolplane.MockInterface
		controlPlaneExposure *mockcontrolplane.MockInterface
		externalDNSRecord    *mockdnsrecord.MockInterface
		internalDNSRecord    *mockdnsrecord.MockInterface
		botanist             *Botanist

		ctx     = context.TODO()
		fakeErr = errors.New("fake err")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		infrastructure = mockinfrastructure.NewMockInterface(ctrl)
		controlPlane = mockcontrolplane.NewMockInterface(ctrl)
		controlPlaneExposure = mockcontrolplane.NewMockInterface(ctrl)
		externalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)
		internalDNSRecord = mockdnsrecord.NewMockInterface(ctrl)

		botanist = &Botanist{
			Operation: &operation.Operation{
				Shoot: &shoot.Shoot{
					Components: &shoot.Components{
						Extensions: &shoot.Extensions{
							ControlPlane:         controlPlane,
							ControlPlaneExposure: controlPlaneExposure,
							ExternalDNSRecord:    externalDNSRecord,
							InternalDNSRecord:    internalDNSRecord,
							Infrastructure:       infrastructure,
						},
					},
				},
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployControlPlane", func() {
		var infrastructureStatus = &runtime.RawExtension{Raw: []byte("infra-status")}

		BeforeEach(func() {
			infrastructure.EXPECT().ProviderStatus().Return(infrastructureStatus)
			controlPlane.EXPECT().SetInfrastructureProviderStatus(infrastructureStatus)
		})

		Context("deploy", func() {
			It("should deploy successfully", func() {
				controlPlane.EXPECT().Deploy(ctx)
				Expect(botanist.DeployControlPlane(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				controlPlane.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployControlPlane(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			var shootState = &gardencorev1beta1.ShootState{}

			BeforeEach(func() {
				botanist.Shoot.SetShootState(shootState)
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				})
			})

			It("should restore successfully", func() {
				controlPlane.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployControlPlane(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				controlPlane.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployControlPlane(ctx)).To(MatchError(fakeErr))
			})
		})
	})

	Describe("#DeployControlPlaneExposure()", func() {
		Context("deploy", func() {
			It("should deploy successfully", func() {
				controlPlaneExposure.EXPECT().Deploy(ctx)
				Expect(botanist.DeployControlPlaneExposure(ctx)).To(Succeed())
			})

			It("should return the error during deployment", func() {
				controlPlaneExposure.EXPECT().Deploy(ctx).Return(fakeErr)
				Expect(botanist.DeployControlPlaneExposure(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("restore", func() {
			var shootState = &gardencorev1beta1.ShootState{}

			BeforeEach(func() {
				botanist.Shoot.SetShootState(shootState)
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				})
			})

			It("should restore successfully", func() {
				controlPlaneExposure.EXPECT().Restore(ctx, shootState)
				Expect(botanist.DeployControlPlaneExposure(ctx)).To(Succeed())
			})

			It("should return the error during restoration", func() {
				controlPlaneExposure.EXPECT().Restore(ctx, shootState).Return(fakeErr)
				Expect(botanist.DeployControlPlaneExposure(ctx)).To(MatchError(fakeErr))
			})
		})
	})
})
