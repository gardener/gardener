// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/networking/coredns"
	"github.com/gardener/gardener/pkg/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/controller/networkpolicy/hostnameresolver"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// IsPodNetworkAvailable checks if the ManagedResource for CoreDNS is deployed and ready. If yes, pod network must be
// available. Otherwise, CoreDNS which runs in this network wouldn't be available.
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

// ApplyNetworkPolicies reconciles all namespaces in the cluster in order to apply the network policies.
func (b *AutonomousBotanist) ApplyNetworkPolicies(ctx context.Context) error {
	reconciler := &networkpolicy.Reconciler{
		RuntimeClient: b.SeedClientSet.Client(),
		Resolver:      hostnameresolver.NewNoOpProvider(),
		RuntimeNetworks: networkpolicy.RuntimeNetworkConfig{
			IPFamilies: b.Shoot.GetInfo().Spec.Networking.IPFamilies,
			Pods:       netIPNetSliceToStringSlice(b.Shoot.Networks.Pods),
			Services:   netIPNetSliceToStringSlice(b.Shoot.Networks.Services),
			Nodes:      netIPNetSliceToStringSlice(b.Shoot.Networks.Nodes),
		},
	}

	namespaceList := &corev1.NamespaceList{}
	if err := b.SeedClientSet.Client().List(ctx, namespaceList); err != nil {
		return fmt.Errorf("failed listing namespaces: %w", err)
	}

	for _, namespace := range namespaceList.Items {
		b.Logger.Info("Reconciling NetworkPolicies using gardenlet's reconciliation logic", "namespaceName", namespace.Name)

		reconcilerCtx := log.IntoContext(ctx, b.Logger.WithName("networkpolicy-reconciler").WithValues("namespaceName", namespace.Name))
		if _, err := reconciler.Reconcile(reconcilerCtx, reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.Name}}); err != nil {
			return fmt.Errorf("failed running NetworkPolicy controller for namespace %q: %w", namespace.Name, err)
		}
	}

	return nil
}

func netIPNetSliceToStringSlice(in []net.IPNet) []string {
	out := make([]string, 0, len(in))
	for _, ip := range in {
		out = append(out, ip.String())
	}
	return out
}
