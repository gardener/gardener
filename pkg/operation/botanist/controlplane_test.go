// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	mockcontrolplane "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/controlplane/mock"
	mockdnsrecord "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/dnsrecord/mock"
	mockinfrastructure "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/infrastructure/mock"
	"github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
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
		fakeErr = fmt.Errorf("fake err")
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

	DescribeTable("#getResourcesForAPIServer",
		func(nodes int, storageClass, expectedCPURequest, expectedMemoryRequest, expectedCPULimit, expectedMemoryLimit string) {
			cpuRequest, memoryRequest, cpuLimit, memoryLimit := getResourcesForAPIServer(int32(nodes), storageClass)

			Expect(cpuRequest).To(Equal(expectedCPURequest))
			Expect(memoryRequest).To(Equal(expectedMemoryRequest))
			Expect(cpuLimit).To(Equal(expectedCPULimit))
			Expect(memoryLimit).To(Equal(expectedMemoryLimit))
		},

		// nodes tests
		Entry("nodes <= 2", 2, "", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("nodes <= 10", 10, "", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("nodes <= 50", 50, "", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("nodes <= 100", 100, "", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("nodes > 100", 1000, "", "3000m", "5200Mi", "4000m", "7800Mi"),

		// scaling class tests
		Entry("scaling class small", -1, "small", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("scaling class medium", -1, "medium", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("scaling class large", -1, "large", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("scaling class xlarge", -1, "xlarge", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("scaling class 2xlarge", -1, "2xlarge", "3000m", "5200Mi", "4000m", "7800Mi"),

		// scaling class always decides if provided
		Entry("nodes > 100, scaling class small", 100, "small", "800m", "800Mi", "1000m", "1200Mi"),
		Entry("nodes <= 100, scaling class medium", 100, "medium", "1000m", "1100Mi", "1200m", "1900Mi"),
		Entry("nodes <= 50, scaling class large", 50, "large", "1200m", "1600Mi", "1500m", "3900Mi"),
		Entry("nodes <= 10, scaling class xlarge", 10, "xlarge", "2500m", "5200Mi", "3000m", "5900Mi"),
		Entry("nodes <= 2, scaling class 2xlarge", 2, "2xlarge", "3000m", "5200Mi", "4000m", "7800Mi"),
	)

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
			var shootState = &gardencorev1alpha1.ShootState{}

			BeforeEach(func() {
				botanist.SetShootState(shootState)
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
			var shootState = &gardencorev1alpha1.ShootState{}

			BeforeEach(func() {
				botanist.SetShootState(shootState)
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
