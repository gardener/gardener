// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extauthzserver

import (
	"context"
	"fmt"
	"time"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// Port is the port exposed by the ext-authz-server.
	Port = 10000

	name                = "ext-authz-server"
	managedResourceName = name
)

// Values is the values for ext-authz-server configuration.
type Values struct {
	// Image is the ext-authz-server container image.
	Image string
	// PriorityClassName is the name of the priority class of the ext-authz-server.
	PriorityClassName string
	// Replicas is the number of pod replicas for the ext-authz-server.
	Replicas int32
	// IsGardenCluster specifies whether the cluster is garden cluster.
	IsGardenCluster bool
}

type extAuthzServer struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// New creates a new instance of an ext-authz-server deployer.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.DeployWaiter {
	return &extAuthzServer{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

func (e *extAuthzServer) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	serializedResources, err := registry.AddAllAndSerialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resources: %w", err)
	}

	return managedresources.CreateForSeed(ctx, e.client, e.namespace, e.getPrefix()+managedResourceName, false, serializedResources)
}

func (e *extAuthzServer) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, e.client, e.namespace, e.getPrefix()+managedResourceName)
}

var timeoutWaitForManagedResources = 2 * time.Minute

func (e *extAuthzServer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, e.client, e.namespace, e.getPrefix()+managedResourceName)
}

func (e *extAuthzServer) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResources)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, e.client, e.namespace, e.getPrefix()+managedResourceName)
}

func (e *extAuthzServer) getPrefix() string {
	if e.values.IsGardenCluster {
		return operatorv1alpha1.VirtualGardenNamePrefix
	}

	return ""
}

func (e *extAuthzServer) getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp: e.getPrefix() + name,
	}
}
