// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensionresources

import (
	"context"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	druidvalidation "github.com/gardener/etcd-druid/api/validation"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/extensions/validation"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// BackupBucketWebhookPath is the HTTP handler path for this admission webhook handler for BackupBucket.
	BackupBucketWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-backupbucket"
	// BackupEntryWebhookPath is the HTTP handler path for this admission webhook handler for BackupEntry.
	BackupEntryWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-backupentry"
	// BastionWebhookPath is the HTTP handler path for this admission webhook handler for Bastion.
	BastionWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-bastion"
	// ContainerRuntimeWebhookPath is the HTTP handler path for this admission webhook handler for ContainerRuntime.
	ContainerRuntimeWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-containerruntime"
	// ControlPlaneWebhookPath is the HTTP handler path for this admission webhook handler for ControlPlane.
	ControlPlaneWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-controlplane"
	// DNSRecordWebhookPath is the HTTP handler path for this admission webhook handler DNSRecord.
	DNSRecordWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-dnsrecord"
	// EtcdWebhookPath is the HTTP handler path for this admission webhook handler for Etcd.
	EtcdWebhookPath = "/validate-druid-gardener-cloud-v1alpha1-etcd"
	// ExtensionWebhookPath is the HTTP handler path for this admission webhook handler for Extension.
	ExtensionWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-extension"
	// InfrastructureWebhookPath is the HTTP handler path for this admission webhook handler Infrastructure.
	InfrastructureWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-infrastructure"
	// NetworkWebhookPath is the HTTP handler path for this admission webhook handler for Network.
	NetworkWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-network"
	// OperatingSystemConfigWebhookPath is the HTTP handler path for this admission webhook handler for OperatingSystemConfig.
	OperatingSystemConfigWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-operatingsystemconfig"
	// WorkerWebhookPath is the HTTP handler path for this admission webhook handler for Worker.
	WorkerWebhookPath = "/validate-extensions-gardener-cloud-v1alpha1-worker"
)

// AddWebhooks add extension's valdation webhook to manager
func AddWebhooks(mgr manager.Manager) error {
	for obj, validator := range validators {
		if err := builder.WebhookManagedBy(mgr).WithValidator(validator).For(obj).Complete(); err != nil {
			return err
		}
	}

	return nil
}

var (
	_ admission.CustomValidator = &backupBucketValidator{}
	_ admission.CustomValidator = &backupEntryValidator{}
	_ admission.CustomValidator = &bastionValidator{}
	_ admission.CustomValidator = &containerRuntimeValidator{}
	_ admission.CustomValidator = &controlPlaneValidator{}
	_ admission.CustomValidator = &dnsRecordValidator{}
	_ admission.CustomValidator = &etcdValidator{}
	_ admission.CustomValidator = &extensionValidator{}
	_ admission.CustomValidator = &infrastructureValidator{}
	_ admission.CustomValidator = &networkValidator{}
	_ admission.CustomValidator = &operatingSystemConfigValidator{}
	_ admission.CustomValidator = &workerValidator{}

	validators = map[client.Object]admission.CustomValidator{
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
	}
)

type (
	backupBucketValidator          struct{}
	backupEntryValidator           struct{}
	bastionValidator               struct{}
	containerRuntimeValidator      struct{}
	controlPlaneValidator          struct{}
	dnsRecordValidator             struct{}
	etcdValidator                  struct{}
	extensionValidator             struct{}
	infrastructureValidator        struct{}
	networkValidator               struct{}
	operatingSystemConfigValidator struct{}
	workerValidator                struct{}
)

func (*backupBucketValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateBackupBucket(obj.(*extensionsv1alpha1.BackupBucket))
	return errors.ToAggregate()
}

func (*backupBucketValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateBackupBucketUpdate(newObj.(*extensionsv1alpha1.BackupBucket), oldObj.(*extensionsv1alpha1.BackupBucket))
	return errors.ToAggregate()
}

func (*backupBucketValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*backupEntryValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateBackupEntry(obj.(*extensionsv1alpha1.BackupEntry))
	return errors.ToAggregate()
}

func (*backupEntryValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateBackupEntryUpdate(newObj.(*extensionsv1alpha1.BackupEntry), oldObj.(*extensionsv1alpha1.BackupEntry))
	return errors.ToAggregate()
}

func (*backupEntryValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*bastionValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateBastion(obj.(*extensionsv1alpha1.Bastion))
	return errors.ToAggregate()
}

func (*bastionValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateBastionUpdate(newObj.(*extensionsv1alpha1.Bastion), oldObj.(*extensionsv1alpha1.Bastion))
	return errors.ToAggregate()
}

func (*bastionValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*containerRuntimeValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateContainerRuntime(obj.(*extensionsv1alpha1.ContainerRuntime))
	return errors.ToAggregate()
}

func (*containerRuntimeValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateContainerRuntimeUpdate(newObj.(*extensionsv1alpha1.ContainerRuntime), oldObj.(*extensionsv1alpha1.ContainerRuntime))
	return errors.ToAggregate()
}

func (*containerRuntimeValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*controlPlaneValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateControlPlane(obj.(*extensionsv1alpha1.ControlPlane))
	return errors.ToAggregate()
}

func (*controlPlaneValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateControlPlaneUpdate(newObj.(*extensionsv1alpha1.ControlPlane), oldObj.(*extensionsv1alpha1.ControlPlane))
	return errors.ToAggregate()
}

func (*controlPlaneValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*dnsRecordValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateDNSRecord(obj.(*extensionsv1alpha1.DNSRecord))
	return errors.ToAggregate()
}

func (*dnsRecordValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateDNSRecordUpdate(newObj.(*extensionsv1alpha1.DNSRecord), oldObj.(*extensionsv1alpha1.DNSRecord))
	return errors.ToAggregate()
}

func (*dnsRecordValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*etcdValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := druidvalidation.ValidateEtcd(obj.(*druidv1alpha1.Etcd))
	return errors.ToAggregate()
}

func (*etcdValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := druidvalidation.ValidateEtcdUpdate(newObj.(*druidv1alpha1.Etcd), oldObj.(*druidv1alpha1.Etcd))
	return errors.ToAggregate()
}

func (*etcdValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*extensionValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateExtension(obj.(*extensionsv1alpha1.Extension))
	return errors.ToAggregate()
}

func (*extensionValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateExtensionUpdate(newObj.(*extensionsv1alpha1.Extension), oldObj.(*extensionsv1alpha1.Extension))
	return errors.ToAggregate()
}

func (*extensionValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*infrastructureValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateInfrastructure(obj.(*extensionsv1alpha1.Infrastructure))
	return errors.ToAggregate()
}

func (*infrastructureValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateInfrastructureUpdate(newObj.(*extensionsv1alpha1.Infrastructure), oldObj.(*extensionsv1alpha1.Infrastructure))
	return errors.ToAggregate()
}

func (*infrastructureValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*networkValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateNetwork(obj.(*extensionsv1alpha1.Network))
	return errors.ToAggregate()
}

func (*networkValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateNetworkUpdate(newObj.(*extensionsv1alpha1.Network), oldObj.(*extensionsv1alpha1.Network))
	return errors.ToAggregate()
}

func (*networkValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*operatingSystemConfigValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateOperatingSystemConfig(obj.(*extensionsv1alpha1.OperatingSystemConfig))
	return errors.ToAggregate()
}

func (*operatingSystemConfigValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateOperatingSystemConfigUpdate(newObj.(*extensionsv1alpha1.OperatingSystemConfig), oldObj.(*extensionsv1alpha1.OperatingSystemConfig))
	return errors.ToAggregate()
}

func (*operatingSystemConfigValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (*workerValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	errors := validation.ValidateWorker(obj.(*extensionsv1alpha1.Worker))
	return errors.ToAggregate()
}

func (*workerValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	errors := validation.ValidateWorkerUpdate(newObj.(*extensionsv1alpha1.Worker), oldObj.(*extensionsv1alpha1.Worker))
	return errors.ToAggregate()
}

func (*workerValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
