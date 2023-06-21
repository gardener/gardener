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
	"context"
	"errors"
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockbackupentry "github.com/gardener/gardener/pkg/component/backupentry/mock"
	mocketcd "github.com/gardener/gardener/pkg/component/etcd/mock"
	mockcontainerruntime "github.com/gardener/gardener/pkg/component/extensions/containerruntime/mock"
	mockcontrolplane "github.com/gardener/gardener/pkg/component/extensions/controlplane/mock"
	mockinfrastructure "github.com/gardener/gardener/pkg/component/extensions/infrastructure/mock"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/mock"
	mockworker "github.com/gardener/gardener/pkg/component/extensions/worker/mock"
	mockcomponent "github.com/gardener/gardener/pkg/component/mock"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
)

var _ = Describe("migration", func() {
	var (
		ctrl *gomock.Controller

		containerRuntime      *mockcontainerruntime.MockInterface
		controlPlane          *mockcontrolplane.MockInterface
		controlPlaneExposure  *mockcontrolplane.MockInterface
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
			controlPlaneExposure.EXPECT().Migrate(ctx)
			network.EXPECT().Migrate(ctx)
			operatingSystemConfig.EXPECT().Migrate(ctx)
			worker.EXPECT().Migrate(ctx)

			Expect(botanist.MigrateExtensionResourcesInParallel(ctx)).To(Succeed())
		})

		It("should return an error if not all the Migrate() func of all extension components succeed", func() {
			containerRuntime.EXPECT().Migrate(ctx)
			controlPlaneExposure.EXPECT().Migrate(ctx)
			network.EXPECT().Migrate(ctx).Return(fakeErr)
			operatingSystemConfig.EXPECT().Migrate(ctx)
			worker.EXPECT().Migrate(ctx)

			err := botanist.MigrateExtensionResourcesInParallel(ctx)
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr)))
		})
	})

	Describe("#WaitUntilAllExtensionResourcesMigrated", func() {
		It("should call the Migrate() func of all extension components", func() {
			containerRuntime.EXPECT().WaitMigrate(ctx)
			controlPlaneExposure.EXPECT().WaitMigrate(ctx)
			network.EXPECT().WaitMigrate(ctx)
			operatingSystemConfig.EXPECT().WaitMigrate(ctx)
			worker.EXPECT().WaitMigrate(ctx)

			Expect(botanist.WaitUntilExtensionResourcesMigrated(ctx)).To(Succeed())
		})

		It("should call the Migrate() func of all the required extension components for workerless Shoot", func() {
			botanist.Shoot.IsWorkerless = true

			Expect(botanist.WaitUntilExtensionResourcesMigrated(ctx)).To(Succeed())
		})

		It("should return an error if not all the WaitMigrate() func of all extension components succeed", func() {
			containerRuntime.EXPECT().WaitMigrate(ctx)
			controlPlaneExposure.EXPECT().WaitMigrate(ctx)
			network.EXPECT().WaitMigrate(ctx).Return(fakeErr)
			operatingSystemConfig.EXPECT().WaitMigrate(ctx)
			worker.EXPECT().WaitMigrate(ctx).Return(fakeErr)

			err := botanist.WaitUntilExtensionResourcesMigrated(ctx)
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr), Equal(fakeErr)))
		})
	})

	Describe("#DestroyAllExtensionResources", func() {
		It("should call the Destroy() func of all extension components", func() {
			containerRuntime.EXPECT().Destroy(ctx)
			controlPlaneExposure.EXPECT().Destroy(ctx)
			network.EXPECT().Destroy(ctx)
			operatingSystemConfig.EXPECT().Destroy(ctx)
			worker.EXPECT().Destroy(ctx)

			Expect(botanist.DestroyExtensionResourcesInParallel(ctx)).To(Succeed())
		})

		It("should call the Destroy() func of all required extension components (workerless shoot)", func() {
			botanist.Shoot.IsWorkerless = true

			Expect(botanist.DestroyExtensionResourcesInParallel(ctx)).To(Succeed())
		})

		It("should return an error if not all the Destroy() func of all extension components succeed", func() {
			containerRuntime.EXPECT().Destroy(ctx).Return(fakeErr)
			controlPlaneExposure.EXPECT().Destroy(ctx).Return(fakeErr)
			network.EXPECT().Destroy(ctx)
			operatingSystemConfig.EXPECT().Destroy(ctx)
			worker.EXPECT().Destroy(ctx)

			err := botanist.DestroyExtensionResourcesInParallel(ctx)
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(fakeErr), Equal(fakeErr)))
		})
	})

	Describe("#IsCopyOfBackupsRequired", func() {
		var (
			etcdMain          *mocketcd.MockInterface
			backupEntry       *mockbackupentry.MockInterface
			sourceBackupEntry *mockbackupentry.MockInterface
			fakeErr           = errors.New("Fake error")
		)

		BeforeEach(func() {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: "foo",
				},
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						Type: gardencorev1beta1.LastOperationTypeRestore,
					},
				},
			})
			botanist.Seed = &seed.Seed{}
			botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "seed",
					Namespace: "garden",
					UID:       "new-seed",
				},
				Spec: gardencorev1beta1.SeedSpec{
					Backup: &gardencorev1beta1.SeedBackup{
						Provider: "gcp",
					},
				},
			})

			etcdMain = mocketcd.NewMockInterface(ctrl)
			backupEntry = mockbackupentry.NewMockInterface(ctrl)
			sourceBackupEntry = mockbackupentry.NewMockInterface(ctrl)
			botanist.Shoot.Components.ControlPlane = &shootpkg.ControlPlane{
				EtcdMain: etcdMain,
			}
			botanist.Shoot.Components.BackupEntry = backupEntry
			botanist.Shoot.Components.SourceBackupEntry = sourceBackupEntry
		})

		It("should return false if lastOperation is not restore", func() {
			botanist.Shoot.GetInfo().Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
			copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(copyRequired).To(BeFalse())
		})

		It("should return false if seed backup is not set", func() {
			botanist.Seed.GetInfo().Spec.Backup = nil
			copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(copyRequired).To(BeFalse())
		})

		It("should return false if lastOperation is nil", func() {
			botanist.Shoot.GetInfo().Status.LastOperation = nil
			copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(copyRequired).To(BeFalse())
		})

		Context("Last operation is restore and etcd main exists", func() {
			It("should return false if etcd main resource has been deployed", func() {
				etcdMain.EXPECT().Get(ctx)
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(copyRequired).To(BeFalse())
			})

			It("should return error if retrieval of etcd main resource fials", func() {
				etcdMain.EXPECT().Get(ctx).Return(nil, fakeErr)
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).To(MatchError(fakeErr))
				Expect(copyRequired).To(BeFalse())
			})

		})

		Context("Last operation is restore and etcd main does not exist", func() {
			BeforeEach(func() {
				etcdMain.EXPECT().Get(ctx).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "etcd-main"))
			})

			It("should return an error if backupentry retrieval fails", func() {
				backupEntry.EXPECT().Get(ctx).Return(nil, fakeErr)
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).To(MatchError(fakeErr))
				Expect(copyRequired).To(BeFalse())
			})

			It("should return an error if backupentry is not found", func() {
				backupEntryNotFoundErr := apierrors.NewNotFound(schema.GroupResource{}, "backupentry")
				backupEntry.EXPECT().Get(ctx).Return(nil, backupEntryNotFoundErr)
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).To(MatchError(backupEntryNotFoundErr))
				Expect(copyRequired).To(BeFalse())
			})

			It("should return true if backupentry.Spec.BucketName has not been switched to the new seed", func() {
				backupEntry.EXPECT().Get(ctx).Return(&gardencorev1beta1.BackupEntry{
					Spec: gardencorev1beta1.BackupEntrySpec{
						BucketName: "old-seed",
					},
				}, nil)
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).To(Succeed())
				Expect(copyRequired).To(BeTrue())
			})
		})

		Context("Last operation is restore, etcd-main resource exists and backupentry.Spec.BucketName is switched to the new seed", func() {
			BeforeEach(func() {
				etcdMain.EXPECT().Get(ctx).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "etcd-main"))
				backupEntry.EXPECT().Get(ctx).Return(&gardencorev1beta1.BackupEntry{
					Spec: gardencorev1beta1.BackupEntrySpec{
						BucketName: string(botanist.Seed.GetInfo().UID),
					},
				}, nil)
			})

			It("should return error if source backupentry does not exist", func() {
				sourceBackupEntryNotFoundErr := apierrors.NewNotFound(schema.GroupResource{}, "source-backupentry")
				sourceBackupEntry.EXPECT().Get(ctx).Return(nil, sourceBackupEntryNotFoundErr)
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).To(MatchError(sourceBackupEntryNotFoundErr))
				Expect(copyRequired).To(BeFalse())
			})

			It("should return an error if source backupentry retrieval fails", func() {
				sourceBackupEntry.EXPECT().Get(ctx).Return(nil, fakeErr)
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).To(MatchError(fakeErr))
				Expect(copyRequired).To(BeFalse())
			})

			It("should return an error if source backupentry and destination backupentry point to the same bucket", func() {
				sourceBackupEntry.EXPECT().Get(ctx).Return(&gardencorev1beta1.BackupEntry{
					Spec: gardencorev1beta1.BackupEntrySpec{
						BucketName: string(botanist.Seed.GetInfo().UID),
					},
				}, nil)
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).To(HaveOccurred())
				Expect(copyRequired).To(BeFalse())
			})

			It("should return true if source backupentry and destination backupentry point to different buckets", func() {
				sourceBackupEntry.EXPECT().Get(ctx).Return(&gardencorev1beta1.BackupEntry{
					Spec: gardencorev1beta1.BackupEntrySpec{
						BucketName: "old-seed",
					},
				}, nil)
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).To(Succeed())
				Expect(copyRequired).To(BeTrue())
			})
		})
	})

	Describe("#IsRestorePhase", func() {
		It("should return true", func() {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{LastOperation: &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeRestore}}})
			Expect(botanist.IsRestorePhase()).To(BeTrue())
		})

		It("should return false", func() {
			botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{LastOperation: &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}}})
			Expect(botanist.IsRestorePhase()).To(BeFalse())
		})
	})
})
