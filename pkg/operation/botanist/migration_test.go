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
	mockcontainerruntime "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/containerruntime/mock"
	mockcontrolplane "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/controlplane/mock"
	mockextension "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/extension/mock"
	mockinfrastructure "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/infrastructure/mock"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/mock"
	mockworker "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/worker/mock"
	mockcomponent "github.com/gardener/gardener/pkg/operation/botanist/component/mock"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("migration", func() {
	var (
		ctrl *gomock.Controller

		containerRuntime      *mockcontainerruntime.MockInterface
		controlPlane          *mockcontrolplane.MockInterface
		controlPlaneExposure  *mockcontrolplane.MockInterface
		extension             *mockextension.MockInterface
		infrastructure        *mockinfrastructure.MockInterface
		network               *mockcomponent.MockDeployMigrateWaiter
		operatingSystemConfig *mockoperatingsystemconfig.MockInterface
		worker                *mockworker.MockInterface

		botanist *Botanist

		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		containerRuntime = mockcontainerruntime.NewMockInterface(ctrl)
		controlPlane = mockcontrolplane.NewMockInterface(ctrl)
		controlPlaneExposure = mockcontrolplane.NewMockInterface(ctrl)
		extension = mockextension.NewMockInterface(ctrl)
		infrastructure = mockinfrastructure.NewMockInterface(ctrl)
		network = mockcomponent.NewMockDeployMigrateWaiter(ctrl)
		operatingSystemConfig = mockoperatingsystemconfig.NewMockInterface(ctrl)
		worker = mockworker.NewMockInterface(ctrl)

		botanist = &Botanist{Operation: &operation.Operation{
			Shoot: &shootpkg.Shoot{
				Components: &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						ContainerRuntime:      containerRuntime,
						ControlPlane:          controlPlane,
						ControlPlaneExposure:  controlPlaneExposure,
						Extension:             extension,
						Infrastructure:        infrastructure,
						Network:               network,
						OperatingSystemConfig: operatingSystemConfig,
						Worker:                worker,
					},
				},
			},
		}}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#MigrateAllExtensionResources", func() {
		It("should call the Migrate() func of all extension components", func() {
			containerRuntime.EXPECT().Migrate(ctx)
			controlPlane.EXPECT().Migrate(ctx)
			controlPlaneExposure.EXPECT().Migrate(ctx)
			extension.EXPECT().Migrate(ctx)
			infrastructure.EXPECT().Migrate(ctx)
			network.EXPECT().Migrate(ctx)
			operatingSystemConfig.EXPECT().Migrate(ctx)
			worker.EXPECT().Migrate(ctx)

			Expect(botanist.MigrateAllExtensionResources(ctx)).To(Succeed())
		})

		It("should return an error if not all the Migrate() func of all extension components succeed", func() {
			containerRuntime.EXPECT().Migrate(ctx)
			controlPlane.EXPECT().Migrate(ctx).Return(fakeErr)
			controlPlaneExposure.EXPECT().Migrate(ctx)
			extension.EXPECT().Migrate(ctx)
			infrastructure.EXPECT().Migrate(ctx)
			network.EXPECT().Migrate(ctx)
			operatingSystemConfig.EXPECT().Migrate(ctx)
			worker.EXPECT().Migrate(ctx)

			err := botanist.MigrateAllExtensionResources(ctx)
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
		})
	})

	Describe("#WaitUntilAllExtensionResourcesMigrated", func() {
		It("should call the Migrate() func of all extension components", func() {
			containerRuntime.EXPECT().WaitMigrate(ctx)
			controlPlane.EXPECT().WaitMigrate(ctx)
			controlPlaneExposure.EXPECT().WaitMigrate(ctx)
			extension.EXPECT().WaitMigrate(ctx)
			infrastructure.EXPECT().WaitMigrate(ctx)
			network.EXPECT().WaitMigrate(ctx)
			operatingSystemConfig.EXPECT().WaitMigrate(ctx)
			worker.EXPECT().WaitMigrate(ctx)

			Expect(botanist.WaitUntilAllExtensionResourcesMigrated(ctx)).To(Succeed())
		})

		It("should return an error if not all the WaitMigrate() func of all extension components succeed", func() {
			containerRuntime.EXPECT().WaitMigrate(ctx)
			controlPlane.EXPECT().WaitMigrate(ctx)
			controlPlaneExposure.EXPECT().WaitMigrate(ctx)
			extension.EXPECT().WaitMigrate(ctx)
			infrastructure.EXPECT().WaitMigrate(ctx)
			network.EXPECT().WaitMigrate(ctx).Return(fakeErr)
			operatingSystemConfig.EXPECT().WaitMigrate(ctx)
			worker.EXPECT().WaitMigrate(ctx).Return(fakeErr)

			err := botanist.WaitUntilAllExtensionResourcesMigrated(ctx)
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr), Equal(fakeErr)))
		})
	})

	Describe("#DestroyAllExtensionResources", func() {
		It("should call the Destroy() func of all extension components", func() {
			containerRuntime.EXPECT().Destroy(ctx)
			controlPlane.EXPECT().Destroy(ctx)
			controlPlaneExposure.EXPECT().Destroy(ctx)
			extension.EXPECT().Destroy(ctx)
			infrastructure.EXPECT().Destroy(ctx)
			network.EXPECT().Destroy(ctx)
			operatingSystemConfig.EXPECT().Destroy(ctx)
			worker.EXPECT().Destroy(ctx)

			Expect(botanist.DestroyAllExtensionResources(ctx)).To(Succeed())
		})

		It("should return an error if not all the Destroy() func of all extension components succeed", func() {
			containerRuntime.EXPECT().Destroy(ctx).Return(fakeErr)
			controlPlane.EXPECT().Destroy(ctx)
			controlPlaneExposure.EXPECT().Destroy(ctx).Return(fakeErr)
			extension.EXPECT().Destroy(ctx)
			infrastructure.EXPECT().Destroy(ctx)
			network.EXPECT().Destroy(ctx)
			operatingSystemConfig.EXPECT().Destroy(ctx)
			worker.EXPECT().Destroy(ctx)

			err := botanist.DestroyAllExtensionResources(ctx)
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr), Equal(fakeErr)))
		})
	})
})
