// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/networking/coredns"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// IsPodNetworkAvailable checks if the ManagedResource for CoreDNS is deployed and ready. If yes, pod network must be
// available (otherwise, CoreDNS (which runs in this network) wouldn't be available).
func (b *AutonomousBotanist) IsPodNetworkAvailable(ctx context.Context) (bool, error) {
	managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: coredns.ManagedResourceName, Namespace: b.Shoot.ControlPlaneNamespace}}
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource); err != nil {
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed reading ManagedResource %s: %w", client.ObjectKeyFromObject(managedResource), err)
	}
	return health.CheckManagedResource(managedResource) == nil, nil
}
