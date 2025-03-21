// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionvalidation

import (
	"context"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	druidvalidation "github.com/gardener/etcd-druid/api/validation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/extensions/validation"
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

func (backupBucketValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.BackupBucket)
	if errs := validation.ValidateBackupBucket(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupBucketResource), object.GetName(), errs)
	}
	return nil, nil
}

func (backupBucketValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.BackupBucket)
	if errs := validation.ValidateBackupBucketUpdate(object, oldObj.(*extensionsv1alpha1.BackupBucket)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupBucketResource), object.GetName(), errs)
	}
	return nil, nil
}

func (backupBucketValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (backupEntryValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.BackupEntry)
	if errs := validation.ValidateBackupEntry(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupEntryResource), object.GetName(), errs)
	}
	return nil, nil
}

func (backupEntryValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.BackupEntry)
	if errs := validation.ValidateBackupEntryUpdate(object, oldObj.(*extensionsv1alpha1.BackupEntry)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BackupEntryResource), object.GetName(), errs)
	}
	return nil, nil
}

func (backupEntryValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (bastionValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.Bastion)
	if errs := validation.ValidateBastion(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BastionResource), object.GetName(), errs)
	}
	return nil, nil
}

func (bastionValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.Bastion)
	if errs := validation.ValidateBastionUpdate(object, oldObj.(*extensionsv1alpha1.Bastion)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.BastionResource), object.GetName(), errs)
	}
	return nil, nil
}

func (bastionValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (containerRuntimeValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.ContainerRuntime)
	if errs := validation.ValidateContainerRuntime(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ContainerRuntimeResource), object.GetName(), errs)
	}
	return nil, nil
}

func (containerRuntimeValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.ContainerRuntime)
	if errs := validation.ValidateContainerRuntimeUpdate(object, oldObj.(*extensionsv1alpha1.ContainerRuntime)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ContainerRuntimeResource), object.GetName(), errs)
	}
	return nil, nil
}

func (containerRuntimeValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (controlPlaneValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.ControlPlane)
	if errs := validation.ValidateControlPlane(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ControlPlaneResource), object.GetName(), errs)
	}
	return nil, nil
}

func (controlPlaneValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.ControlPlane)
	if errs := validation.ValidateControlPlaneUpdate(object, oldObj.(*extensionsv1alpha1.ControlPlane)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ControlPlaneResource), object.GetName(), errs)
	}
	return nil, nil
}

func (controlPlaneValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (dnsRecordValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.DNSRecord)
	if errs := validation.ValidateDNSRecord(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.DNSRecordResource), object.GetName(), errs)
	}
	return nil, nil
}

func (dnsRecordValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.DNSRecord)
	if errs := validation.ValidateDNSRecordUpdate(object, oldObj.(*extensionsv1alpha1.DNSRecord)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.DNSRecordResource), object.GetName(), errs)
	}
	return nil, nil
}

func (dnsRecordValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (etcdValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*druidcorev1alpha1.Etcd)
	if errs := druidvalidation.ValidateEtcd(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(object.GroupVersionKind().GroupKind(), object.GetName(), errs)
	}
	return nil, nil
}

func (etcdValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*druidcorev1alpha1.Etcd)
	if errs := druidvalidation.ValidateEtcdUpdate(object, oldObj.(*druidcorev1alpha1.Etcd)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(object.GroupVersionKind().GroupKind(), object.GetName(), errs)
	}
	return nil, nil
}

func (etcdValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (extensionValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.Extension)
	if errs := validation.ValidateExtension(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ExtensionResource), object.GetName(), errs)
	}
	return nil, nil
}

func (extensionValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.Extension)
	if errs := validation.ValidateExtensionUpdate(object, oldObj.(*extensionsv1alpha1.Extension)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.ExtensionResource), object.GetName(), errs)
	}
	return nil, nil
}

func (extensionValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (infrastructureValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.Infrastructure)
	if errs := validation.ValidateInfrastructure(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.InfrastructureResource), object.GetName(), errs)
	}
	return nil, nil
}

func (infrastructureValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.Infrastructure)
	if errs := validation.ValidateInfrastructureUpdate(object, oldObj.(*extensionsv1alpha1.Infrastructure)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.InfrastructureResource), object.GetName(), errs)
	}
	return nil, nil
}

func (infrastructureValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (networkValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.Network)
	if errs := validation.ValidateNetwork(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.NetworkResource), object.GetName(), errs)
	}
	return nil, nil
}

func (networkValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.Network)
	if errs := validation.ValidateNetworkUpdate(object, oldObj.(*extensionsv1alpha1.Network)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.NetworkResource), object.GetName(), errs)
	}
	return nil, nil
}

func (networkValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (operatingSystemConfigValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.OperatingSystemConfig)
	if errs := validation.ValidateOperatingSystemConfig(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.OperatingSystemConfigResource), object.GetName(), errs)
	}
	return nil, nil
}

func (operatingSystemConfigValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.OperatingSystemConfig)
	if errs := validation.ValidateOperatingSystemConfigUpdate(object, oldObj.(*extensionsv1alpha1.OperatingSystemConfig)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.OperatingSystemConfigResource), object.GetName(), errs)
	}
	return nil, nil
}

func (operatingSystemConfigValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (workerValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	object := obj.(*extensionsv1alpha1.Worker)
	if errs := validation.ValidateWorker(object); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.WorkerResource), object.GetName(), errs)
	}
	return nil, nil
}

func (workerValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	object := newObj.(*extensionsv1alpha1.Worker)
	if errs := validation.ValidateWorkerUpdate(object, oldObj.(*extensionsv1alpha1.Worker)); len(errs) > 0 {
		return nil, apierrors.NewInvalid(extensionsv1alpha1.Kind(extensionsv1alpha1.WorkerResource), object.GetName(), errs)
	}
	return nil, nil
}

func (workerValidator) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
