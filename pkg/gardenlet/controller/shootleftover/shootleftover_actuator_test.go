// Copyright (c) 2021 SAP SE or an SAP affiliate company.All rights reserved.This file is licensed under the Apache Software License, v.2 except as noted otherwise in the LICENSE file
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

package shootleftover_test

import (
	"context"
	"errors"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	mockclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/mock"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shootleftover"
	mockshootleftover "github.com/gardener/gardener/pkg/gardenlet/controller/shootleftover/mock"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl *gomock.Controller

		clientMap      *mockclientmap.MockClientMap
		gardenClient   *mockkubernetes.MockInterface
		seedClient     *mockkubernetes.MockInterface
		gc             *mockclient.MockClient
		gsw            *mockclient.MockStatusWriter
		sc             *mockclient.MockClient
		cleanerFactory *mockshootleftover.MockCleanerFactory
		cleaner        *mockshootleftover.MockCleaner

		actuator Actuator

		ctx context.Context

		shootLeftover *gardencorev1alpha1.ShootLeftover
		ns            *corev1.Namespace
		cluster       *extensionsv1alpha1.Cluster
		backupEntry   *extensionsv1alpha1.BackupEntry

		cleanup func()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		clientMap = mockclientmap.NewMockClientMap(ctrl)
		gardenClient = mockkubernetes.NewMockInterface(ctrl)
		seedClient = mockkubernetes.NewMockInterface(ctrl)
		gc = mockclient.NewMockClient(ctrl)
		gsw = mockclient.NewMockStatusWriter(ctrl)
		sc = mockclient.NewMockClient(ctrl)
		cleanerFactory = mockshootleftover.NewMockCleanerFactory(ctrl)
		cleaner = mockshootleftover.NewMockCleaner(ctrl)

		clientMap.EXPECT().GetClient(gomock.Any(), keys.ForSeedWithName(seedName)).Return(seedClient, nil).AnyTimes()
		gardenClient.EXPECT().Client().Return(gc).AnyTimes()
		seedClient.EXPECT().Client().Return(sc).AnyTimes()
		gc.EXPECT().Status().Return(gsw).AnyTimes()
		cleanerFactory.EXPECT().NewCleaner(sc, technicalID, uid, gomock.Any(), gomock.Any()).Return(cleaner).AnyTimes()

		actuator = NewActuator(gardenClient, clientMap, cleanerFactory, 1*time.Second, 1*time.Second, 1*time.Second)

		ctx = context.TODO()

		shootLeftover = &gardencorev1alpha1.ShootLeftover{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: gardencorev1alpha1.ShootLeftoverSpec{
				SeedName:    seedName,
				ShootName:   shootName,
				TechnicalID: pointer.String(technicalID),
				UID:         func(v types.UID) *types.UID { return &v }(uid),
			},
		}
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: technicalID,
			},
		}
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: technicalID,
			},
		}
		backupEntry = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: uid + "--" + technicalID,
			},
		}

		cleanup = test.WithVar(&gardenerlogger.Logger, gardenerlogger.NewNopLogger())
	})

	AfterEach(func() {
		cleanup()
		ctrl.Finish()
	})

	var (
		expectPatchShootLeftoverStatus = func(expect func(*gardencorev1alpha1.ShootLeftover)) *gomock.Call {
			return gsw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1alpha1.ShootLeftover{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, slo *gardencorev1alpha1.ShootLeftover, _ client.Patch, _ ...client.PatchOption) error {
					expect(slo)
					*shootLeftover = *slo
					return nil
				},
			)
		}
	)

	Describe("#Reconcile", func() {
		BeforeEach(func() {
			expectPatchShootLeftoverStatus(func(slo *gardencorev1alpha1.ShootLeftover) {
				Expect(slo.Status.LastOperation).To(Not(BeNil()))
			}).AnyTimes()
		})

		It("should return true if some leftover resources still exist", func() {
			cleaner.EXPECT().GetNamespace(gomock.Any()).Return(ns, nil)
			cleaner.EXPECT().GetCluster(gomock.Any()).Return(cluster, nil)
			cleaner.EXPECT().GetBackupEntry(gomock.Any()).Return(backupEntry, nil)
			cleaner.EXPECT().GetDNSOwners(gomock.Any()).Return(nil, nil)

			resourcesExist, err := actuator.Reconcile(ctx, shootLeftover)
			Expect(err).ToNot(HaveOccurred())
			Expect(resourcesExist).To(BeTrue())
		})

		It("should return false if leftover resources no longer exist", func() {
			cleaner.EXPECT().GetNamespace(gomock.Any()).Return(nil, nil)
			cleaner.EXPECT().GetCluster(gomock.Any()).Return(nil, nil)
			cleaner.EXPECT().GetBackupEntry(gomock.Any()).Return(nil, nil)
			cleaner.EXPECT().GetDNSOwners(gomock.Any()).Return(nil, nil)

			resourcesExist, err := actuator.Reconcile(ctx, shootLeftover)
			Expect(err).ToNot(HaveOccurred())
			Expect(resourcesExist).To(BeFalse())
		})

		It("should fail if the reconciliation flow failed", func() {
			err := errors.New("test")
			cleaner.EXPECT().GetNamespace(gomock.Any()).Return(nil, err).AnyTimes()
			cleaner.EXPECT().GetCluster(gomock.Any()).Return(cluster, nil)
			cleaner.EXPECT().GetBackupEntry(gomock.Any()).Return(backupEntry, nil)
			cleaner.EXPECT().GetDNSOwners(gomock.Any()).Return(nil, nil)

			resourcesExist, err := actuator.Reconcile(ctx, shootLeftover)
			Expect(err).To(HaveOccurred())
			Expect(resourcesExist).To(BeTrue())
		})
	})

	Describe("#Delete", func() {
		BeforeEach(func() {
			expectPatchShootLeftoverStatus(func(slo *gardencorev1alpha1.ShootLeftover) {
				Expect(slo.Status.LastOperation).To(Not(BeNil()))
			}).AnyTimes()
		})

		It("should return false if the deletion flow succeeded", func() {
			var (
				migrateExtensionObjects           = cleaner.EXPECT().MigrateExtensionObjects(gomock.Any()).Return(nil)
				waitUntilExtensionObjectsMigrated = cleaner.EXPECT().WaitUntilExtensionObjectsMigrated(gomock.Any()).Return(nil).After(migrateExtensionObjects)
				deleteExtensionObjects            = cleaner.EXPECT().DeleteExtensionObjects(gomock.Any()).Return(nil).After(waitUntilExtensionObjectsMigrated)
				waitUntilExtensionObjectsDeleted  = cleaner.EXPECT().WaitUntilExtensionObjectsDeleted(gomock.Any()).Return(nil).After(deleteExtensionObjects)
				migrateBackupEntry                = cleaner.EXPECT().MigrateBackupEntry(gomock.Any()).Return(nil)
				waitUntilBackupEntryMigrated      = cleaner.EXPECT().WaitUntilBackupEntryMigrated(gomock.Any()).Return(nil).After(migrateBackupEntry)
				deleteBackupEntry                 = cleaner.EXPECT().DeleteBackupEntry(gomock.Any()).Return(nil).After(waitUntilBackupEntryMigrated)
				waitUntilBackupEntryDeleted       = cleaner.EXPECT().WaitUntilBackupEntryDeleted(gomock.Any()).Return(nil).After(deleteBackupEntry)
				deleteCluster                     = cleaner.EXPECT().DeleteCluster(gomock.Any()).Return(nil).After(waitUntilExtensionObjectsDeleted).After(waitUntilBackupEntryDeleted)
				_                                 = cleaner.EXPECT().WaitUntilClusterDeleted(gomock.Any()).Return(nil).After(deleteCluster)
				deleteEtcds                       = cleaner.EXPECT().DeleteEtcds(gomock.Any()).Return(nil)
				waitUntilEtcdsDeleted             = cleaner.EXPECT().WaitUntilEtcdsDeleted(gomock.Any()).Return(nil).After(deleteEtcds)
				setKeepObjectsForManagedResources = cleaner.EXPECT().SetKeepObjectsForManagedResources(gomock.Any()).Return(nil)
				deleteManagedResources            = cleaner.EXPECT().DeleteManagedResources(gomock.Any()).Return(nil).After(setKeepObjectsForManagedResources)
				waitUntilManagedResourcesDeleted  = cleaner.EXPECT().WaitUntilManagedResourcesDeleted(gomock.Any()).Return(nil).After(deleteManagedResources)
				deleteDNSOwners                   = cleaner.EXPECT().DeleteDNSOwners(gomock.Any()).Return(nil)
				waitUntilDNSOwnersDeleted         = cleaner.EXPECT().WaitUntilDNSOwnersDeleted(gomock.Any()).Return(nil).After(deleteDNSOwners)
				deleteDNSEntries                  = cleaner.EXPECT().DeleteDNSEntries(gomock.Any()).Return(nil).After(waitUntilDNSOwnersDeleted)
				waitUntilDNSEntriesDeleted        = cleaner.EXPECT().WaitUntilDNSEntriesDeleted(gomock.Any()).Return(nil).After(deleteDNSEntries)
				deleteDNSProviders                = cleaner.EXPECT().DeleteDNSProviders(gomock.Any()).Return(nil).After(waitUntilDNSEntriesDeleted)
				waitUntilDNSProvidersDeleted      = cleaner.EXPECT().WaitUntilDNSProvidersDeleted(gomock.Any()).Return(nil).After(deleteDNSProviders)
				deleteSecrets                     = cleaner.EXPECT().DeleteSecrets(gomock.Any()).Return(nil).After(waitUntilExtensionObjectsDeleted).After(waitUntilEtcdsDeleted).After(waitUntilManagedResourcesDeleted).After(waitUntilDNSEntriesDeleted).After(waitUntilDNSProvidersDeleted)
				deleteNamespace                   = cleaner.EXPECT().DeleteNamespace(gomock.Any()).Return(nil).After(deleteSecrets)
				_                                 = cleaner.EXPECT().WaitUntilNamespaceDeleted(gomock.Any()).Return(nil).After(deleteNamespace)
			)

			resourcesExist, err := actuator.Delete(ctx, shootLeftover)
			Expect(err).ToNot(HaveOccurred())
			Expect(resourcesExist).To(BeFalse())
		})

		It("should fail and return true if the deletion flow failed", func() {
			err := errors.New("test")
			cleaner.EXPECT().MigrateExtensionObjects(gomock.Any()).Return(err).AnyTimes()
			cleaner.EXPECT().MigrateBackupEntry(gomock.Any()).Return(err).AnyTimes()
			cleaner.EXPECT().DeleteEtcds(gomock.Any()).Return(err).AnyTimes()
			cleaner.EXPECT().SetKeepObjectsForManagedResources(gomock.Any()).Return(err).AnyTimes()
			cleaner.EXPECT().DeleteDNSOwners(gomock.Any()).Return(err).AnyTimes()

			resourcesExist, err := actuator.Delete(ctx, shootLeftover)
			Expect(err).To(HaveOccurred())
			Expect(resourcesExist).To(BeTrue())
		})
	})
})
