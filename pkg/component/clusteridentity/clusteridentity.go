// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusteridentity

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ManagedResourceControlName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceControlName = "cluster-identity"
	// ShootManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ShootManagedResourceName = "shoot-core-" + ManagedResourceControlName
)

// Interface contains functions for managing cluster identities.
type Interface interface {
	component.DeployWaiter
	SetIdentity(string)
}

type clusterIdentity struct {
	client                  client.Client
	namespace               string
	identity                string
	identityType            string
	managedResourceName     string
	managedResourceDeleteFn func(ctx context.Context, client client.Client, namespace string, name string) error
}

func newComponent(
	c client.Client,
	namespace string,
	identity string,
	identityType string,
	managedResourceName string,
	managedResourceDeleteFn func(ctx context.Context, client client.Client, namespace string, name string) error,
) Interface {
	return &clusterIdentity{
		client:                  c,
		namespace:               namespace,
		identity:                identity,
		identityType:            identityType,
		managedResourceName:     managedResourceName,
		managedResourceDeleteFn: managedResourceDeleteFn,
	}
}

// NewForSeed creates new instance of Deployer for the seed's cluster identity.
func NewForSeed(c client.Client, namespace, identity string) Interface {
	return newComponent(
		c,
		namespace,
		identity,
		v1beta1constants.ClusterIdentityOriginSeed,
		ManagedResourceControlName,
		managedresources.DeleteForSeed,
	)
}

// NewForShoot creates new instance of Deployer for the shoot's cluster identity.
func NewForShoot(c client.Client, namespace, identity string) Interface {
	return newComponent(
		c,
		namespace,
		identity,
		v1beta1constants.ClusterIdentityOriginShoot,
		ShootManagedResourceName,
		managedresources.DeleteForShoot,
	)
}

func (c *clusterIdentity) Deploy(ctx context.Context) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.ClusterIdentity,
			Namespace: metav1.NamespaceSystem,
		},
		Immutable: ptr.To(true),
		Data: map[string]string{
			v1beta1constants.ClusterIdentity:       c.identity,
			v1beta1constants.ClusterIdentityOrigin: c.identityType,
		},
	}

	var registry *managedresources.Registry
	if c.identityType == v1beta1constants.ClusterIdentityOriginShoot {
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	} else {
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	}

	resources, err := registry.AddAllAndSerialize(configMap)
	if err != nil {
		return err
	}

	switch c.identityType {
	case v1beta1constants.ClusterIdentityOriginShoot:
		return managedresources.CreateForShoot(ctx, c.client, c.namespace, c.managedResourceName, managedresources.LabelValueGardener, false, resources)
	case v1beta1constants.ClusterIdentityOriginSeed:
		return managedresources.CreateForSeed(ctx, c.client, c.namespace, c.managedResourceName, false, resources)
	default:
		// this should never happen
		return fmt.Errorf("unknown cluster identity type %s", c.identityType)
	}
}

func (c *clusterIdentity) Destroy(ctx context.Context) error {
	return c.managedResourceDeleteFn(ctx, c.client, c.namespace, c.managedResourceName)
}

func (c *clusterIdentity) SetIdentity(identity string) {
	c.identity = identity
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (c *clusterIdentity) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, c.client, c.namespace, c.managedResourceName)
}

func (c *clusterIdentity) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, c.client, c.namespace, c.managedResourceName)
}

// IsClusterIdentityEmptyOrFromOrigin checks if the cluster-identity config map does not exist or is from the same origin
func IsClusterIdentityEmptyOrFromOrigin(ctx context.Context, c client.Client, origin string) (bool, error) {
	clusterIdentity := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: v1beta1constants.ClusterIdentity}, clusterIdentity); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	return clusterIdentity.Data[v1beta1constants.ClusterIdentityOrigin] == origin || clusterIdentity.Data[v1beta1constants.ClusterIdentityOrigin] == "", nil
}
