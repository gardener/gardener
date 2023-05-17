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

package dnsrecord

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
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
	// DefaultSevereThreshold is the default threshold until an error reported by another component is treated as
	// 'severe'.
	DefaultSevereThreshold = 15 * time.Second
	// DefaultTimeout is the default timeout and defines how long Gardener should wait for a successful reconciliation
	// of a DNSRecord resource.
	DefaultTimeout = 2 * time.Minute
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Interface is an interface for managing DNSRecords
type Interface interface {
	component.DeployMigrateWaiter
	GetValues() *Values
	SetRecordType(extensionsv1alpha1.DNSRecordType)
	SetValues([]string)
}

// Values contains the values used to create DNSRecord resources.
type Values struct {
	// Namespace is the Shoot namespace in the seed.
	Namespace string
	// Name is the name of the DNSRecord resource. Commonly the Shoot's name + the purpose of the DNS record.
	Name string
	// SecretName is the name of the secret referenced by the DNSRecord resource.
	SecretName string
	// ReconcileOnlyOnChangeOrError specifies that the DNSRecord resource should only be reconciled when first created
	// or if its last operation was not successful or if its desired state has changed compared to the current one.
	ReconcileOnlyOnChangeOrError bool
	// AnnotateOperation indicates if the DNSRecord resource shall be annotated with the respective
	// "gardener.cloud/operation" (forcing a reconciliation or restoration). If this is false then the DNSRecord object
	// will be created/updated but the extension controller will not act upon it.
	AnnotateOperation bool
	// Type is the type of the DNSRecord provider.
	Type string
	// SecretData is the secret data of the DNSRecord (containing provider credentials, etc.)
	SecretData map[string][]byte
	// Zone is the DNS hosted zone of the DNSRecord.
	Zone *string
	// DNSName is the fully qualified domain name of the DNSRecord.
	DNSName string
	// RecordType is the record type of the DNSRecord.
	RecordType extensionsv1alpha1.DNSRecordType
	// Values is the list of values of the DNSRecord.
	Values []string
	// TTL is the time to live in seconds of the DNSRecord.
	TTL *int64
}

// New creates a new instance that implements component.DeployMigrateWaiter.
func New(
	log logr.Logger,
	client client.Client,
	values *Values,
	waitInterval time.Duration,
	waitSevereThreshold time.Duration,
	waitTimeout time.Duration,
) Interface {
	return &dnsRecord{
		log:                 log,
		client:              client,
		values:              values,
		waitInterval:        waitInterval,
		waitSevereThreshold: waitSevereThreshold,
		waitTimeout:         waitTimeout,

		dnsRecord: &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.Name,
				Namespace: values.Namespace,
			},
		},
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.SecretName,
				Namespace: values.Namespace,
			},
		},
	}
}

type dnsRecord struct {
	log                 logr.Logger
	client              client.Client
	values              *Values
	waitInterval        time.Duration
	waitSevereThreshold time.Duration
	waitTimeout         time.Duration

	dnsRecord *extensionsv1alpha1.DNSRecord
	secret    *corev1.Secret
}

// Deploy uses the seed client to create or update the DNSRecord resource.
func (d *dnsRecord) Deploy(ctx context.Context) error {
	_, err := d.deploy(ctx, v1beta1constants.GardenerOperationReconcile)
	return err
}

func (d *dnsRecord) deploy(ctx context.Context, operation string) (extensionsv1alpha1.Object, error) {
	if err := d.deploySecret(ctx); err != nil {
		return nil, err
	}

	mutateFn := func() error {
		if d.values.AnnotateOperation || d.valuesDontMatchDNSRecord() || d.lastOperationNotSuccessful() || d.statusIsOutdatedOrTimestampIsInvalid() {
			metav1.SetMetaDataAnnotation(&d.dnsRecord.ObjectMeta, v1beta1constants.GardenerOperation, operation)
			metav1.SetMetaDataAnnotation(&d.dnsRecord.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))
		}

		d.dnsRecord.Spec = extensionsv1alpha1.DNSRecordSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: d.values.Type,
			},
			SecretRef: corev1.SecretReference{
				Name:      d.secret.Name,
				Namespace: d.secret.Namespace,
			},
			Zone:       d.values.Zone,
			Name:       d.values.DNSName,
			RecordType: d.values.RecordType,
			Values:     d.values.Values,
			TTL:        d.values.TTL,
		}

		return nil
	}

	if d.values.ReconcileOnlyOnChangeOrError {
		if err := d.client.Get(ctx, client.ObjectKeyFromObject(d.dnsRecord), d.dnsRecord); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, err
			}

			// DNSRecord doesn't exist yet, create it.
			_ = mutateFn()
			if err := d.client.Create(ctx, d.dnsRecord); err != nil {
				return nil, err
			}
		} else {
			patch := client.MergeFrom(d.dnsRecord.DeepCopy())
			if d.valuesDontMatchDNSRecord() || d.lastOperationNotSuccessful() || d.statusIsOutdatedOrTimestampIsInvalid() {
				// If the DNSRecord is not yet Succeeded or values have changed, reconcile it again.
				_ = mutateFn()
			}
			if err := d.client.Patch(ctx, d.dnsRecord, patch); err != nil {
				return nil, err
			}
		}
	} else {
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, d.client, d.dnsRecord, mutateFn); err != nil {
			return nil, err
		}
	}

	return d.dnsRecord, nil
}

func (d *dnsRecord) deploySecret(ctx context.Context) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, d.client, d.secret, func() error {
		d.secret.Type = corev1.SecretTypeOpaque
		d.secret.Data = d.values.SecretData
		return nil
	})
	return err
}

// Restore uses the seed client and the ShootState to create the DNSRecord resource and restore its state.
func (d *dnsRecord) Restore(ctx context.Context, shootState *gardencorev1beta1.ShootState) error {
	return extensions.RestoreExtensionWithDeployFunction(
		ctx,
		d.client,
		shootState,
		extensionsv1alpha1.DNSRecordResource,
		d.deploy,
	)
}

// Migrate migrates the DNSRecord resource.
func (d *dnsRecord) Migrate(ctx context.Context) error {
	return extensions.MigrateExtensionObject(
		ctx,
		d.client,
		d.dnsRecord,
	)
}

// Destroy deletes the DNSRecord resource.
func (d *dnsRecord) Destroy(ctx context.Context) error {
	if err := d.deploySecret(ctx); err != nil {
		return err
	}

	return extensions.DeleteExtensionObject(
		ctx,
		d.client,
		d.dnsRecord,
	)
}

// WaitUntilExtensionObjectReady is an alias for extensions.WaitUntilExtensionObjectReady. Exposed for tests.
var WaitUntilExtensionObjectReady = extensions.WaitUntilExtensionObjectReady

// Wait waits until the DNSRecord resource is ready.
func (d *dnsRecord) Wait(ctx context.Context) error {
	return WaitUntilExtensionObjectReady(
		ctx,
		d.client,
		d.log,
		d.dnsRecord,
		extensionsv1alpha1.DNSRecordResource,
		d.waitInterval,
		d.waitSevereThreshold,
		d.waitTimeout,
		nil,
	)
}

// WaitMigrate waits until the DNSRecord resource is migrated successfully.
func (d *dnsRecord) WaitMigrate(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectMigrated(
		ctx,
		d.client,
		d.dnsRecord,
		extensionsv1alpha1.DNSRecordResource,
		d.waitInterval,
		d.waitTimeout,
	)
}

// WaitCleanup waits until the DNSRecord resource is deleted.
func (d *dnsRecord) WaitCleanup(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		d.client,
		d.log,
		d.dnsRecord,
		extensionsv1alpha1.DNSRecordResource,
		d.waitInterval,
		d.waitTimeout,
	)
}

// GetValues returns the current configuration values of the deployer.
func (d *dnsRecord) GetValues() *Values {
	return d.values
}

// SetRecordType sets the record type in the values.
func (d *dnsRecord) SetRecordType(recordType extensionsv1alpha1.DNSRecordType) {
	d.values.RecordType = recordType
}

// SetValues sets the values in the values.
func (d *dnsRecord) SetValues(values []string) {
	d.values.Values = values
}

func (d *dnsRecord) valuesDontMatchDNSRecord() bool {
	return d.values.SecretName != d.dnsRecord.Spec.SecretRef.Name ||
		!pointer.StringEqual(d.values.Zone, d.dnsRecord.Spec.Zone) ||
		!reflect.DeepEqual(d.values.Values, d.dnsRecord.Spec.Values) ||
		!pointer.Int64Equal(d.values.TTL, d.dnsRecord.Spec.TTL)
}

func (d *dnsRecord) lastOperationNotSuccessful() bool {
	return d.dnsRecord.Status.LastOperation != nil && d.dnsRecord.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded
}

func (d *dnsRecord) statusIsOutdatedOrTimestampIsInvalid() bool {
	timestamp, ok := d.dnsRecord.Annotations[v1beta1constants.GardenerTimestamp]
	if ok && d.dnsRecord.Status.LastOperation != nil {
		parsedTimestamp, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			// this should not happen
			// we cannot do anything meaningful about this error so we mark the timestamp invalid
			return true
		}

		if parsedTimestamp.Truncate(time.Second).UTC().After(d.dnsRecord.Status.LastOperation.LastUpdateTime.Time.UTC()) {
			return true
		}
	}

	return false
}
