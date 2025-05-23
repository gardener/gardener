// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
		extensionsv1alpha1.ContainerRuntimeResource:      &extensionsv1alpha1.ContainerRuntimeList{},
		extensionsv1alpha1.ControlPlaneResource:          &extensionsv1alpha1.ControlPlaneList{},
		extensionsv1alpha1.ExtensionResource:             &extensionsv1alpha1.ExtensionList{},
		extensionsv1alpha1.InfrastructureResource:        &extensionsv1alpha1.InfrastructureList{},
		extensionsv1alpha1.NetworkResource:               &extensionsv1alpha1.NetworkList{},
		extensionsv1alpha1.OperatingSystemConfigResource: &extensionsv1alpha1.OperatingSystemConfigList{},
		extensionsv1alpha1.WorkerResource:                &extensionsv1alpha1.WorkerList{},
	}

	machineKindToObjectList = map[string]client.ObjectList{
		"MachineDeployment": &machinev1alpha1.MachineDeploymentList{},
		"MachineSet":        &machinev1alpha1.MachineSetList{},
		"Machine":           &machinev1alpha1.MachineList{},
		"MachineClass":      &machinev1alpha1.MachineClassList{},
	}

	kubernetesKindToObjectList = map[string]client.ObjectList{
		"ConfigMap": &corev1.ConfigMapList{},
		"Secret":    &corev1.SecretList{},
	}
)

type cleaner struct {
	seedClient    client.Client
	gardenClient  client.Client
	seedNamespace string
	log           logr.Logger
}

// NewCleaner creates a cleaner with the given clients and logger, for a shoot with the given namespace.
func NewCleaner(log logr.Logger, seedClient, gardenClient client.Client, seedNamespace string) *cleaner {
	return &cleaner{
		seedClient:    seedClient,
		gardenClient:  gardenClient,
		seedNamespace: seedNamespace,
		log:           log,
	}
}

// DeleteExtensionObjects deletes all extension objects in the shoot namespace.
func (c *cleaner) DeleteExtensionObjects(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			c.log.Info("Deleting all extension resources", "kind", kind, "namespace", c.seedNamespace)
			return extensions.DeleteExtensionObjects(ctx, c.seedClient, objectList, c.seedNamespace, nil)
		}
	}, extensionKindToObjectList)
}

// WaitUntilExtensionObjectsDeleted waits until all extension objects in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilExtensionObjectsDeleted(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			return extensions.WaitUntilExtensionObjectsDeleted(ctx, c.seedClient, c.log, objectList, kind, c.seedNamespace, DefaultInterval, DefaultTimeout, nil)
		}
	}, extensionKindToObjectList)
}

// DeleteMachineResources deletes all MachineControllerManager resources in the shoot namespace.
func (c *cleaner) DeleteMachineResources(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		c.log.Info("Deleting all machine resources in namespace", "namespace", c.seedNamespace, "kind", kind)
		return utilclient.ForceDeleteObjects(c.seedClient, c.seedNamespace, objectList)
	}, machineKindToObjectList)
}

// WaitUntilMachineResourcesDeleted waits until all MachineControllerManager resources in the shoot namespace have been deleted.
func (c *cleaner) WaitUntilMachineResourcesDeleted(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(_ string, objectList client.ObjectList) flow.TaskFn {
		return func(ctx context.Context) error {
			return kubernetesutils.WaitUntilResourcesDeleted(ctx, c.seedClient, objectList, DefaultInterval, client.InNamespace(c.seedNamespace))
		}
	}, machineKindToObjectList)
}

// SetKeepObjectsForManagedResources sets keepObjects to false for all ManagedResource resources in the shoot namespace.
func (c *cleaner) SetKeepObjectsForManagedResources(ctx context.Context) error {
	mrList := &resourcesv1alpha1.ManagedResourceList{}
	if err := c.seedClient.List(ctx, mrList, client.InNamespace(c.seedNamespace)); err != nil {
		return err
	}

	return utilclient.ApplyToObjects(ctx, mrList, func(ctx context.Context, object client.Object) error {
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
	return kubernetesutils.WaitUntilResourcesDeleted(ctx, c.seedClient, &resourcesv1alpha1.ManagedResourceList{}, DefaultInterval, client.InNamespace(c.seedNamespace))
}

// DeleteKubernetesResources removes all remaining finalizers and deletes all passed kubernetes resources in the shoot namespace.
func (c *cleaner) DeleteKubernetesResources(ctx context.Context) error {
	return utilclient.ApplyToObjectKinds(ctx, func(kind string, objectList client.ObjectList) flow.TaskFn {
		c.log.Info("Deleting all resources in namespace", "namespace", c.seedNamespace, "kind", kind)
		return utilclient.ForceDeleteObjects(c.seedClient, c.seedNamespace, objectList)
	}, kubernetesKindToObjectList)
}

// DeleteCluster deletes the shoot Cluster resource in the seed cluster.
func (c *cleaner) DeleteCluster(ctx context.Context) error {
	cluster := c.getEmptyCluster()

	c.log.Info("Deleting Cluster resource", "clusterName", cluster.Name)
	return client.IgnoreNotFound(c.seedClient.Delete(ctx, cluster))
}

func (c *cleaner) getEmptyCluster() *extensionsv1alpha1.Cluster {
	return &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: c.seedNamespace}}
}

func (c *cleaner) removeFinalizersFromObjects(ctx context.Context, objectList client.ObjectList) error {
	return utilclient.ApplyToObjects(ctx, objectList, func(ctx context.Context, object client.Object) error {
		if len(object.GetFinalizers()) > 0 {
			c.log.Info("Removing finalizers", "kind", object.GetObjectKind().GroupVersionKind().Kind, "object", client.ObjectKeyFromObject(object))
			return controllerutils.RemoveAllFinalizers(ctx, c.seedClient, object)
		}
		return nil
	})
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

	return c.removeFinalizersFromObjects(ctx, shootMRList)
}
