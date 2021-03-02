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
	"errors"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	// OverwriteSeedName indicates whether the provided SeedName in the values shall be used even if the BackupEntry
	// already exists (if this is false then the existing `.spec.seedName` will be kept even if the SeedName in these
	// values differs.
	OverwriteSeedName bool
	// BucketName is the name of the bucket in which the BackupEntry shall be reconciled. This value is only used if the
	// BackupEntry does not exist yet. Otherwise, the existing `.spec.bucketName` will be kept even if the BucketName in
	// these values differs.
	BucketName string
}

// New creates a new instance of DeployWaiter for a BackupEntry.
func New(
	logger logrus.FieldLogger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitTimeout time.Duration,
) component.DeployWaiter {
	return &backupEntry{
		client:       client,
		logger:       logger,
		values:       values,
		waitInterval: waitInterval,
		waitTimeout:  waitTimeout,
	}
}

type backupEntry struct {
	values       *Values
	logger       logrus.FieldLogger
	client       client.Client
	waitInterval time.Duration
	waitTimeout  time.Duration
}

// Deploy uses the garden client to create or update the BackupEntry resource in the project namespace in the Garden.
func (b *backupEntry) Deploy(ctx context.Context) error {
	var (
		backupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.values.Name,
				Namespace: b.values.Namespace,
			},
		}
		bucketName = b.values.BucketName
		seedName   = b.values.SeedName
	)

	if err := b.client.Get(ctx, kutil.Key(b.values.Namespace, b.values.Name), backupEntry); err == nil {
		bucketName = backupEntry.Spec.BucketName
		seedName = backupEntry.Spec.SeedName
	} else if client.IgnoreNotFound(err) != nil {
		return err
	}

	if b.values.OverwriteSeedName {
		seedName = b.values.SeedName
	}

	_, err := controllerutil.CreateOrUpdate(ctx, b.client, backupEntry, func() error {
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())
		if b.values.ShootPurpose != nil {
			metav1.SetMetaDataAnnotation(&backupEntry.ObjectMeta, v1beta1constants.ShootPurpose, string(*b.values.ShootPurpose))
		}

		finalizers := sets.NewString(backupEntry.GetFinalizers()...)
		finalizers.Insert(gardencorev1beta1.GardenerName)
		backupEntry.SetFinalizers(finalizers.UnsortedList())

		if b.values.OwnerReference != nil {
			backupEntry.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*b.values.OwnerReference}
		}

		backupEntry.Spec.BucketName = bucketName
		backupEntry.Spec.SeedName = seedName

		return nil
	})

	return err
}

// Wait waits until the BackupEntry resource is ready.
func (b *backupEntry) Wait(ctx context.Context) error {
	return retry.UntilTimeout(ctx, b.waitInterval, b.waitTimeout, func(ctx context.Context) (done bool, err error) {
		be := &gardencorev1beta1.BackupEntry{}
		if err := b.client.Get(ctx, kutil.Key(b.values.Namespace, b.values.Name), be); err != nil {
			return retry.SevereError(err)
		}

		if be.Status.LastOperation != nil {
			if be.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
				b.logger.Info("Backup entry has been successfully reconciled.")
				return retry.Ok()
			}
			if be.Status.LastOperation.State == gardencorev1beta1.LastOperationStateError {
				b.logger.Info("Backup entry has been reconciled with error.")
				return retry.SevereError(errors.New(be.Status.LastError.Description))
			}
		}

		b.logger.Info("Waiting until the backup entry has been reconciled in the Garden cluster...")
		return retry.MinorError(fmt.Errorf("backup entry %q has not yet been reconciled", be.Name))
	})
}

// Destroy is not implemented yet.
func (b *backupEntry) Destroy(_ context.Context) error { return nil }

// WaitCleanup is not implemented yet.
func (b *backupEntry) WaitCleanup(_ context.Context) error { return nil }
