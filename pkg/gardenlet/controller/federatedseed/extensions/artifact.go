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
	"context"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
)

// controllerArtifacts bundles a list of artifacts for extension kinds
// which are required for state and ControllerInstallation processing.
type controllerArtifacts struct {
	stateArtifacts                  map[string]*artifact
	controllerInstallationArtifacts map[string]*artifact
	hasSyncedFuncs                  []cache.InformerSynced
	shutDownFuncs                   []func()
}

type predicateFn func(newObj, oldObj interface{}) bool

// artifact is specified for extension kinds.
// It servers as a helper to setup the corresponding reconciliation function.
type artifact struct {
	gvk               schema.GroupVersionKind
	newFunc           func() runtime.Object
	informer          runtimecache.Informer
	queue             workqueue.RateLimitingInterface
	predicate         predicateFn
	addEventHandlerFn func()
}

// newControllerArtifacts creates a new controllerArtifacts instance with the necessary artifacts
// for state and ControllerInstallation processing.
func newControllerArtifacts() controllerArtifacts {
	a := controllerArtifacts{
		controllerInstallationArtifacts: make(map[string]*artifact),
		stateArtifacts:                  make(map[string]*artifact),
	}

	gvk := dnsv1alpha1.SchemeGroupVersion.WithKind(dnsv1alpha1.DNSProviderKind)
	a.registerExtensionControllerArtifacts(
		newControllerInstallationArtifact(gvk, func() runtime.Object { return &dnsv1alpha1.DNSProviderList{} }, func(newObj, oldObj interface{}) bool {
			var (
				newExtensionObj, ok1 = newObj.(*dnsv1alpha1.DNSProvider)
				oldExtensionObj, ok2 = oldObj.(*dnsv1alpha1.DNSProvider)
			)
			return ok1 && ok2 && oldExtensionObj.Spec.Type != newExtensionObj.Spec.Type
		}),
		disabledArtifact(),
	)

	gvk = extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.BackupBucketResource)
	a.registerExtensionControllerArtifacts(
		newControllerInstallationArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.BackupBucketList{} }, extensionTypeChanged),
		disabledArtifact(),
	)

	gvk = extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.BackupEntryResource)
	a.registerExtensionControllerArtifacts(
		newControllerInstallationArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.BackupEntryList{} }, extensionTypeChanged),
		newStateArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.BackupEntry{} }, extensionStateOrResourcesChanged),
	)

	gvk = extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.ContainerRuntimeResource)
	a.registerExtensionControllerArtifacts(
		newControllerInstallationArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.ContainerRuntimeList{} }, extensionTypeChanged),
		newStateArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.ContainerRuntime{} }, extensionStateOrResourcesChanged),
	)

	gvk = extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.ControlPlaneResource)
	a.registerExtensionControllerArtifacts(newControllerInstallationArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.ControlPlaneList{} }, extensionTypeChanged),
		newStateArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.ControlPlane{} }, extensionStateOrResourcesChanged),
	)

	gvk = extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.ExtensionResource)
	a.registerExtensionControllerArtifacts(
		newControllerInstallationArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.ExtensionList{} }, extensionTypeChanged),
		newStateArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.Extension{} }, extensionStateOrResourcesChanged),
	)

	gvk = extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.InfrastructureResource)
	a.registerExtensionControllerArtifacts(
		newControllerInstallationArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.InfrastructureList{} }, extensionTypeChanged),
		newStateArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.Infrastructure{} }, extensionStateOrResourcesChanged),
	)

	gvk = extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.NetworkResource)
	a.registerExtensionControllerArtifacts(
		newControllerInstallationArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.NetworkList{} }, extensionTypeChanged),
		newStateArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.Network{} }, extensionStateOrResourcesChanged),
	)

	gvk = extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.OperatingSystemConfigResource)
	a.registerExtensionControllerArtifacts(
		newControllerInstallationArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.OperatingSystemConfigList{} }, extensionTypeChanged),
		newStateArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.OperatingSystemConfig{} }, extensionStateOrResourcesChanged),
	)

	gvk = extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.WorkerResource)
	a.registerExtensionControllerArtifacts(
		newControllerInstallationArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.WorkerList{} }, extensionTypeChanged),
		newStateArtifact(gvk, func() runtime.Object { return &extensionsv1alpha1.Worker{} }, extensionStateOrResourcesChanged),
	)

	return a
}

func (c *controllerArtifacts) registerExtensionControllerArtifacts(controllerInstallation, state *artifact) {
	if controllerInstallation != nil {
		c.controllerInstallationArtifacts[controllerInstallation.gvk.Kind] = controllerInstallation
	}
	if state != nil {
		c.stateArtifacts[state.gvk.Kind] = state
	}
}

// initialize obtains the informers for the enclosing artifacts.
func (c *controllerArtifacts) initialize(ctx context.Context, seedClient kubernetes.Interface) error {
	initialize := func(a *artifact) error {
		informer, err := seedClient.Cache().GetInformerForKind(ctx, a.gvk)
		if err != nil {
			return err
		}
		a.informer = informer
		c.hasSyncedFuncs = append(c.hasSyncedFuncs, informer.HasSynced)
		c.shutDownFuncs = append(c.shutDownFuncs, a.queue.ShutDown)
		a.addEventHandlerFn()
		return nil
	}

	for _, artifact := range c.controllerInstallationArtifacts {
		if err := initialize(artifact); err != nil {
			return err
		}
	}

	for _, artifact := range c.stateArtifacts {
		if err := initialize(artifact); err != nil {
			return err
		}
	}

	return nil
}

func (c *controllerArtifacts) shutdownQueues() {
	for _, shutdown := range c.shutDownFuncs {
		shutdown()
	}
}

func newControllerInstallationArtifact(gvk schema.GroupVersionKind, newObjFunc func() runtime.Object, fn predicateFn) *artifact {
	a := &artifact{
		gvk:       gvk,
		newFunc:   newObjFunc,
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("controllerinstallation-extension-%s", gvk.Kind)),
		predicate: fn,
	}

	a.addEventHandlerFn = func() {
		a.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    createEnqueueFunc(a.queue),
			UpdateFunc: createEnqueueOnUpdateFunc(a.queue, a.predicate),
			DeleteFunc: createEnqueueFunc(a.queue),
		})
	}

	return a
}

func newStateArtifact(gvk schema.GroupVersionKind, newObjFunc func() runtime.Object, fn predicateFn) *artifact {
	a := &artifact{
		gvk:       gvk,
		newFunc:   newObjFunc,
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("shootstate-%s", gvk.Kind)),
		predicate: fn,
	}

	a.addEventHandlerFn = func() {
		a.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    createEnqueueFunc(a.queue),
			UpdateFunc: createEnqueueOnUpdateFunc(a.queue, a.predicate),
		})
	}

	return a
}

func disabledArtifact() *artifact {
	return nil
}
