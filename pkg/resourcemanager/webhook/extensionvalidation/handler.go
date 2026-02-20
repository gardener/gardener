// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionvalidation

import (
	"context"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	druidvalidation "github.com/gardener/etcd-druid/api/validation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/extensions/validation"
)

type (
	backupBucketValidator            struct{}
	backupEntryValidator             struct{}
	bastionValidator                 struct{}
	containerRuntimeValidator        struct{}
	controlPlaneValidator            struct{}
	dnsRecordValidator               struct{}
	etcdValidator                    struct{}
	extensionValidator               struct{}
	infrastructureValidator          struct{}
	networkValidator                 struct{}
	operatingSystemConfigValidator   struct{}
	selfHostedShootExposureValidator struct{}
	workerValidator                  struct{}
)

func (backupBucketValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.BackupBucket) (admission.Warnings, error) {
	if errs := validation.ValidateBackupBucket(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupBucketResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (backupBucketValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.BackupBucket) (admission.Warnings, error) {
	if errs := validation.ValidateBackupBucketUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupBucketResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (backupBucketValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.BackupBucket) (admission.Warnings, error) {
	return nil, nil
}

func (backupEntryValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.BackupEntry) (admission.Warnings, error) {
	if errs := validation.ValidateBackupEntry(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupEntryResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (backupEntryValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.BackupEntry) (admission.Warnings, error) {
	if errs := validation.ValidateBackupEntryUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupEntryResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (backupEntryValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.BackupEntry) (admission.Warnings, error) {
	return nil, nil
}

func (bastionValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.Bastion) (admission.Warnings, error) {
	if errs := validation.ValidateBastion(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BastionResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (bastionValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.Bastion) (admission.Warnings, error) {
	if errs := validation.ValidateBastionUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BastionResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (bastionValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.Bastion) (admission.Warnings, error) {
	return nil, nil
}

func (containerRuntimeValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.ContainerRuntime) (admission.Warnings, error) {
	if errs := validation.ValidateContainerRuntime(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ContainerRuntimeResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (containerRuntimeValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.ContainerRuntime) (admission.Warnings, error) {
	if errs := validation.ValidateContainerRuntimeUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ContainerRuntimeResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (containerRuntimeValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.ContainerRuntime) (admission.Warnings, error) {
	return nil, nil
}

func (controlPlaneValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.ControlPlane) (admission.Warnings, error) {
	if errs := validation.ValidateControlPlane(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ControlPlaneResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (controlPlaneValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.ControlPlane) (admission.Warnings, error) {
	if errs := validation.ValidateControlPlaneUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ControlPlaneResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (controlPlaneValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.ControlPlane) (admission.Warnings, error) {
	return nil, nil
}

func (dnsRecordValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.DNSRecord) (admission.Warnings, error) {
	if errs := validation.ValidateDNSRecord(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.DNSRecordResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (dnsRecordValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.DNSRecord) (admission.Warnings, error) {
	if errs := validation.ValidateDNSRecordUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.DNSRecordResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (dnsRecordValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.DNSRecord) (admission.Warnings, error) {
	return nil, nil
}

func (etcdValidator) ValidateCreate(_ context.Context, obj *druidcorev1alpha1.Etcd) (admission.Warnings, error) {
	if errs := druidvalidation.ValidateEtcd(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(obj.GroupVersionKind().GroupKind(), obj.GetName(), errs)
	}
	return nil, nil
}

func (etcdValidator) ValidateUpdate(_ context.Context, oldObj, newObj *druidcorev1alpha1.Etcd) (admission.Warnings, error) {
	if errs := druidvalidation.ValidateEtcdUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(newObj.GroupVersionKind().GroupKind(), newObj.GetName(), errs)
	}
	return nil, nil
}

func (etcdValidator) ValidateDelete(_ context.Context, _ *druidcorev1alpha1.Etcd) (admission.Warnings, error) {
	return nil, nil
}

func (extensionValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.Extension) (admission.Warnings, error) {
	if errs := validation.ValidateExtension(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ExtensionResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (extensionValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.Extension) (admission.Warnings, error) {
	if errs := validation.ValidateExtensionUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ExtensionResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (extensionValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.Extension) (admission.Warnings, error) {
	return nil, nil
}

func (infrastructureValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.Infrastructure) (admission.Warnings, error) {
	if errs := validation.ValidateInfrastructure(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.InfrastructureResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (infrastructureValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.Infrastructure) (admission.Warnings, error) {
	if errs := validation.ValidateInfrastructureUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.InfrastructureResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (infrastructureValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.Infrastructure) (admission.Warnings, error) {
	return nil, nil
}

func (selfHostedShootExposureValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.SelfHostedShootExposure) (admission.Warnings, error) {
	if errs := validation.ValidateSelfHostedShootExposure(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.SelfHostedShootExposureResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (selfHostedShootExposureValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.SelfHostedShootExposure) (admission.Warnings, error) {
	if errs := validation.ValidateSelfHostedShootExposureUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.SelfHostedShootExposureResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (selfHostedShootExposureValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.SelfHostedShootExposure) (admission.Warnings, error) {
	return nil, nil
}

func (networkValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.Network) (admission.Warnings, error) {
	if errs := validation.ValidateNetwork(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.NetworkResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (networkValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.Network) (admission.Warnings, error) {
	if errs := validation.ValidateNetworkUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.NetworkResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (networkValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.Network) (admission.Warnings, error) {
	return nil, nil
}

func (operatingSystemConfigValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.OperatingSystemConfig) (admission.Warnings, error) {
	if errs := validation.ValidateOperatingSystemConfig(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.OperatingSystemConfigResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (operatingSystemConfigValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.OperatingSystemConfig) (admission.Warnings, error) {
	if errs := validation.ValidateOperatingSystemConfigUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.OperatingSystemConfigResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (operatingSystemConfigValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.OperatingSystemConfig) (admission.Warnings, error) {
	return nil, nil
}

func (workerValidator) ValidateCreate(_ context.Context, obj *extensionsv1alpha1.Worker) (admission.Warnings, error) {
	if errs := validation.ValidateWorker(obj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.WorkerResource), obj.GetName(), errs)
	}
	return nil, nil
}

func (workerValidator) ValidateUpdate(_ context.Context, oldObj, newObj *extensionsv1alpha1.Worker) (admission.Warnings, error) {
	if errs := validation.ValidateWorkerUpdate(newObj, oldObj); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.WorkerResource), newObj.GetName(), errs)
	}
	return nil, nil
}

func (workerValidator) ValidateDelete(_ context.Context, _ *extensionsv1alpha1.Worker) (admission.Warnings, error) {
	return nil, nil
}
