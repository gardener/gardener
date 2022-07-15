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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// AddWebhooks add extension's validation webhook to manager
func AddWebhooks(mgr manager.Manager) error {
	for obj, validator := range validators {
		if err := builder.WebhookManagedBy(mgr).WithValidator(validator).For(obj).Complete(); err != nil {
			return err
		}
	}

	return nil
}

var (
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

func (backupBucketValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.BackupBucket)
	if errs := validation.ValidateBackupBucket(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupBucketResource), object.GetName(), errs)
	}
	return nil
}

func (backupBucketValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.BackupBucket)
	if errs := validation.ValidateBackupBucketUpdate(object, oldObj.(*extensionsv1alpha1.BackupBucket)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupBucketResource), object.GetName(), errs)
	}
	return nil
}

func (backupBucketValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (backupEntryValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.BackupEntry)
	if errs := validation.ValidateBackupEntry(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupEntryResource), object.GetName(), errs)
	}
	return nil
}

func (backupEntryValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.BackupEntry)
	if errs := validation.ValidateBackupEntryUpdate(object, oldObj.(*extensionsv1alpha1.BackupEntry)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupEntryResource), object.GetName(), errs)
	}
	return nil
}

func (backupEntryValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (bastionValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.Bastion)
	if errs := validation.ValidateBastion(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BastionResource), object.GetName(), errs)
	}
	return nil
}

func (bastionValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.Bastion)
	if errs := validation.ValidateBastionUpdate(object, oldObj.(*extensionsv1alpha1.Bastion)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BastionResource), object.GetName(), errs)
	}
	return nil
}

func (bastionValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (containerRuntimeValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.ContainerRuntime)
	if errs := validation.ValidateContainerRuntime(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ContainerRuntimeResource), object.GetName(), errs)
	}
	return nil
}

func (containerRuntimeValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.ContainerRuntime)
	if errs := validation.ValidateContainerRuntimeUpdate(object, oldObj.(*extensionsv1alpha1.ContainerRuntime)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ContainerRuntimeResource), object.GetName(), errs)
	}
	return nil
}

func (containerRuntimeValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (controlPlaneValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.ControlPlane)
	if errs := validation.ValidateControlPlane(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ControlPlaneResource), object.GetName(), errs)
	}
	return nil
}

func (controlPlaneValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.ControlPlane)
	if errs := validation.ValidateControlPlaneUpdate(object, oldObj.(*extensionsv1alpha1.ControlPlane)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ControlPlaneResource), object.GetName(), errs)
	}
	return nil
}

func (controlPlaneValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (dnsRecordValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.DNSRecord)
	if errs := validation.ValidateDNSRecord(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.DNSRecordResource), object.GetName(), errs)
	}
	return nil
}

func (dnsRecordValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.DNSRecord)
	if errs := validation.ValidateDNSRecordUpdate(object, oldObj.(*extensionsv1alpha1.DNSRecord)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.DNSRecordResource), object.GetName(), errs)
	}
	return nil
}

func (dnsRecordValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (etcdValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*druidv1alpha1.Etcd)
	if errs := druidvalidation.ValidateEtcd(object); len(errs) > 0 {
		return apierrors.NewInvalid(object.GroupVersionKind().GroupKind(), object.GetName(), errs)
	}
	return nil
}

func (etcdValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*druidv1alpha1.Etcd)
	if errs := druidvalidation.ValidateEtcdUpdate(object, oldObj.(*druidv1alpha1.Etcd)); len(errs) > 0 {
		return apierrors.NewInvalid(object.GroupVersionKind().GroupKind(), object.GetName(), errs)
	}
	return nil
}

func (etcdValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (extensionValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.Extension)
	if errs := validation.ValidateExtension(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ExtensionResource), object.GetName(), errs)
	}
	return nil
}

func (extensionValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.Extension)
	if errs := validation.ValidateExtensionUpdate(object, oldObj.(*extensionsv1alpha1.Extension)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ExtensionResource), object.GetName(), errs)
	}
	return nil
}

func (extensionValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (infrastructureValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.Infrastructure)
	if errs := validation.ValidateInfrastructure(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.InfrastructureResource), object.GetName(), errs)
	}
	return nil
}

func (infrastructureValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.Infrastructure)
	if errs := validation.ValidateInfrastructureUpdate(object, oldObj.(*extensionsv1alpha1.Infrastructure)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.InfrastructureResource), object.GetName(), errs)
	}
	return nil
}

func (infrastructureValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (networkValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.Network)
	if errs := validation.ValidateNetwork(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.NetworkResource), object.GetName(), errs)
	}
	return nil
}

func (networkValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.Network)
	if errs := validation.ValidateNetworkUpdate(object, oldObj.(*extensionsv1alpha1.Network)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.NetworkResource), object.GetName(), errs)
	}
	return nil
}

func (networkValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (operatingSystemConfigValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.OperatingSystemConfig)
	if errs := validation.ValidateOperatingSystemConfig(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.OperatingSystemConfigResource), object.GetName(), errs)
	}
	return nil
}

func (operatingSystemConfigValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.OperatingSystemConfig)
	if errs := validation.ValidateOperatingSystemConfigUpdate(object, oldObj.(*extensionsv1alpha1.OperatingSystemConfig)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.OperatingSystemConfigResource), object.GetName(), errs)
	}
	return nil
}

func (operatingSystemConfigValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (workerValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	object := obj.(*extensionsv1alpha1.Worker)
	if errs := validation.ValidateWorker(object); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.WorkerResource), object.GetName(), errs)
	}
	return nil
}

func (workerValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	object := newObj.(*extensionsv1alpha1.Worker)
	if errs := validation.ValidateWorkerUpdate(object, oldObj.(*extensionsv1alpha1.Worker)); len(errs) > 0 {
		return apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.WorkerResource), object.GetName(), errs)
	}
	return nil
}

func (workerValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
