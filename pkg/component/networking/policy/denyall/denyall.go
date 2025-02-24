// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package denyall

import (
	"context"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const managedResourceName = "shoot-core-deny-all-traffic"

// Interface contains functions for a DenyAllTraffic deployer.
type Interface interface {
	component.DeployWaiter
}

// New creates a new instance of DeployWaiter for DenyAllTraffic.
func New(
	client client.Client,
	namespace string,
) Interface {
	return &denyAllTraffic{
		client:    client,
		namespace: namespace,
	}
}

type denyAllTraffic struct {
	client    client.Client
	namespace string
}

func (d *denyAllTraffic) Deploy(ctx context.Context) error {
	data, err := d.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, d.client, d.namespace, managedResourceName, managedresources.LabelValueGardener, false, data)
}

func (d *denyAllTraffic) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, d.client, d.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (d *denyAllTraffic) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, d.client, d.namespace, managedResourceName)
}

func (d *denyAllTraffic) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, d.client, d.namespace, managedResourceName)
}

func (d *denyAllTraffic) computeResourcesData() (map[string][]byte, error) {
	var registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

	return registry.AddAllAndSerialize(&networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardener.cloud--deny-all",
			Namespace: metav1.NamespaceSystem,
			Annotations: map[string]string{
				v1beta1constants.GardenerDescription: "Disables all ingress and egress traffic into/from this namespace.",
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Egress:      []networkingv1.NetworkPolicyEgressRule{},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		},
	})
}
