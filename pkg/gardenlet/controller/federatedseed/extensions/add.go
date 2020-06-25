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

package extensions

import (
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/mock/go/context"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type addOptions struct {
	object  runtime.Object
	options []controllerOptions
}

type controllerOptions struct {
	name                string
	concurrentReconcile int
	reconciler          reconcile.Reconciler
	updateFunc          func(event event.UpdateEvent) bool
}

func addToManagerWithOptions(mgr manager.Manager, a addOptions) error {
	for _, opt := range a.options {
		ctrl, err := controller.New(
			opt.name, mgr,
			controller.Options{
				MaxConcurrentReconciles: opt.concurrentReconcile,
				Reconciler:              opt.reconciler,
			})
		if err != nil {
			return err
		}
		if err := ctrl.Watch(&source.Kind{Type: a.object}, &handler.EnqueueRequestForObject{}, predicate.Funcs{
			UpdateFunc: opt.updateFunc,
		}); err != nil {
			return err
		}
	}
	return nil
}

func addToManager(ctx context.Context, mgr manager.Manager, controllerInstallationWorkers, shootStateWorkers int, control controllerInstallationControl, stateControl shootStateControl) error {
	var (
		controllerInstallationName = "controllerinstallation-extension"
		shootStateName             = "shootstate"
	)

	// DNSProvider controller installation controller
	controllerArtifact := addOptions{
		object: &dnsv1alpha1.DNSProvider{},
		options: []controllerOptions{
			{
				name:                fmt.Sprintf("%s-%s", controllerInstallationName, dnsv1alpha1.DNSProviderKind),
				concurrentReconcile: controllerInstallationWorkers,
				reconciler:          control.createExtensionRequiredReconcileFunc(ctx, dnsv1alpha1.DNSProviderKind, func() runtime.Object { return &dnsv1alpha1.DNSProviderList{} }),
				updateFunc:          dnsTypeChanged,
			},
		},
	}
	if err := addToManagerWithOptions(mgr, controllerArtifact); err != nil {
		return err
	}

	// BackupBucket controller installation controller
	controllerArtifact = addOptions{
		object: &extensionsv1alpha1.BackupBucket{},
		options: []controllerOptions{
			{
				name:                fmt.Sprintf("%s-%s", controllerInstallationName, extensionsv1alpha1.BackupBucketResource),
				concurrentReconcile: controllerInstallationWorkers,
				reconciler:          control.createExtensionRequiredReconcileFunc(ctx, extensionsv1alpha1.BackupBucketResource, func() runtime.Object { return &extensionsv1alpha1.BackupBucketList{} }),
				updateFunc:          extensionTypeChanged,
			},
		},
	}
	if err := addToManagerWithOptions(mgr, controllerArtifact); err != nil {
		return err
	}

	// BackupEntry controller installation controller
	controllerArtifact = addOptions{
		object: &extensionsv1alpha1.BackupEntry{},
		options: []controllerOptions{
			{
				name:                fmt.Sprintf("%s-%s", controllerInstallationName, extensionsv1alpha1.BackupEntryResource),
				concurrentReconcile: controllerInstallationWorkers,
				reconciler:          control.createExtensionRequiredReconcileFunc(ctx, extensionsv1alpha1.BackupEntryResource, func() runtime.Object { return &extensionsv1alpha1.BackupEntryList{} }),
				updateFunc:          extensionTypeChanged,
			},
			{
				name:                fmt.Sprintf("%s-%s", shootStateName, extensionsv1alpha1.BackupEntryResource),
				concurrentReconcile: shootStateWorkers,
				reconciler:          stateControl.createShootStateSyncReconcileFunc(ctx, extensionsv1alpha1.BackupEntryResource, func() runtime.Object { return &extensionsv1alpha1.BackupEntryList{} }),
				updateFunc:          extensionStateOrResourcesChanged,
			},
		},
	}
	if err := addToManagerWithOptions(mgr, controllerArtifact); err != nil {
		return err
	}

	// ContainerRuntime controller installation controller
	controllerArtifact = addOptions{
		object: &extensionsv1alpha1.ContainerRuntime{},
		options: []controllerOptions{
			{
				name:                fmt.Sprintf("%s-%s", controllerInstallationName, extensionsv1alpha1.ContainerRuntimeResource),
				concurrentReconcile: controllerInstallationWorkers,
				reconciler:          control.createExtensionRequiredReconcileFunc(ctx, extensionsv1alpha1.ContainerRuntimeResource, func() runtime.Object { return &extensionsv1alpha1.ContainerRuntimeList{} }),
				updateFunc:          extensionTypeChanged,
			},
			{
				name:                fmt.Sprintf("%s-%s", shootStateName, extensionsv1alpha1.ContainerRuntimeResource),
				concurrentReconcile: shootStateWorkers,
				reconciler:          stateControl.createShootStateSyncReconcileFunc(ctx, extensionsv1alpha1.ContainerRuntimeResource, func() runtime.Object { return &extensionsv1alpha1.ContainerRuntimeList{} }),
				updateFunc:          extensionStateOrResourcesChanged,
			},
		},
	}
	if err := addToManagerWithOptions(mgr, controllerArtifact); err != nil {
		return err
	}

	// ControlPlaneResource
	controllerArtifact = addOptions{
		object: &extensionsv1alpha1.ControlPlane{},
		options: []controllerOptions{
			{
				name:                fmt.Sprintf("%s-%s", controllerInstallationName, extensionsv1alpha1.ControlPlaneResource),
				concurrentReconcile: controllerInstallationWorkers,
				reconciler:          control.createExtensionRequiredReconcileFunc(ctx, extensionsv1alpha1.ControlPlaneResource, func() runtime.Object { return &extensionsv1alpha1.ControlPlaneList{} }),
				updateFunc:          extensionTypeChanged,
			},
			{
				name:                fmt.Sprintf("%s-%s", shootStateName, extensionsv1alpha1.ControlPlaneResource),
				concurrentReconcile: shootStateWorkers,
				reconciler:          stateControl.createShootStateSyncReconcileFunc(ctx, extensionsv1alpha1.ControlPlaneResource, func() runtime.Object { return &extensionsv1alpha1.ControlPlaneList{} }),
				updateFunc:          extensionStateOrResourcesChanged,
			},
		},
	}
	if err := addToManagerWithOptions(mgr, controllerArtifact); err != nil {
		return err
	}

	// ExtensionResource
	controllerArtifact = addOptions{
		object: &extensionsv1alpha1.Extension{},
		options: []controllerOptions{
			{
				name:                fmt.Sprintf("%s-%s", controllerInstallationName, extensionsv1alpha1.ExtensionResource),
				concurrentReconcile: controllerInstallationWorkers,
				reconciler:          control.createExtensionRequiredReconcileFunc(ctx, extensionsv1alpha1.ExtensionResource, func() runtime.Object { return &extensionsv1alpha1.ExtensionList{} }),
				updateFunc:          extensionTypeChanged,
			},
			{
				name:                fmt.Sprintf("%s-%s", shootStateName, extensionsv1alpha1.ExtensionResource),
				concurrentReconcile: shootStateWorkers,
				reconciler:          stateControl.createShootStateSyncReconcileFunc(ctx, extensionsv1alpha1.ExtensionResource, func() runtime.Object { return &extensionsv1alpha1.ExtensionList{} }),
				updateFunc:          extensionStateOrResourcesChanged,
			},
		},
	}
	if err := addToManagerWithOptions(mgr, controllerArtifact); err != nil {
		return err
	}

	// InfrastructureResource
	controllerArtifact = addOptions{
		object: &extensionsv1alpha1.Infrastructure{},
		options: []controllerOptions{
			{
				name:                fmt.Sprintf("%s-%s", controllerInstallationName, extensionsv1alpha1.InfrastructureResource),
				concurrentReconcile: controllerInstallationWorkers,
				reconciler:          control.createExtensionRequiredReconcileFunc(ctx, extensionsv1alpha1.InfrastructureResource, func() runtime.Object { return &extensionsv1alpha1.InfrastructureList{} }),
				updateFunc:          extensionTypeChanged,
			},
			{
				name:                fmt.Sprintf("%s-%s", shootStateName, extensionsv1alpha1.InfrastructureResource),
				concurrentReconcile: shootStateWorkers,
				reconciler:          stateControl.createShootStateSyncReconcileFunc(ctx, extensionsv1alpha1.InfrastructureResource, func() runtime.Object { return &extensionsv1alpha1.InfrastructureList{} }),
				updateFunc:          extensionStateOrResourcesChanged,
			},
		},
	}
	if err := addToManagerWithOptions(mgr, controllerArtifact); err != nil {
		return err
	}

	// NetworkResource
	controllerArtifact = addOptions{
		object: &extensionsv1alpha1.Network{},
		options: []controllerOptions{
			{
				name:                fmt.Sprintf("%s-%s", controllerInstallationName, extensionsv1alpha1.NetworkResource),
				concurrentReconcile: controllerInstallationWorkers,
				reconciler:          control.createExtensionRequiredReconcileFunc(ctx, extensionsv1alpha1.NetworkResource, func() runtime.Object { return &extensionsv1alpha1.NetworkList{} }),
				updateFunc:          extensionTypeChanged,
			},
			{
				name:                fmt.Sprintf("%s-%s", shootStateName, extensionsv1alpha1.NetworkResource),
				concurrentReconcile: shootStateWorkers,
				reconciler:          stateControl.createShootStateSyncReconcileFunc(ctx, extensionsv1alpha1.NetworkResource, func() runtime.Object { return &extensionsv1alpha1.NetworkList{} }),
				updateFunc:          extensionStateOrResourcesChanged,
			},
		},
	}
	if err := addToManagerWithOptions(mgr, controllerArtifact); err != nil {
		return err
	}

	// OperatingSystemConfigResource
	controllerArtifact = addOptions{
		object: &extensionsv1alpha1.OperatingSystemConfig{},
		options: []controllerOptions{
			{
				name:                fmt.Sprintf("%s-%s", controllerInstallationName, extensionsv1alpha1.OperatingSystemConfigResource),
				concurrentReconcile: controllerInstallationWorkers,
				reconciler:          control.createExtensionRequiredReconcileFunc(ctx, extensionsv1alpha1.OperatingSystemConfigResource, func() runtime.Object { return &extensionsv1alpha1.OperatingSystemConfigList{} }),
				updateFunc:          extensionTypeChanged,
			},
			{
				name:                fmt.Sprintf("%s-%s", shootStateName, extensionsv1alpha1.OperatingSystemConfigResource),
				concurrentReconcile: shootStateWorkers,
				reconciler:          stateControl.createShootStateSyncReconcileFunc(ctx, extensionsv1alpha1.OperatingSystemConfigResource, func() runtime.Object { return &extensionsv1alpha1.OperatingSystemConfigList{} }),
				updateFunc:          extensionStateOrResourcesChanged,
			},
		},
	}
	if err := addToManagerWithOptions(mgr, controllerArtifact); err != nil {
		return err
	}

	// WorkerResource
	controllerArtifact = addOptions{
		object: &extensionsv1alpha1.Worker{},
		options: []controllerOptions{
			{
				name:                fmt.Sprintf("%s-%s", controllerInstallationName, extensionsv1alpha1.WorkerResource),
				concurrentReconcile: controllerInstallationWorkers,
				reconciler:          control.createExtensionRequiredReconcileFunc(ctx, extensionsv1alpha1.WorkerResource, func() runtime.Object { return &extensionsv1alpha1.WorkerList{} }),
				updateFunc:          extensionTypeChanged,
			},
			{
				name:                fmt.Sprintf("%s-%s", shootStateName, extensionsv1alpha1.WorkerResource),
				concurrentReconcile: shootStateWorkers,
				reconciler:          stateControl.createShootStateSyncReconcileFunc(ctx, extensionsv1alpha1.WorkerResource, func() runtime.Object { return &extensionsv1alpha1.WorkerList{} }),
				updateFunc:          extensionStateOrResourcesChanged,
			},
		},
	}
	if err := addToManagerWithOptions(mgr, controllerArtifact); err != nil {
		return err
	}

	return nil
}
