// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensionvalidation

import (
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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
	// WebhookPathWorker is the HTTP handler path for this admission webhook handler for Worker.
	WebhookPathWorker = "/validate-extensions-gardener-cloud-v1alpha1-worker"
)

// AddToManager add the validators to the given managers.
func AddToManager(mgr manager.Manager) error {
	for obj, validator := range map[client.Object]admission.CustomValidator{
		&extensionsv1alpha1.BackupBucket{}:          &backupBucketValidator{},
		&extensionsv1alpha1.BackupEntry{}:           &backupEntryValidator{},
		&extensionsv1alpha1.Bastion{}:               &bastionValidator{},
		&extensionsv1alpha1.ContainerRuntime{}:      &containerRuntimeValidator{},
		&extensionsv1alpha1.ControlPlane{}:          &controlPlaneValidator{},
		&extensionsv1alpha1.DNSRecord{}:             &dnsRecordValidator{},
		&druidv1alpha1.Etcd{}:                       &etcdValidator{},
		&extensionsv1alpha1.Extension{}:             &extensionValidator{},
		&extensionsv1alpha1.Infrastructure{}:        &infrastructureValidator{},
		&extensionsv1alpha1.Network{}:               &networkValidator{},
		&extensionsv1alpha1.OperatingSystemConfig{}: &operatingSystemConfigValidator{},
		&extensionsv1alpha1.Worker{}:                &workerValidator{},
	} {
		if err := builder.
			WebhookManagedBy(mgr).
			WithValidator(validator).
			For(obj).
			RecoverPanic().
			Complete(); err != nil {
			return err
		}
	}

	return nil
}
