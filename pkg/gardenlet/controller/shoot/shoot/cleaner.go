// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shoot

import (
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

var (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second
	// DefaultTimeout is the default timeout for waiting for resources to be migrated or deleted.
	DefaultTimeout = 5 * time.Minute
)

var (
	extensionKindToObjectList = map[string]client.ObjectList{
		extensionsv1alpha1.BastionResource:               &extensionsv1alpha1.BastionList{},
		extensionsv1alpha1.ContainerRuntimeResource:      &extensionsv1alpha1.ContainerRuntimeList{},
		extensionsv1alpha1.ControlPlaneResource:          &extensionsv1alpha1.ControlPlaneList{},
		extensionsv1alpha1.DNSRecordResource:             &extensionsv1alpha1.DNSRecordList{},
		extensionsv1alpha1.ExtensionResource:             &extensionsv1alpha1.ExtensionList{},
		extensionsv1alpha1.InfrastructureResource:        &extensionsv1alpha1.InfrastructureList{},
		extensionsv1alpha1.NetworkResource:               &extensionsv1alpha1.NetworkList{},
		extensionsv1alpha1.OperatingSystemConfigResource: &extensionsv1alpha1.OperatingSystemConfigList{},
		extensionsv1alpha1.WorkerResource:                &extensionsv1alpha1.WorkerList{},
	}

	machineKindToObjectList = map[string]client.ObjectList{
		"MachineDeployment": &machinev1alpha1.MachineDeploymentList{},
		"MachineSet":        &machinev1alpha1.MachineSetList{},
		"MachineClass":      &machinev1alpha1.MachineClassList{},
		"Machine":           &machinev1alpha1.MachineList{},
	}

	kubernetesKindToObjectList = map[string]client.ObjectList{
		"ConfigMap": &corev1.ConfigMapList{},
		"Secret":    &corev1.SecretList{},
	}
)

type Cleaner struct {
	seedClient    client.Client
	gardenClient  client.Client
	seedNamespace string
	log           logr.Logger
}

// NewCleaner creates a Cleaner with the given clients and logger, for a shoot with the given namespace.
func NewCleaner(log logr.Logger, seedClient, gardenClient client.Client, seedNamespace string) *Cleaner {
	return &Cleaner{
		seedClient:    seedClient,
		gardenClient:  gardenClient,
		seedNamespace: seedNamespace,
		log:           log,
	}
}

// DeleteExtensionObjects deletes all extension objects in the shoot namespace.
func (c *Cleaner) DeleteExtensionObjects(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			c.log.Info("Deleting all extension resources", "kind", kind, "namespace", c.seedNamespace)
			return extensions.DeleteExtensionObjects(ctx, c.seedClient, objectList, c.seedNamespace, nil)
		}
	}, extensionKindToObjectList)
}

// WaitUntilExtensionObjectsDeleted waits until all extension objects in the shoot namespace have been deleted.
func (c *Cleaner) WaitUntilExtensionObjectsDeleted(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			return extensions.WaitUntilExtensionObjectsDeleted(ctx, c.seedClient, c.log, objectList, kind, c.seedNamespace, DefaultInterval, DefaultTimeout, nil)
		}
	}, extensionKindToObjectList)
}

// DeleteMachineResources deletes all MachineControllerManager resources in the shoot namespace.
func (c *Cleaner) DeleteMachineResources(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		c.log.Info("Deleting all machine resources in namespace", "namespace", c.seedNamespace, "kind", kind)
		return utilclient.ForceDeleteObjects(ctx, c.seedClient, kind, c.seedNamespace, objectList)
	}, machineKindToObjectList)
}

// WaitUntilMachineResourcesDeleted waits until all MachineControllerManager resources in the shoot namespace have been deleted.
func (c *Cleaner) WaitUntilMachineResourcesDeleted(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			return kubernetesutils.WaitUntilResourcesDeleted(ctx, c.seedClient, objectList, DefaultInterval, client.InNamespace(c.seedNamespace))
		}
	}, machineKindToObjectList)
}

// SetKeepObjectsForManagedResources sets keepObjects to false for all ManagedResource resources in the shoot namespace.
func (c *Cleaner) SetKeepObjectsForManagedResources(ctx context.Context) error {
	mrList := &resourcesv1alpha1.ManagedResourceList{}
	if err := c.seedClient.List(ctx, mrList, client.InNamespace(c.seedNamespace)); err != nil {
		return err
	}

	return utilclient.ApplyToObjects(ctx, mrList, func(ctx context.Context, object client.Object) error {
		return managedresources.SetKeepObjects(ctx, c.seedClient, object.GetNamespace(), object.GetName(), false)
	})
}

// DeleteManagedResources removes all remaining finalizers and deletes all ManagedResource resources in the shoot namespace.
func (c *Cleaner) DeleteManagedResources(ctx context.Context) error {
	c.log.Info("Deleting all ManagedResource resources in namespace", "namespace", c.seedNamespace)
	if err := c.seedClient.DeleteAllOf(ctx, &resourcesv1alpha1.ManagedResource{}, client.InNamespace(c.seedNamespace)); err != nil {
		return err
	}

	return c.finalizeShootManagedResources(ctx, c.seedNamespace)
}

// WaitUntilManagedResourcesDeleted waits until all ManagedResource resources in the shoot namespace have been deleted.
func (c *Cleaner) WaitUntilManagedResourcesDeleted(ctx context.Context) error {
	return kubernetesutils.WaitUntilResourcesDeleted(ctx, c.seedClient, &resourcesv1alpha1.ManagedResourceList{}, DefaultInterval, client.InNamespace(c.seedNamespace))
}

// DeleteKubernetesResources removes all remaining finalizers and deletes all passed kubernetes resources in the shoot namespace.
func (c *Cleaner) DeleteKubernetesResources(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		c.log.Info("Deleting all resources in namespace", "namespace", c.seedNamespace, "kind", kind)
		return utilclient.ForceDeleteObjects(ctx, c.seedClient, kind, c.seedNamespace, objectList)
	}, kubernetesKindToObjectList)
}

// DeleteCluster deletes the shoot Cluster resource in the seed cluster.
func (c *Cleaner) DeleteCluster(ctx context.Context) error {
	cluster := c.getEmptyCluster()

	c.log.Info("Deleting Cluster resource", "clusterName", cluster.Name)
	return client.IgnoreNotFound(c.seedClient.Delete(ctx, cluster))
}

func (c *Cleaner) getEmptyCluster() *extensionsv1alpha1.Cluster {
	return &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: c.seedNamespace}}
}

func (c *Cleaner) removeFinalizersFromObjects(ctx context.Context, objectList client.ObjectList) error {
	return utilclient.ApplyToObjects(ctx, objectList, func(ctx context.Context, object client.Object) error {
		if len(object.GetFinalizers()) > 0 {
			c.log.Info("Removing finalizers", "kind", object.GetObjectKind().GroupVersionKind().Kind, "object", client.ObjectKeyFromObject(object))
			return controllerutils.RemoveAllFinalizers(ctx, c.seedClient, object)
		}
		return nil
	})
}

func (c *Cleaner) finalizeShootManagedResources(ctx context.Context, namespace string) error {
	mrList := &resourcesv1alpha1.ManagedResourceList{}
	if err := c.seedClient.List(ctx, mrList, client.InNamespace(namespace)); err != nil {
		return err
	}

	shootMRList := &resourcesv1alpha1.ManagedResourceList{}
	for _, mr := range mrList.Items {
		// TODO(rfranzke): Uncomment the next line after v1.85 has been released.
		// if pointer.StringDeref(mr.Spec.Class, "") != resourcesv1alpha1.ResourceManagerClassShoot {
		// 	continue
		// }
		if mr.Spec.Class != nil {
			continue
		}

		shootMRList.Items = append(shootMRList.Items, mr)
	}

	return c.removeFinalizersFromObjects(ctx, shootMRList)
}
