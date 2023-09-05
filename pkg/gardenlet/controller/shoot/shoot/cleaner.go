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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// defaultInterval is the default interval for retry operations.
	defaultInterval = 5 * time.Second
	// defaultTimeout is the default timeout for waiting for resources to be migrated or deleted.
	defaultTimeout = 5 * time.Minute
)

var (
	extensionKindToObjectList = map[string]client.ObjectList{
		extensionsv1alpha1.BastionResource:               &extensionsv1alpha1.BastionList{},
		extensionsv1alpha1.ContainerRuntimeResource:      &extensionsv1alpha1.ContainerRuntimeList{},
		extensionsv1alpha1.ExtensionResource:             &extensionsv1alpha1.ExtensionList{},
		extensionsv1alpha1.InfrastructureResource:        &extensionsv1alpha1.InfrastructureList{},
		extensionsv1alpha1.NetworkResource:               &extensionsv1alpha1.NetworkList{},
		extensionsv1alpha1.OperatingSystemConfigResource: &extensionsv1alpha1.OperatingSystemConfigList{},
		extensionsv1alpha1.WorkerResource:                &extensionsv1alpha1.WorkerList{},
	}

	mcmKindToObjectList = map[string]client.ObjectList{
		"MachineDeployment": &machinev1alpha1.MachineDeploymentList{},
		"MachineSet":        &machinev1alpha1.MachineSetList{},
		"MachineClass":      &machinev1alpha1.MachineClassList{},
		"Machine":           &machinev1alpha1.MachineList{},
	}
)

// Cleaner provides methods for deleting and waiting upon deletion of shoot resources in a seed cluster.
type Cleaner interface {
	// DeleteBackupEntry deletes the shoot BackupEntry resource in the garden cluster.
	DeleteBackupEntry(ctx context.Context) error
	// DeleteCluster deletes the shoot Cluster resource in the seed cluster.
	DeleteCluster(ctx context.Context) error
	// DeleteExtensionObjects deletes all extension resources in the shoot namespace.
	DeleteExtensionObjects(ctx context.Context) error
	// DeleteSecrets removes all remaining finalizers and deletes all secrets in the shoot namespace.
	DeleteSecrets(ctx context.Context) error
	// DeleteManagedResources removes all remaining finalizers and deletes all ManagedResource resources in the shoot namespace.
	DeleteManagedResources(ctx context.Context) error
	// DeleteNamespace deletes the shoot namespace in the seed cluster.
	DeleteNamespace(ctx context.Context) error
	// DeleteControlPlanes deletes all ControlPlane resources in the shoot namespace.
	DeleteControlPlanes(ctx context.Context) error
	// DeleteDNSRecords deletes all DNSRecord resources in the shoot namespace.
	DeleteDNSRecords(ctx context.Context) error
	// DeleteMCMResources deletes all MachineControllerManager resources in the shoot namespace.
	DeleteMCMResources(ctx context.Context) error

	// SetKeepObjectsForManagedResources sets keepObjects to false for all ManagedResource resources in the shoot namespace.
	SetKeepObjectsForManagedResources(ctx context.Context) error

	// WaitUntilBackupEntryDeleted waits until the shoot BackupEntry resource in the garden cluster has been deleted.
	WaitUntilBackupEntryDeleted(ctx context.Context) error
	// WaitUntilClusterDeleted waits until the shoot Cluster resource in the seed cluster has been deleted.
	WaitUntilClusterDeleted(ctx context.Context) error
	// WaitUntilExtensionObjectsDeleted waits until all extension resources in the shoot namespace have been deleted.
	WaitUntilExtensionObjectsDeleted(ctx context.Context) error
	// WaitUntilManagedResourcesDeleted waits until all ManagedResource resources in the shoot namespace have been deleted.
	WaitUntilManagedResourcesDeleted(ctx context.Context) error
	// WaitUntilNamespaceDeleted waits until the shoot namespace in the seed cluster has been deleted.
	WaitUntilNamespaceDeleted(ctx context.Context) error
	// WaitUntilControlPlanesDeleted waits until all ControlPlane resources in the shoot namespace have been deleted.
	WaitUntilControlPlanesDeleted(ctx context.Context) error
	// WaitUntilDNSRecordsDeleted waits until all DNSRecord resources in the shoot namespace have been deleted.
	WaitUntilDNSRecordsDeleted(ctx context.Context) error
	// WaitUntilMCMResourcesDeleted waits until all MachineControllerManager resources in the shoot namespace have been deleted.
	WaitUntilMCMResourcesDeleted(ctx context.Context) error
}

type cleaner struct {
	seedClient       client.Client
	gardenClient     client.Client
	seedNamespace    string
	projectNamespace string
	backupEntryName  string
	log              logr.Logger
}

// NewCleaner creates a Cleaner with the given clients and logger, for a shoot with the given namespaces and backupEntryName.
func NewCleaner(seedClient, gardenClient client.Client, seedNamespace, projectNamespace string, backupEntryName string, log logr.Logger) Cleaner {
	return &cleaner{
		seedClient:       seedClient,
		gardenClient:     gardenClient,
		seedNamespace:    seedNamespace,
		projectNamespace: projectNamespace,
		backupEntryName:  backupEntryName,
		log:              log,
	}
}

// DeleteExtensionObjects deletes all extension objects in the shoot namespace.
func (c *cleaner) DeleteExtensionObjects(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			c.log.Info("Deleting all extension resources", "kind", kind)
			if err := extensions.DeleteExtensionObjects(ctx, c.seedClient, objectList, c.seedNamespace, nil); err != nil {
				return err
			}

			return c.removeFinalizersFromObjects(ctx, c.seedNamespace, objectList)
		}
	}, extensionKindToObjectList)
}

// WaitUntilExtensionObjectsDeleted waits until all extension objects in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilExtensionObjectsDeleted(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			return extensions.WaitUntilExtensionObjectsDeleted(ctx, c.seedClient, c.log, objectList, kind, c.seedNamespace, defaultInterval, defaultTimeout, nil)
		}
	}, extensionKindToObjectList)
}

// DeleteBackupEntry deletes the shoot BackupEntry resource in the garden cluster.
func (c *cleaner) DeleteBackupEntry(ctx context.Context) error {
	c.log.Info("Deleting BackupEntry resource", "backupentry", c.backupEntryName)
	return kubernetesutils.DeleteObject(ctx, c.gardenClient, c.getEmptyBackupEntry())

}

// WaitUntilBackupEntryDeleted waits until the shoot BackupEntry resource in the garden cluster has been deleted.
func (c *cleaner) WaitUntilBackupEntryDeleted(ctx context.Context) error {
	return kubernetesutils.WaitUntilResourceDeleted(ctx, c.gardenClient, c.getEmptyBackupEntry(), defaultInterval)
}

// DeleteControlPlanes deletes all ControlPlane resources in the shoot namespace.
func (c *cleaner) DeleteControlPlanes(ctx context.Context) error {
	c.log.Info("Deleting all ControlPlane resources in namespace", "namespace", c.seedNamespace)
	return extensions.DeleteExtensionObjects(ctx, c.seedClient, &extensionsv1alpha1.ControlPlaneList{}, c.seedNamespace, nil)
}

// WaitUntilControlPlanesDeleted waits until all ControlPlane resources in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilControlPlanesDeleted(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectsDeleted(ctx, c.seedClient, c.log, &extensionsv1alpha1.ControlPlaneList{}, extensionsv1alpha1.ControlPlaneResource, c.seedNamespace, defaultInterval, defaultTimeout, nil)
}

// DeleteDNSRecords deletes all DNSRecord resources in the shoot namespace.
func (c *cleaner) DeleteDNSRecords(ctx context.Context) error {
	c.log.Info("Deleting all DNSRecord resources in namespace", "namespace", c.seedNamespace)
	return extensions.DeleteExtensionObjects(ctx, c.seedClient, &extensionsv1alpha1.DNSRecordList{}, c.seedNamespace, nil)
}

// WaitUntilDNSRecordsDeleted waits until all DNSRecord resources in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilDNSRecordsDeleted(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectsDeleted(ctx, c.seedClient, c.log, &extensionsv1alpha1.DNSRecordList{}, extensionsv1alpha1.DNSRecordResource, c.seedNamespace, defaultInterval, defaultTimeout, nil)
}

// DeleteMCMResources deletes all MachineControllerManager resources in the shoot namespace.
func (c *cleaner) DeleteMCMResources(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return utilclient.ForceDeleteObjects(ctx, c.log, c.seedClient, kind, c.seedNamespace, objectList)
	}, mcmKindToObjectList)
}

// WaitUntilMCMResourcesDeleted waits until all MachineControllerManager resources in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilMCMResourcesDeleted(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			return kubernetesutils.WaitUntilResourcesDeleted(ctx, c.seedClient, objectList, defaultInterval, client.InNamespace(c.seedNamespace))
		}
	}, mcmKindToObjectList)
}

// SetKeepObjectsForManagedResources sets keepObjects to false for all ManagedResource resources in the shoot namespace.
func (c *cleaner) SetKeepObjectsForManagedResources(ctx context.Context) error {
	return c.applyToObjects(ctx, c.seedNamespace, &resourcesv1alpha1.ManagedResourceList{}, func(ctx context.Context, object client.Object) error {
		c.log.Info("Setting keepObjects to false for ManagedResource", "managedresource", client.ObjectKeyFromObject(object))
		return managedresources.SetKeepObjects(ctx, c.seedClient, object.GetNamespace(), object.GetName(), false)
	})
}

// DeleteManagedResources removes all remaining finalizers and deletes all ManagedResource resources in the shoot namespace.
func (c *cleaner) DeleteManagedResources(ctx context.Context) error {
	c.log.Info("Deleting all ManagedResource resources in namespace", "namespace", c.seedNamespace)
	if err := c.seedClient.DeleteAllOf(ctx, &resourcesv1alpha1.ManagedResource{}, client.InNamespace(c.seedNamespace)); err != nil {
		return err
	}

	return c.finalizeShootManagedResources(ctx, c.seedNamespace)
}

// WaitUntilManagedResourcesDeleted waits until all ManagedResource resources in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilManagedResourcesDeleted(ctx context.Context) error {
	return kubernetesutils.WaitUntilResourcesDeleted(ctx, c.seedClient, &resourcesv1alpha1.ManagedResourceList{}, defaultInterval, client.InNamespace(c.seedNamespace))
}

// DeleteSecrets removes all remaining finalizers and deletes all secrets in the shoot namespace.
func (c *cleaner) DeleteSecrets(ctx context.Context) error {
	c.log.Info("Deleting all secrets in namespace", "namespace", c.seedNamespace)
	if err := c.seedClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(c.seedNamespace)); client.IgnoreNotFound(err) != nil {
		return err
	}
	return c.removeFinalizersFromObjects(ctx, c.seedNamespace, &corev1.SecretList{})
}

// DeleteNamespace deletes the shoot namespace in the seed cluster.
func (c *cleaner) DeleteNamespace(ctx context.Context) error {
	c.log.Info("Deleting namespace", "namespace", c.seedNamespace)
	return client.IgnoreNotFound(c.seedClient.Delete(ctx, c.getEmptyNamespace()))
}

// WaitUntilNamespaceDeleted waits until the shoot namespace in the seed cluster has been deleted.
func (c *cleaner) WaitUntilNamespaceDeleted(ctx context.Context) error {
	return kubernetesutils.WaitUntilResourceDeleted(ctx, c.seedClient, c.getEmptyNamespace(), defaultInterval)
}

// DeleteCluster deletes the shoot Cluster resource in the seed cluster.
func (c *cleaner) DeleteCluster(ctx context.Context) error {
	cluster := c.getEmptyCluster()

	c.log.Info("Deleting Cluster resource", "cluster", cluster.Name)
	if err := client.IgnoreNotFound(c.seedClient.Delete(ctx, cluster)); err != nil {
		return err
	}

	return controllerutils.RemoveAllFinalizers(ctx, c.seedClient, cluster)
}

// WaitUntilClusterDeleted waits until the shoot Cluster resource in the seed cluster has been deleted.
func (c *cleaner) WaitUntilClusterDeleted(ctx context.Context) error {
	return kubernetesutils.WaitUntilResourceDeleted(ctx, c.seedClient, c.getEmptyCluster(), defaultInterval)
}

func (c *cleaner) getEmptyNamespace() *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: c.seedNamespace}}
}

func (c *cleaner) getEmptyCluster() *extensionsv1alpha1.Cluster {
	return &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: c.seedNamespace}}
}

func (c *cleaner) getEmptyBackupEntry() *gardencorev1beta1.BackupEntry {
	return &gardencorev1beta1.BackupEntry{ObjectMeta: metav1.ObjectMeta{Name: c.backupEntryName, Namespace: c.projectNamespace}}
}

func (c *cleaner) removeFinalizersFromObjects(ctx context.Context, namespace string, objectList client.ObjectList) error {
	return c.applyToObjects(ctx, namespace, objectList, func(ctx context.Context, object client.Object) error {
		if len(object.GetFinalizers()) > 0 {
			c.log.Info("Removing finalizers", "kind", object.GetObjectKind().GroupVersionKind().Kind, "object", client.ObjectKeyFromObject(object))
			return controllerutils.RemoveAllFinalizers(ctx, c.seedClient, object)
		}
		return nil
	})
}

func (c *cleaner) applyToObjects(ctx context.Context, namespace string, objectList client.ObjectList, fn func(ctx context.Context, object client.Object) error) error {
	if err := c.seedClient.List(ctx, objectList, client.InNamespace(namespace)); err != nil {
		return err
	}
	return utilclient.ApplyToObjects(ctx, objectList, fn)
}

func (c *cleaner) finalizeShootManagedResources(ctx context.Context, namespace string) error {
	mrList := &resourcesv1alpha1.ManagedResourceList{}
	if err := c.seedClient.List(ctx, mrList, client.InNamespace(namespace)); err != nil {
		return err
	}

	shootMRList := &resourcesv1alpha1.ManagedResourceList{}
	for _, mr := range mrList.Items {
		if mr.Spec.Class != nil {
			continue
		}

		shootMRList.Items = append(shootMRList.Items, mr)
	}

	return utilclient.ApplyToObjects(ctx, shootMRList, func(ctx context.Context, object client.Object) error {
		if len(object.GetFinalizers()) > 0 {
			c.log.Info("Removing finalizers from ManagedResource", "object", client.ObjectKeyFromObject(object))
			return controllerutils.RemoveAllFinalizers(ctx, c.seedClient, object)
		}
		return nil
	})
}
