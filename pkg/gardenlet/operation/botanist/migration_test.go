// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	mocketcd "github.com/gardener/gardener/pkg/component/etcd/etcd/mock"
	mockcontainerruntime "github.com/gardener/gardener/pkg/component/extensions/containerruntime/mock"
	mockcontrolplane "github.com/gardener/gardener/pkg/component/extensions/controlplane/mock"
	mockinfrastructure "github.com/gardener/gardener/pkg/component/extensions/infrastructure/mock"
	mocknetwork "github.com/gardener/gardener/pkg/component/extensions/network/mock"
	mockoperatingsystemconfig "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/mock"
	mockworker "github.com/gardener/gardener/pkg/component/extensions/worker/mock"
	mockbackupentry "github.com/gardener/gardener/pkg/component/garden/backupentry/mock"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("migration", func() {
	var (
		ctrl *gomock.Controller

		containerRuntime      *mockcontainerruntime.MockInterface
		controlPlane          *mockcontrolplane.MockInterface
		controlPlaneExposure  *mockcontrolplane.MockInterface
		infrastructure        *mockinfrastructure.MockInterface
		network               *mocknetwork.MockInterface
		operatingSystemConfig *mockoperatingsystemconfig.MockInterface
		worker                *mockworker.MockInterface

		botanist *Botanist

		ctx                     = context.TODO()
		fakeErr                 = errors.New("fake")
		fakeClient              client.Client
		fakeKubernetesInterface kubernetes.Interface
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		containerRuntime = mockcontainerruntime.NewMockInterface(ctrl)
		controlPlane = mockcontrolplane.NewMockInterface(ctrl)
		controlPlaneExposure = mockcontrolplane.NewMockInterface(ctrl)
		infrastructure = mockinfrastructure.NewMockInterface(ctrl)
		network = mocknetwork.NewMockInterface(ctrl)
		operatingSystemConfig = mockoperatingsystemconfig.NewMockInterface(ctrl)
		worker = mockworker.NewMockInterface(ctrl)

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeKubernetesInterface = fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		botanist = &Botanist{Operation: &operation.Operation{
			SeedClientSet: fakeKubernetesInterface,
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

			It("should return error if retrieval of etcd main resource fails", func() {
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
				backupEntry.EXPECT().Get(ctx).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "backupentry"))
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).To(BeNotFoundError())
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
				sourceBackupEntry.EXPECT().Get(ctx).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "source-backupentry"))
				copyRequired, err := botanist.IsCopyOfBackupsRequired(ctx)
				Expect(err).To(BeNotFoundError())
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

	Describe("#ShallowDeleteMachineResources", func() {
		It("should delete most of the resources and remove MCM finalizers", func() {
			var (
				machine             = &machinev1alpha1.Machine{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: shootNamespace, Finalizers: []string{"machine.sapcloud.io/machine-controller-manager", "machine.sapcloud.io/machine-controller"}}}
				machineSet          = &machinev1alpha1.MachineSet{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: shootNamespace, Finalizers: []string{"machine.sapcloud.io/machine-controller-manager", "machine.sapcloud.io/machine-controller"}}}
				machineDeployment   = &machinev1alpha1.MachineDeployment{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: shootNamespace, Finalizers: []string{"machine.sapcloud.io/machine-controller-manager", "machine.sapcloud.io/machine-controller"}}}
				machineClass        = &machinev1alpha1.MachineClass{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: shootNamespace, Finalizers: []string{"machine.sapcloud.io/machine-controller-manager", "machine.sapcloud.io/machine-controller"}}}
				machineClassSecret  = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: shootNamespace, Finalizers: []string{"machine.sapcloud.io/machine-controller-manager", "machine.sapcloud.io/machine-controller"}, Labels: map[string]string{"gardener.cloud/purpose": "machineclass"}}}
				cloudProviderSecret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: shootNamespace, Finalizers: []string{"machine.sapcloud.io/machine-controller-manager", "machine.sapcloud.io/machine-controller", "do-not-remove-me"}, Labels: map[string]string{"gardener.cloud/purpose": "cloudprovider"}}}
				unrelatedSecret     = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{GenerateName: "obj-", Namespace: shootNamespace}}
			)

			Expect(fakeClient.Create(ctx, machine)).To(Succeed())
			Expect(fakeClient.Create(ctx, machineSet)).To(Succeed())
			Expect(fakeClient.Create(ctx, machineDeployment)).To(Succeed())
			Expect(fakeClient.Create(ctx, machineClass)).To(Succeed())
			Expect(fakeClient.Create(ctx, machineClassSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, cloudProviderSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, unrelatedSecret)).To(Succeed())

			Expect(botanist.ShallowDeleteMachineResources(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(machineSet), machineSet)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(machineDeployment), machineDeployment)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(machineClass), machineClass)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(machineClassSecret), machineClassSecret)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(cloudProviderSecret), cloudProviderSecret)).To(Succeed())
			Expect(cloudProviderSecret.DeletionTimestamp).To(BeNil())
			Expect(cloudProviderSecret.Finalizers).To(ConsistOf("do-not-remove-me"))
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(unrelatedSecret), unrelatedSecret)).To(Succeed())
		})
	})
})
