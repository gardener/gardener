// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait
	// for a successful reconciliation of a BackupBucket resource.
	DefaultTimeout = 10 * time.Minute
)

// Values contains the values used to create a BackupBucket resource.
type Values struct {
	// Name is the name of the BackupBucket resource.
	Name string
	// Config is the backup configuration.
	Config *gardencorev1beta1.Backup
	// DefaultRegion is the default region where the bucket should be deployed to.
	DefaultRegion string
	// Seed is an optional Seed object related to the BackupBucket.
	Seed *gardencorev1beta1.Seed
	// Clock is a clock.
	Clock clock.Clock
}

// New creates a new instance of DeployWaiter for a BackupBucket. It takes a garden client and returns a deployer for a
// core.gardener.cloud/v1beta1.BackupBucket resource in the garden cluster.
func New(
	log logr.Logger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &backupBucket{
		log:          log,
		client:       client,
		values:       values,
		waitInterval: waitInterval,
		waitTimeout:  waitTimeout,

		backupBucket: &gardencorev1beta1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: values.Name,
			},
		},
	}
}

// Interface contains functions for a BackupBucket deployer.
type Interface interface {
	component.DeployWaiter
	// Get retrieves and returns the BackupBucket resource based on the configured values.
	Get(context.Context) (*gardencorev1beta1.BackupBucket, error)
}

type backupBucket struct {
	log          logr.Logger
	values       *Values
	client       client.Client
	waitInterval time.Duration
	waitTimeout  time.Duration

	backupBucket *gardencorev1beta1.BackupBucket
}

// Deploy uses the garden client to create or update the BackupBucket resource in the Garden.
func (b *backupBucket) Deploy(ctx context.Context) error {
	_, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, b.client, b.backupBucket, func() error {
		metav1.SetMetaDataAnnotation(&b.backupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&b.backupBucket.ObjectMeta, v1beta1constants.GardenerTimestamp, b.values.Clock.Now().UTC().Format(time.RFC3339Nano))

		b.backupBucket.Spec = gardencorev1beta1.BackupBucketSpec{
			Provider: gardencorev1beta1.BackupBucketProvider{
				Type:   b.values.Config.Provider,
				Region: ptr.Deref(b.values.Config.Region, b.values.DefaultRegion),
			},
			ProviderConfig: b.values.Config.ProviderConfig,
			SecretRef: corev1.SecretReference{ // TODO(vpnachev): Add support for WorkloadIdentity
				Name:      b.values.Config.CredentialsRef.Name,
				Namespace: b.values.Config.CredentialsRef.Namespace,
			},
		}

		if b.values.Seed != nil {
			ownerRef := metav1.NewControllerRef(b.values.Seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))
			b.backupBucket.OwnerReferences = []metav1.OwnerReference{*ownerRef}
			b.backupBucket.Spec.SeedName = &b.values.Seed.Name
		}

		return nil
	})
	return err
}

// Wait waits until the BackupBucket resource is ready.
func (b *backupBucket) Wait(ctx context.Context) error {
	return extensions.WaitUntilObjectReadyWithHealthFunction(
		ctx,
		b.client,
		b.log,
		health.CheckBackupBucket,
		b.backupBucket,
		"BackupBucket",
		b.waitInterval,
		b.waitTimeout,
		b.waitTimeout,
		nil,
	)
}

// Destroy deletes the BackupBucket resource
func (b *backupBucket) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObject(
		ctx,
		b.client,
		b.backupBucket,
	)
}

// WaitCleanup waits until the BackupBucket is deleted.
func (b *backupBucket) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, b.waitTimeout)
	defer cancel()
	return kubernetesutils.WaitUntilResourceDeleted(timeoutCtx, b.client, b.backupBucket, b.waitInterval)
}

// Get retrieves and returns the BackupBucket resource based on the configured values.
func (b *backupBucket) Get(ctx context.Context) (*gardencorev1beta1.BackupBucket, error) {
	if err := b.client.Get(ctx, client.ObjectKeyFromObject(b.backupBucket), b.backupBucket); err != nil {
		return nil, err
	}
	return b.backupBucket, nil
}
