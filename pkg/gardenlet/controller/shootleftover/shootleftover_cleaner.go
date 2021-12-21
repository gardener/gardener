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

package shootleftover

import (
	"context"
	"fmt"
	"strings"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// defaultInterval is the default interval for retry operations.
	defaultInterval = 5 * time.Second
	// defaultTimeout is the default timeout for waiting for resources to be migrated or deleted.
	defaultTimeout = 5 * time.Minute
)

// Cleaner provides methods for getting, migrating, deleting, and waiting upon the migration and deletion of
// various shoot leftover resources in a seed cluster.
type Cleaner interface {
	// GetNamespace returns the shoot namespace in the seed cluster.
	GetNamespace(ctx context.Context) (*corev1.Namespace, error)
	// GetCluster returns the shoot Cluster resource in the seed cluster.
	GetCluster(ctx context.Context) (*extensionsv1alpha1.Cluster, error)
	// GetBackupEntry returns the shoot BackupEntry resource in the seed cluster.
	GetBackupEntry(ctx context.Context) (*extensionsv1alpha1.BackupEntry, error)
	// GetDNSOwners returns all DNSOwner resources in the seed cluster associated with the shoot.
	GetDNSOwners(ctx context.Context) ([]dnsv1alpha1.DNSOwner, error)

	// MigrateExtensionObjects migrates all extension resources in the shoot namespace.
	MigrateExtensionObjects(ctx context.Context) error
	// DeleteExtensionObjects deletes all extension resources in the shoot namespace.
	DeleteExtensionObjects(ctx context.Context) error
	// MigrateBackupEntry migrates the shoot BackupEntry resource in the seed cluster.
	MigrateBackupEntry(ctx context.Context) error
	// DeleteBackupEntry deletes the shoot BackupEntry resource in the seed cluster.
	DeleteBackupEntry(ctx context.Context) error
	// DeleteEtcds deletes all Etcd resources in the shoot namespace.
	DeleteEtcds(ctx context.Context) error
	// SetKeepObjectsForManagedResources sets keepObjects to true for all ManagedResource resources in the shoot namespace.
	SetKeepObjectsForManagedResources(ctx context.Context) error
	// DeleteManagedResources removes all remaining finalizers and deletes all ManagedResource resources in the shoot namespace.
	DeleteManagedResources(ctx context.Context) error
	// DeleteDNSOwners deletes all DNSOwner resources in the seed cluster associated with the shoot.
	DeleteDNSOwners(ctx context.Context) error
	// DeleteDNSEntries deletes all DNSEntry resources in the shoot namespace.
	DeleteDNSEntries(ctx context.Context) error
	// DeleteDNSProviders deletes all DNSProvider resources in the shoot namespace.
	DeleteDNSProviders(ctx context.Context) error
	// DeleteSecrets removes all remaining finalizers and deletes all secrets in the shoot namespace.
	DeleteSecrets(ctx context.Context) error
	// DeleteNamespace deletes the shoot namespace in the seed cluster.
	DeleteNamespace(ctx context.Context) error
	// DeleteCluster deletes the shoot Cluster resource in the seed cluster.
	DeleteCluster(ctx context.Context) error

	// WaitUntilExtensionObjectsMigrated waits until all extension resources in the shoot namespace have been migrated.
	WaitUntilExtensionObjectsMigrated(ctx context.Context) error
	// WaitUntilExtensionObjectsDeleted waits until all extension resources in the shoot namespace have been deleted.
	WaitUntilExtensionObjectsDeleted(ctx context.Context) error
	// WaitUntilBackupEntryMigrated waits until the shoot BackupEntry resource in the seed cluster has been migrated.
	WaitUntilBackupEntryMigrated(ctx context.Context) error
	// WaitUntilBackupEntryDeleted waits until the shoot BackupEntry resource in the seed cluster has been deleted.
	WaitUntilBackupEntryDeleted(ctx context.Context) error
	// WaitUntilEtcdsDeleted waits until all Etcd resources in the shoot namespace have been deleted.
	WaitUntilEtcdsDeleted(ctx context.Context) error
	// WaitUntilManagedResourcesDeleted waits until all ManagedResource resources in the shoot namespace have been deleted.
	WaitUntilManagedResourcesDeleted(ctx context.Context) error
	// WaitUntilDNSOwnersDeleted waits until all DNSOwner resources in the seed cluster associated with the shoot have been deleted.
	WaitUntilDNSOwnersDeleted(ctx context.Context) error
	// WaitUntilDNSEntriesDeleted waits until all DNSEntry resources in the shoot namespace have been deleted.
	WaitUntilDNSEntriesDeleted(ctx context.Context) error
	// WaitUntilDNSProvidersDeleted waits until all DNSProvider resources in the shoot namespace have been deleted.
	WaitUntilDNSProvidersDeleted(ctx context.Context) error
	// WaitUntilNamespaceDeleted waits until the shoot namespace in the seed cluster has been deleted.
	WaitUntilNamespaceDeleted(ctx context.Context) error
	// WaitUntilClusterDeleted waits until the shoot Cluster resource in the seed cluster has been deleted.
	WaitUntilClusterDeleted(ctx context.Context) error
}

// cleaner is a concrete implementation of Cleaner
type cleaner struct {
	client          client.Client
	namespace       string
	backupEntryName string
	logger          logr.Logger
	// TODO Remove when field loggers are no longer used by the extensions package
	fieldLogger logrus.FieldLogger
}

// newCleaner creates a new Cleaner with the given client and logger, for a shoot with the given technicalID and uid.
func newCleaner(client client.Client, technicalID, uid string, logger logr.Logger, fieldLogger logrus.FieldLogger) Cleaner {
	return &cleaner{
		client:          client,
		namespace:       technicalID,
		backupEntryName: technicalID + "--" + uid,
		logger:          logger,
		fieldLogger:     fieldLogger,
	}
}

// GetNamespace returns the shoot namespace in the seed cluster.
func (c *cleaner) GetNamespace(ctx context.Context) (*corev1.Namespace, error) {
	namespace := c.getEmptyNamespace()
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return namespace, nil
}

// GetCluster returns the shoot Cluster resource in the seed cluster.
func (c *cleaner) GetCluster(ctx context.Context) (*extensionsv1alpha1.Cluster, error) {
	cluster := c.getEmptyCluster()
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(cluster), cluster); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return cluster, nil
}

// GetBackupEntry returns the shoot BackupEntry resource in the seed cluster.
func (c *cleaner) GetBackupEntry(ctx context.Context) (*extensionsv1alpha1.BackupEntry, error) {
	backupEntry := c.getEmptyBackupEntry()
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(backupEntry), backupEntry); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return backupEntry, nil
}

// GetDNSOwners returns all DNSOwner resources in the seed cluster associated with the shoot.
func (c *cleaner) GetDNSOwners(ctx context.Context) ([]dnsv1alpha1.DNSOwner, error) {
	dnsOwnerList := &dnsv1alpha1.DNSOwnerList{}
	if err := c.client.List(ctx, dnsOwnerList); err != nil {
		return nil, err
	}
	var dnsOwners []dnsv1alpha1.DNSOwner
	for _, dnsOwner := range dnsOwnerList.Items {
		if strings.HasPrefix(dnsOwner.Name, c.namespace+"-") {
			dnsOwners = append(dnsOwners, dnsOwner)
		}
	}
	return dnsOwners, nil
}

// MigrateExtensionObjects migrates all extension resources in the shoot namespace.
func (c *cleaner) MigrateExtensionObjects(ctx context.Context) error {
	return applyToExtensionObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			c.logger.Info("Migrating all extension resources", "kind", kind)
			return extensions.MigrateExtensionObjects(ctx, c.client, objectList, c.namespace)
		}
	})
}

// WaitUntilExtensionObjectsMigrated waits until all extension resources in the shoot namespace have been migrated.
func (c *cleaner) WaitUntilExtensionObjectsMigrated(ctx context.Context) error {
	return applyToExtensionObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			return extensions.WaitUntilExtensionObjectsMigrated(ctx, c.client, objectList, c.namespace, defaultInterval, defaultTimeout)
		}
	})
}

// DeleteExtensionObjects deletes all extension objects in the shoot namespace.
func (c *cleaner) DeleteExtensionObjects(ctx context.Context) error {
	return applyToExtensionObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			c.logger.Info("Deleting all extension resources", "kind", kind)
			return extensions.DeleteExtensionObjects(ctx, c.client, objectList, c.namespace, nil)
		}
	})
}

// WaitUntilExtensionObjectsDeleted waits until all extension objects in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilExtensionObjectsDeleted(ctx context.Context) error {
	return applyToExtensionObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			return extensions.WaitUntilExtensionObjectsDeleted(ctx, c.client, c.fieldLogger, objectList, kind, c.namespace, defaultInterval, defaultTimeout, nil)
		}
	})
}

// MigrateBackupEntry migrates the shoot BackupEntry resource.
func (c *cleaner) MigrateBackupEntry(ctx context.Context) error {
	c.logger.Info("Migrating BackupEntry resource", "backupentry", c.backupEntryName)
	return extensions.MigrateExtensionObject(ctx, c.client, c.getEmptyBackupEntry())
}

// WaitUntilBackupEntryMigrated waits until the shoot BackupEntry resource has been migrated.
func (c *cleaner) WaitUntilBackupEntryMigrated(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectMigrated(ctx, c.client, c.getEmptyBackupEntry(), defaultInterval, defaultTimeout)
}

// DeleteBackupEntry deletes the shoot BackupEntry resource in the seed cluster.
func (c *cleaner) DeleteBackupEntry(ctx context.Context) error {
	c.logger.Info("Deleting BackupEntry resource", "backupentry", c.backupEntryName)
	return extensions.DeleteExtensionObject(ctx, c.client, c.getEmptyBackupEntry())
}

// WaitUntilBackupEntryDeleted waits until the shoot BackupEntry resource in the seed cluster has been deleted.
func (c *cleaner) WaitUntilBackupEntryDeleted(ctx context.Context) error {
	return extensions.WaitUntilExtensionObjectDeleted(ctx, c.client, c.fieldLogger, c.getEmptyBackupEntry(), extensionsv1alpha1.BackupEntryResource, defaultInterval, defaultTimeout)
}

// DeleteEtcds deletes all Etcd resources in the shoot namespace.
func (c *cleaner) DeleteEtcds(ctx context.Context) error {
	if err := c.confirmObjectsDeletion(ctx, &druidv1alpha1.EtcdList{}); err != nil {
		return err
	}
	c.logger.Info("Deleting all Etcd resources in namespace", "namespace", c.namespace)
	return c.client.DeleteAllOf(ctx, &druidv1alpha1.Etcd{}, client.InNamespace(c.namespace))
}

// WaitUntilEtcdsDeleted waits until all Etcd resources in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilEtcdsDeleted(ctx context.Context) error {
	return kutil.WaitUntilResourcesDeletedWithTimeout(ctx, c.client, &druidv1alpha1.EtcdList{}, defaultInterval, defaultTimeout, client.InNamespace(c.namespace))
}

// SetKeepObjectsForManagedResources sets keepObjects to true for all ManagedResource resources in the shoot namespace.
func (c *cleaner) SetKeepObjectsForManagedResources(ctx context.Context) error {
	return c.applyToObjects(ctx, &resourcesv1alpha1.ManagedResourceList{}, func(ctx context.Context, object client.Object) error {
		c.logger.Info("Setting keepObjects to true for ManagedResource", "managedresource", client.ObjectKeyFromObject(object))
		return managedresources.SetKeepObjects(ctx, c.client, object.GetNamespace(), object.GetName(), true)
	})
}

// DeleteManagedResources removes all remaining finalizers and deletes all ManagedResource resources in the shoot namespace.
func (c *cleaner) DeleteManagedResources(ctx context.Context) error {
	if err := c.removeFinalizersFromObjects(ctx, &resourcesv1alpha1.ManagedResourceList{}); err != nil {
		return err
	}
	c.logger.Info("Deleting all ManagedResource resources in namespace", "namespace", c.namespace)
	return c.client.DeleteAllOf(ctx, &resourcesv1alpha1.ManagedResource{}, client.InNamespace(c.namespace))
}

// WaitUntilManagedResourcesDeleted waits until all ManagedResource resources in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilManagedResourcesDeleted(ctx context.Context) error {
	return kutil.WaitUntilResourcesDeletedWithTimeout(ctx, c.client, &resourcesv1alpha1.ManagedResourceList{}, defaultInterval, defaultTimeout, client.InNamespace(c.namespace))
}

// DeleteDNSOwners deletes all DNSOwner resources in the seed cluster associated with the shoot.
func (c *cleaner) DeleteDNSOwners(ctx context.Context) error {
	dnsOwnerList := &dnsv1alpha1.DNSOwnerList{}
	if err := c.client.List(ctx, dnsOwnerList); err != nil {
		return err
	}
	return applyToObjects(ctx, dnsOwnerList, func(ctx context.Context, object client.Object) error {
		c.logger.Info("Deleting DNSOwner resource", "dnsowner", client.ObjectKeyFromObject(object))
		if strings.HasPrefix(object.GetName(), c.namespace+"-") {
			return client.IgnoreNotFound(c.client.Delete(ctx, object))
		}
		return nil
	})
}

// WaitUntilDNSOwnersDeleted waits until all DNSOwner resources in the seed cluster associated with the shoot have been deleted.
func (c *cleaner) WaitUntilDNSOwnersDeleted(ctx context.Context) error {
	dnsOwnerList := &dnsv1alpha1.DNSOwnerList{}
	if err := c.client.List(ctx, dnsOwnerList); err != nil {
		return err
	}
	return applyToObjects(ctx, dnsOwnerList, func(ctx context.Context, object client.Object) error {
		if strings.HasPrefix(object.GetName(), c.namespace+"-") {
			return kutil.WaitUntilResourceDeletedWithTimeout(ctx, c.client, object, defaultInterval, defaultTimeout)
		}
		return nil
	})
}

// DeleteDNSEntries deletes all DNSEntry resources in the shoot namespace.
func (c *cleaner) DeleteDNSEntries(ctx context.Context) error {
	c.logger.Info("Deleting all DNSEntry resources in namespace", "namespace", c.namespace)
	return c.client.DeleteAllOf(ctx, &dnsv1alpha1.DNSEntry{}, client.InNamespace(c.namespace))
}

// WaitUntilDNSEntriesDeleted waits until all DNSEntry resources in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilDNSEntriesDeleted(ctx context.Context) error {
	return kutil.WaitUntilResourcesDeletedWithTimeout(ctx, c.client, &dnsv1alpha1.DNSEntryList{}, defaultInterval, defaultTimeout, client.InNamespace(c.namespace))
}

// DeleteDNSProviders deletes all DNSProvider resources in the shoot namespace.
func (c *cleaner) DeleteDNSProviders(ctx context.Context) error {
	c.logger.Info("Deleting all DNSProvider resources in namespace", "namespace", c.namespace)
	return c.client.DeleteAllOf(ctx, &dnsv1alpha1.DNSProvider{}, client.InNamespace(c.namespace))
}

// WaitUntilDNSProvidersDeleted waits until all DNSProvider resources in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilDNSProvidersDeleted(ctx context.Context) error {
	return kutil.WaitUntilResourcesDeletedWithTimeout(ctx, c.client, &dnsv1alpha1.DNSProviderList{}, defaultInterval, defaultTimeout, client.InNamespace(c.namespace))
}

// DeleteSecrets removes all remaining finalizers and deletes all secrets in the shoot namespace.
func (c *cleaner) DeleteSecrets(ctx context.Context) error {
	if err := c.removeFinalizersFromObjects(ctx, &corev1.SecretList{}); err != nil {
		return err
	}
	c.logger.Info("Deleting all secrets in namespace", "namespace", c.namespace)
	return c.client.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(c.namespace))
}

// DeleteNamespace deletes the shoot namespace in the seed cluster.
func (c *cleaner) DeleteNamespace(ctx context.Context) error {
	c.logger.Info("Deleting namespace", "namespace", c.namespace)
	return client.IgnoreNotFound(c.client.Delete(ctx, c.getEmptyNamespace()))
}

// WaitUntilNamespaceDeleted waits until the shoot namespace in the seed cluster has been deleted.
func (c *cleaner) WaitUntilNamespaceDeleted(ctx context.Context) error {
	return kutil.WaitUntilResourceDeletedWithTimeout(ctx, c.client, c.getEmptyNamespace(), defaultInterval, defaultTimeout)
}

// DeleteCluster deletes the shoot Cluster resource in the seed cluster.
func (c *cleaner) DeleteCluster(ctx context.Context) error {
	c.logger.Info("Deleting Cluster resource", "cluster", c.namespace)
	return client.IgnoreNotFound(c.client.Delete(ctx, c.getEmptyCluster()))
}

// WaitUntilClusterDeleted waits until the shoot Cluster resource in the seed cluster has been deleted.
func (c *cleaner) WaitUntilClusterDeleted(ctx context.Context) error {
	return kutil.WaitUntilResourceDeletedWithTimeout(ctx, c.client, c.getEmptyCluster(), defaultInterval, defaultTimeout)
}

func (c *cleaner) getEmptyNamespace() *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: c.namespace}}
}

func (c *cleaner) getEmptyCluster() *extensionsv1alpha1.Cluster {
	return &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: c.namespace}}
}

func (c *cleaner) getEmptyBackupEntry() *extensionsv1alpha1.BackupEntry {
	return &extensionsv1alpha1.BackupEntry{ObjectMeta: metav1.ObjectMeta{Name: c.backupEntryName}}
}

func (c *cleaner) confirmObjectsDeletion(ctx context.Context, objectList client.ObjectList) error {
	return c.applyToObjects(ctx, objectList, func(ctx context.Context, object client.Object) error {
		c.logger.Info("Confirming deletion", "kind", object.GetObjectKind().GroupVersionKind().Kind, "object", client.ObjectKeyFromObject(object))
		return client.IgnoreNotFound(gutil.ConfirmDeletion(ctx, c.client, object))
	})
}

func (c *cleaner) removeFinalizersFromObjects(ctx context.Context, objectList client.ObjectList) error {
	return c.applyToObjects(ctx, objectList, func(ctx context.Context, object client.Object) error {
		if len(object.GetFinalizers()) > 0 {
			c.logger.Info("Removing finalizers", "kind", object.GetObjectKind().GroupVersionKind().Kind, "object", client.ObjectKeyFromObject(object))
			return controllerutils.RemoveAllFinalizers(ctx, c.client, c.client, object)
		}
		return nil
	})
}

func (c *cleaner) applyToObjects(ctx context.Context, objectList client.ObjectList, fn func(ctx context.Context, object client.Object) error) error {
	if err := c.client.List(ctx, objectList, client.InNamespace(c.namespace)); err != nil {
		return err
	}
	return applyToObjects(ctx, objectList, fn)
}

func applyToObjects(ctx context.Context, objectList client.ObjectList, fn func(ctx context.Context, object client.Object) error) error {
	taskFns := make([]flow.TaskFn, 0, meta.LenList(objectList))
	if err := meta.EachListItem(objectList, func(obj runtime.Object) error {
		object, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("expected client.Object but got %T", obj)
		}
		taskFns = append(taskFns, func(ctx context.Context) error {
			return fn(ctx, object)
		})
		return nil
	}); err != nil {
		return err
	}
	return flow.Parallel(taskFns...)(ctx)
}

func applyToExtensionObjectKinds(ctx context.Context, fn func(kind string, objectList client.ObjectList) flow.TaskFn) error {
	var taskFns []flow.TaskFn
	for kind, objectList := range map[string]client.ObjectList{
		extensionsv1alpha1.ContainerRuntimeResource:      &extensionsv1alpha1.ContainerRuntimeList{},
		extensionsv1alpha1.ControlPlaneResource:          &extensionsv1alpha1.ControlPlaneList{},
		extensionsv1alpha1.DNSRecordResource:             &extensionsv1alpha1.DNSRecordList{},
		extensionsv1alpha1.ExtensionResource:             &extensionsv1alpha1.ExtensionList{},
		extensionsv1alpha1.InfrastructureResource:        &extensionsv1alpha1.InfrastructureList{},
		extensionsv1alpha1.NetworkResource:               &extensionsv1alpha1.NetworkList{},
		extensionsv1alpha1.OperatingSystemConfigResource: &extensionsv1alpha1.OperatingSystemConfigList{},
		extensionsv1alpha1.WorkerResource:                &extensionsv1alpha1.WorkerList{},
	} {
		taskFns = append(taskFns, fn(kind, objectList))
	}
	return flow.Parallel(taskFns...)(ctx)
}
