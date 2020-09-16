// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dns

import (
	"context"
	"path/filepath"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OwnerValues contains the values used for DNSOwner creation
type OwnerValues struct {
	Name    string `json:"name,omitempty"`
	OwnerID string `json:"ownerID,omitempty"`
	Active  bool   `json:"active,omitempty"`
}

// NewDNSOwner creates a new instance of DeployWaiter for a specific DNS owner.
func NewDNSOwner(
	values *OwnerValues,
	shootNamespace string,
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	client client.Client,
) component.DeployWaiter {
	return &dnsOwner{
		values:         values,
		shootNamespace: shootNamespace,
		ChartApplier:   applier,
		chartPath:      filepath.Join(chartsRootPath, "seed-dns", "owner"),
		client:         client,
	}
}

type dnsOwner struct {
	values         *OwnerValues
	shootNamespace string
	kubernetes.ChartApplier
	chartPath string
	client    client.Client
}

// Deploy implements Deployer and creates DNSOwner for the provided values
func (d *dnsOwner) Deploy(ctx context.Context) error {
	return d.Apply(ctx, d.chartPath, d.shootNamespace, d.values.Name, kubernetes.Values(d.values))
}

// Destroy implements Deployer and deletes the DNSOwner
func (d *dnsOwner) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(d.client.Delete(ctx, d.owner()))
}

// WaitCleanup implements Waiter
func (d *dnsOwner) WaitCleanup(ctx context.Context) error {
	return kutil.WaitUntilResourceDeleted(ctx, d.client, d.owner(), 5*time.Second)
}

// Wait implements Waiter, not applicable for the DNSOwner
func (d *dnsOwner) Wait(ctx context.Context) error { return nil }

// owner returns an empty DNSOwner used for deletion.
func (d *dnsOwner) owner() *dnsv1alpha1.DNSOwner {
	return &dnsv1alpha1.DNSOwner{ObjectMeta: metav1.ObjectMeta{Name: d.shootNamespace + "-" + d.values.Name}}
}
