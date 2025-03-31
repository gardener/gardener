// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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
	// Class holds the extension class used to control the responsibility for multiple provider extensions.
	Class *extensionsv1alpha1.ExtensionClass
	// SecretData is the secret data of the DNSRecord (containing provider credentials, etc.). If not provided, the
	// secret in the Namespace with name SecretName will be referenced in the DNSRecord object.
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
	// IPStack is the indication of the IP stack used for the DNSRecord. It can be ipv4, ipv6 or dual-stack.
	IPStack string
	// Labels is a set of labels that should be applied to the DNSRecord resource.
	Labels map[string]string
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
	deployer := &dnsRecord{
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
	}

	if values.SecretData != nil {
		deployer.secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      values.SecretName,
				Namespace: values.Namespace,
			},
		}
	}

	return deployer
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
		if d.values.AnnotateOperation ||
			d.valuesDontMatchDNSRecord() ||
			d.lastOperationNotSuccessful() ||
			d.isTimestampInvalidOrAfterLastUpdateTime() {
			metav1.SetMetaDataAnnotation(&d.dnsRecord.ObjectMeta, v1beta1constants.GardenerOperation, operation)
			metav1.SetMetaDataAnnotation(&d.dnsRecord.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().Format(time.RFC3339Nano))
		}

		if d.values.IPStack != "" {
			metav1.SetMetaDataAnnotation(&d.dnsRecord.ObjectMeta, gardenerutils.AnnotationKeyIPStack, d.values.IPStack)
		}

		secretRef := corev1.SecretReference{Name: d.values.SecretName, Namespace: d.values.Namespace}
		if d.secret != nil {
			secretRef = corev1.SecretReference{Name: d.secret.Name, Namespace: d.secret.Namespace}
		}

		for k, v := range d.values.Labels {
			if d.dnsRecord.Labels == nil {
				d.dnsRecord.Labels = make(map[string]string)
			}
			d.dnsRecord.Labels[k] = v
		}

		d.dnsRecord.Spec = extensionsv1alpha1.DNSRecordSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type:  d.values.Type,
				Class: d.values.Class,
			},
			SecretRef:  secretRef,
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
			if d.valuesDontMatchDNSRecord() ||
				d.lastOperationNotSuccessful() ||
				d.isTimestampInvalidOrAfterLastUpdateTime() {
				// If the DNSRecord is not yet Succeeded or values have changed, reconcile it again.
				// Also check if gardener timestamp is in an invalid format or is after status.LastOperation.LastUpdateTime.
				// If that is the case health checks for the dnsrecord will fail so we request a reconciliation to correct the current state.
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
	if d.secret == nil {
		return nil
	}

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
		!ptr.Equal(d.values.Zone, d.dnsRecord.Spec.Zone) ||
		!reflect.DeepEqual(d.values.Values, d.dnsRecord.Spec.Values) ||
		!ptr.Equal(d.values.TTL, d.dnsRecord.Spec.TTL)
}

func (d *dnsRecord) lastOperationNotSuccessful() bool {
	return d.dnsRecord.Status.LastOperation != nil && d.dnsRecord.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded
}

// isTimestampInvalidOrAfterLastUpdateTime returns true if v1beta1constants.GardenerTimestamp is after status.LastOperation.LastUpdateTime
// or if v1beta1constants.GardenerTimestamp is in invalid format
func (d *dnsRecord) isTimestampInvalidOrAfterLastUpdateTime() bool {
	timestamp, ok := d.dnsRecord.Annotations[v1beta1constants.GardenerTimestamp]
	if ok && d.dnsRecord.Status.LastOperation != nil {
		parsedTimestamp, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			// this should not happen
			// we cannot do anything meaningful about this error so we mark the timestamp invalid
			return true
		}

		if parsedTimestamp.Truncate(time.Second).UTC().After(d.dnsRecord.Status.LastOperation.LastUpdateTime.UTC()) {
			return true
		}
	}

	return false
}
