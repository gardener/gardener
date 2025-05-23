// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
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
	log logr.Logger,
	client client.Client,
	clock clock.Clock,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &backupEntry{
		log:                 log,
		client:              client,
		clock:               clock,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		backupEntry: &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: values.Name,
			},
		},
	}
}

type backupEntry struct {
	values              *Values
	log                 logr.Logger
	client              client.Client
	clock               clock.Clock
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	backupEntry *extensionsv1alpha1.BackupEntry
}

// Deploy uses the seed client to create or update the BackupEntry custom resource in the Seed.
func (b *backupEntry) Deploy(ctx context.Context) error {
	_, err := b.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
	return err
}

func (b *backupEntry) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.client, b.backupEntry, func() error {
		metav1.SetMetaDataAnnotation(&b.backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&b.backupEntry.ObjectMeta, v1beta1constants.GardenerTimestamp, b.clock.Now().UTC().Format(time.RFC3339Nano))

		b.backupEntry.Spec = extensionsv1alpha1.BackupEntrySpec{
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

	return b.backupEntry, err
}

// Restore uses the seed client and the ShootState to create the BackupEntry custom resource in the Seed and restore its
// state.
func (b *backupEntry) Restore(ctx context.Context, shootState *gardencorev1beta1.ShootState) error {
	return extensions.RestoreExtensionWithDeployFunction(
		ctx,
		b.client,
		shootState,
		extensionsv1alpha1.BackupEntryResource,
		b.deploy,
	)
}

// Migrate migrates the BackupEntry custom resource
func (b *backupEntry) Migrate(ctx context.Context) error {
	return extensions.MigrateExtensionObject(
		ctx,
		b.client,
		b.backupEntry,
	)
}

// WaitMigrate waits until the BackupEntry custom resource has been successfully migrated.
func (b *backupEntry) WaitMigrate(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectMigrated(
		ctx,
		b.client,
		b.backupEntry,
		extensionsv1alpha1.BackupEntryResource,
		b.waitInterval,
		b.waitTimeout,
	)
}

// Destroy deletes the BackupEntry CRD
func (b *backupEntry) Destroy(ctx context.Context) error {
	return extensions.DeleteExtensionObject(
		ctx,
		b.client,
		b.backupEntry,
	)
}

// Wait waits until the BackupEntry CRD is ready (deployed or restored)
func (b *backupEntry) Wait(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectReady(
		ctx,
		b.client,
		b.log,
		b.backupEntry,
		extensionsv1alpha1.BackupEntryResource,
		b.waitInterval,
		b.waitSevereThreshold,
		b.waitTimeout,
		nil,
	)
}

// WaitCleanup waits until the BackupEntry CRD is deleted
func (b *backupEntry) WaitCleanup(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		b.client,
		b.log,
		b.backupEntry,
		extensionsv1alpha1.BackupEntryResource,
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
