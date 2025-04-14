// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package persesoperator

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// managedResourceName is the name of the ManagedResource for the perses-operator resources.
	managedResourceName = "perses-operator"
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

// Values contains configuration values for the perses-operator resources.
type Values struct {
	// Image defines the container image of perses-operator.
	Image string
	// PriorityClassName is the name of the priority class for the deployment.
	PriorityClassName string
}

// New creates a new instance of DeployWaiter for the perses-operator.
func New(client client.Client, namespace string, values Values) component.DeployWaiter {
	return &persesOperator{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type persesOperator struct {
	client    client.Client
	namespace string
	values    Values
}

func (p *persesOperator) Deploy(ctx context.Context) error {
	registry := managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

	resources, err := registry.AddAllAndSerialize(
		p.serviceAccount(),
		p.deployment(),
		p.vpa(),
		p.clusterRole(),
		p.clusterRoleBinding(),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeedWithLabels(ctx, p.client, p.namespace, managedResourceName, false, map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, resources)
}

func (p *persesOperator) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, p.client, p.namespace, managedResourceName)
}

func (p *persesOperator) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, p.client, p.namespace, managedResourceName)
}

func (p *persesOperator) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, p.client, p.namespace, managedResourceName)
}

// GetLabels returns the labels for the perses-operator.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp: "perses-operator",
	}
}
