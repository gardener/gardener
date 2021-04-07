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

package backupentry

import (
	"context"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as 'severe'.
	DefaultSevereThreshold = 30 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of a BackupEntry resource.
	DefaultTimeout = 10 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Interface is an interface for managing BackupEntries.
type Interface interface {
	component.DeployMigrateWaiter
	SetType(string)
	SetProviderConfig(*runtime.RawExtension)
	SetRegion(string)
	SetBackupBucketProviderStatus(*runtime.RawExtension)
}

// Values contains the values used to create a BackupEntry CRD
type Values struct {
	// Name is the name of the BackupEntry extension.
	Name string
	// Type is the type of BackupEntry plugin/extension.
	Type string
	// ProviderConfig contains the provider config for the BackupEntry extension.
	ProviderConfig *runtime.RawExtension
	// Region is the infrastructure region of the BackupEntry.
	Region string
	// SecretRef is a reference to a secret with the infrastructure credentials.
	SecretRef corev1.SecretReference
	// BucketName is the name of the bucket in which the entry shall be created.
	BucketName string
	// BackupBucketProviderStatus is the optional provider status of the BackupBucket.
	BackupBucketProviderStatus *runtime.RawExtension
}

// New creates a new instance of Interface.
func New(
	logger logrus.FieldLogger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &backupEntry{
		client:              client,
		logger:              logger,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,
	}
}

type backupEntry struct {
	values              *Values
	logger              logrus.FieldLogger
	client              client.Client
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration
}

// Deploy uses the seed client to create or update the BackupEntry custom resource in the Seed.
func (b *backupEntry) Deploy(ctx context.Context) error {
	backupEntry := &extensionsv1alpha1.BackupEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.values.Name,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, b.client, backupEntry, func() error {
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		backupEntry.Spec = extensionsv1alpha1.BackupEntrySpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:           b.values.Type,
				ProviderConfig: b.values.ProviderConfig,
			},
			Region:                     b.values.Region,
			SecretRef:                  b.values.SecretRef,
			BucketName:                 b.values.BucketName,
			BackupBucketProviderStatus: b.values.BackupBucketProviderStatus,
		}

		return nil
	})

	return err
}

// Restore uses the seed client and the ShootState to create the BackupEntry custom resource in the Seed and restore its
// state.
func (b *backupEntry) Restore(ctx context.Context, _ *gardencorev1alpha1.ShootState) error {
	return b.Deploy(ctx)
}

// Migrate migrates the BackupEntry custom resource
func (b *backupEntry) Migrate(ctx context.Context) error {
	return extensions.MigrateExtensionCR(
		ctx,
		b.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupEntry{} },
		"",
		b.values.Name,
	)
}

// WaitMigrate waits until the BackupEntry custom resource has been successfully migrated.
func (b *backupEntry) WaitMigrate(ctx context.Context) error {
	return extensions.WaitUntilExtensionCRMigrated(
		ctx,
		b.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupEntry{} },
		"",
		b.values.Name,
		b.waitInterval,
		b.waitTimeout,
	)
}

// Destroy deletes the BackupEntry CRD
func (b *backupEntry) Destroy(ctx context.Context) error {
	return extensions.DeleteExtensionCR(
		ctx,
		b.client,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupEntry{} },
		"",
		b.values.Name,
	)
}

// Wait waits until the BackupEntry CRD is ready (deployed or restored)
func (b *backupEntry) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionCRReady(
		ctx,
		b.client,
		b.logger,
		func() client.Object { return &extensionsv1alpha1.BackupEntry{} },
		extensionsv1alpha1.BackupEntryResource,
		"",
		b.values.Name,
		b.waitInterval,
		b.waitSevereThreshold,
		b.waitTimeout,
		nil,
	)
}

// WaitCleanup waits until the BackupEntry CRD is deleted
func (b *backupEntry) WaitCleanup(ctx context.Context) error {
	return extensions.WaitUntilExtensionCRDeleted(
		ctx,
		b.client,
		b.logger,
		func() extensionsv1alpha1.Object { return &extensionsv1alpha1.BackupEntry{} },
		extensionsv1alpha1.BackupEntryResource,
		"",
		b.values.Name,
		b.waitInterval,
		b.waitTimeout,
	)
}

func (b *backupEntry) SetType(t string) {
	b.values.Type = t
}

func (b *backupEntry) SetProviderConfig(providerConfig *runtime.RawExtension) {
	b.values.ProviderConfig = providerConfig
}

func (b *backupEntry) SetRegion(region string) {
	b.values.Region = region
}

func (b *backupEntry) SetBackupBucketProviderStatus(providerStatus *runtime.RawExtension) {
	b.values.BackupBucketProviderStatus = providerStatus
}
