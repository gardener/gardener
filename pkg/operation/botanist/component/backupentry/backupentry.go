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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of a BackupEntry resource.
	DefaultTimeout = 10 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Values contains the values used to create a BackupEntry resource.
type Values struct {
	// Namespace is the namespace of the BackupEntry resource.
	Namespace string
	// Name is the name of the BackupEntry resource.
	Name string
	// ShootPurpose is the purpose of the shoot.
	ShootPurpose *gardencorev1beta1.ShootPurpose
	// OwnerReference is a reference to an owner for BackupEntry resource.
	OwnerReference *metav1.OwnerReference
	// SeedName is the name of the seed to which the BackupEntry shall be scheduled.
	SeedName *string
	// BucketName is the name of the bucket in which the BackupEntry shall be reconciled. This value is only used if the
	// BackupEntry does not exist yet. Otherwise, the existing `.spec.bucketName` will be kept even if the BucketName in
	// these values differs.
	BucketName string
}

// New creates a new instance of DeployWaiter for a BackupEntry.
func New(
	log logr.Logger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &backupEntry{
		log:          log,
		client:       client,
		values:       values,
		waitInterval: waitInterval,
		waitTimeout:  waitTimeout,

		backupEntry: &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: values.Namespace,
			},
		},
	}
}

// Interface contains functions for a BackupEntry deployer.
type Interface interface {
	component.DeployMigrateWaiter
	// Get retrieves and returns the BackupEntry resource based on the configured values.
	Get(context.Context) (*gardencorev1beta1.BackupEntry, error)
	// GetActualBucketName returns the name of the BackupBucket that this BackupEntry was created with.
	GetActualBucketName() string
	// SetBucketName sets the name of the BackupBucket for this BackupEntry.
	SetBucketName(string)
}

type backupEntry struct {
	log          logr.Logger
	values       *Values
	client       client.Client
	waitInterval time.Duration
	waitTimeout  time.Duration

	backupEntry *gardencorev1beta1.BackupEntry
}

// Deploy uses the garden client to create or update the BackupEntry resource in the project namespace in the Garden.
func (b *backupEntry) Deploy(ctx context.Context) error {
	var (
		bucketName = b.values.BucketName
		seedName   = b.values.SeedName
	)

	if err := b.client.Get(ctx, client.ObjectKeyFromObject(b.backupEntry), b.backupEntry); err == nil {
		bucketName = b.backupEntry.Spec.BucketName
		seedName = b.backupEntry.Spec.SeedName
	} else if client.IgnoreNotFound(err) != nil {
		return err
	}

	return b.reconcile(ctx, b.backupEntry, seedName, bucketName, v1beta1constants.GardenerOperationReconcile)
}

// Wait waits until the BackupEntry resource is ready.
func (b *backupEntry) Wait(ctx context.Context) error {
	return extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		b.client,
		b.log,
		health.CheckBackupEntry,
		b.backupEntry,
		"BackupEntry",
		b.waitInterval,
		b.waitTimeout,
		b.waitTimeout,
		nil,
	)
}

// Migrate uses the garden client to deschedule the BackupEntry from its current seed.
func (b *backupEntry) Migrate(ctx context.Context) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.client, b.backupEntry, func() error {
		b.backupEntry.Spec.SeedName = b.values.SeedName
		return nil
	})
	return err
}

// WaitMigrate waits until the BackupEntry is migrated
func (b *backupEntry) WaitMigrate(ctx context.Context) error {
	return b.Wait(ctx)
}

// Restore uses the garden client to update the BackupEntry and set the name of the new seed to which it shall be scheduled.
// If the BackupEntry was deleted it will be recreated.
func (b *backupEntry) Restore(ctx context.Context, _ *gardencorev1alpha1.ShootState) error {
	bucketName := b.values.BucketName
	if !gardenletfeatures.FeatureGate.Enabled(features.CopyEtcdBackupsDuringControlPlaneMigration) {
		if err := b.client.Get(ctx, kutil.Key(b.values.Namespace, b.values.Name), b.backupEntry); err == nil {
			bucketName = b.backupEntry.Spec.BucketName
		} else if client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return b.reconcile(ctx, b.backupEntry, b.values.SeedName, bucketName, v1beta1constants.GardenerOperationRestore)
}

func (b *backupEntry) reconcile(ctx context.Context, backupEntry *gardencorev1beta1.BackupEntry, seedName *string, bucketName string, operation string) error {
	_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, b.client, backupEntry, func() error {
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		if b.values.ShootPurpose != nil {
			metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.ShootPurpose, string(*b.values.ShootPurpose))
		}

		if b.values.OwnerReference != nil {
			backupEntry.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*b.values.OwnerReference}
		}

		backupEntry.Spec.BucketName = bucketName
		backupEntry.Spec.SeedName = seedName

		return nil
	})

	return err
}

// Destroy deletes the BackupEntry resource
func (b *backupEntry) Destroy(ctx context.Context) error {
	return kutil.DeleteObject(
		ctx,
		b.client,
		b.backupEntry,
	)
}

// WaitCleanup is not implemented yet.
func (b *backupEntry) WaitCleanup(_ context.Context) error { return nil }

// Get retrieves and returns the BackupEntry resource based on the configured values.
func (b *backupEntry) Get(ctx context.Context) (*gardencorev1beta1.BackupEntry, error) {
	if err := b.client.Get(ctx, client.ObjectKeyFromObject(b.backupEntry), b.backupEntry); err != nil {
		return nil, err
	}
	return b.backupEntry, nil
}

// GetActualBucketName returns the name of the BackupBucket that this BackupEntry was created with.
func (b *backupEntry) GetActualBucketName() string {
	return b.backupEntry.Spec.BucketName
}

// SetBackupBucket sets the name of the BackupBucket for this BackupEntry.
func (b *backupEntry) SetBucketName(name string) {
	b.values.BucketName = name
}
