// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionvalidation

import (
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// HandlerName is the name of this webhook handler.
	HandlerName = "extension-validation"
	// WebhookPathBackupBucket is the HTTP handler path for this admission webhook handler for BackupBucket.
	WebhookPathBackupBucket = "/validate-extensions-gardener-cloud-v1alpha1-backupbucket"
	// WebhookPathBackupEntry is the HTTP handler path for this admission webhook handler for BackupEntry.
	WebhookPathBackupEntry = "/validate-extensions-gardener-cloud-v1alpha1-backupentry"
	// WebhookPathBastion is the HTTP handler path for this admission webhook handler for Bastion.
	WebhookPathBastion = "/validate-extensions-gardener-cloud-v1alpha1-bastion"
	// WebhookPathContainerRuntime is the HTTP handler path for this admission webhook handler for ContainerRuntime.
	WebhookPathContainerRuntime = "/validate-extensions-gardener-cloud-v1alpha1-containerruntime"
	// WebhookPathControlPlane is the HTTP handler path for this admission webhook handler for ControlPlane.
	WebhookPathControlPlane = "/validate-extensions-gardener-cloud-v1alpha1-controlplane"
	// WebhookPathDNSRecord is the HTTP handler path for this admission webhook handler DNSRecord.
	WebhookPathDNSRecord = "/validate-extensions-gardener-cloud-v1alpha1-dnsrecord"
	// WebhookPathEtcd is the HTTP handler path for this admission webhook handler for Etcd.
	WebhookPathEtcd = "/validate-druid-gardener-cloud-v1alpha1-etcd"
	// WebhookPathExtension is the HTTP handler path for this admission webhook handler for Extension.
	WebhookPathExtension = "/validate-extensions-gardener-cloud-v1alpha1-extension"
	// WebhookPathInfrastructure is the HTTP handler path for this admission webhook handler Infrastructure.
	WebhookPathInfrastructure = "/validate-extensions-gardener-cloud-v1alpha1-infrastructure"
	// WebhookPathNetwork is the HTTP handler path for this admission webhook handler for Network.
	WebhookPathNetwork = "/validate-extensions-gardener-cloud-v1alpha1-network"
	// WebhookPathOperatingSystemConfig is the HTTP handler path for this admission webhook handler for OperatingSystemConfig.
	WebhookPathOperatingSystemConfig = "/validate-extensions-gardener-cloud-v1alpha1-operatingsystemconfig"
	// WebhookPathSelfHostedShootExposure is the HTTP handler path for this admission webhook handler for SelfHostedShootExposure.
	WebhookPathSelfHostedShootExposure = "/validate-extensions-gardener-cloud-v1alpha1-selfhostedshootexposure"
	// WebhookPathWorker is the HTTP handler path for this admission webhook handler for Worker.
	WebhookPathWorker = "/validate-extensions-gardener-cloud-v1alpha1-worker"
)

// AddToManager add the validators to the given managers.
func AddToManager(mgr manager.Manager) error {
	for _, register := range []func() error{
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.BackupBucket{}).WithValidator(&backupBucketValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.BackupEntry{}).WithValidator(&backupEntryValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.Bastion{}).WithValidator(&bastionValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.ContainerRuntime{}).WithValidator(&containerRuntimeValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.ControlPlane{}).WithValidator(&controlPlaneValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.DNSRecord{}).WithValidator(&dnsRecordValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &druidcorev1alpha1.Etcd{}).WithValidator(&etcdValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.Extension{}).WithValidator(&extensionValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.Infrastructure{}).WithValidator(&infrastructureValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.Network{}).WithValidator(&networkValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.OperatingSystemConfig{}).WithValidator(&operatingSystemConfigValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.SelfHostedShootExposure{}).WithValidator(&selfHostedShootExposureValidator{}).Complete()
		},
		func() error {
			return builder.WebhookManagedBy(mgr, &extensionsv1alpha1.Worker{}).WithValidator(&workerValidator{}).Complete()
		},
	} {
		if err := register(); err != nil {
			return err
		}
	}

	return nil
}
