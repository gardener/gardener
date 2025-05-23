// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/wait"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controller/tokenrequestor"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/csrapprover"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/health"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/managedresource"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/networkpolicy"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/node"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

// AddToManager adds all controllers to the given manager.
func AddToManager(ctx context.Context, mgr manager.Manager, sourceCluster, targetCluster cluster.Cluster, cfg *resourcemanagerconfigv1alpha1.ResourceManagerConfiguration) error {
	targetClientSet, err := kubernetesclientset.NewForConfig(targetCluster.GetConfig())
	if err != nil {
		return fmt.Errorf("failed creating Kubernetes client: %w", err)
	}

	if cfg.Controllers.CSRApprover.Enabled {
		if err := (&csrapprover.Reconciler{
			CertificatesClient: targetClientSet.CertificatesV1().CertificateSigningRequests(),
			Config:             cfg.Controllers.CSRApprover,
		}).AddToManager(mgr, sourceCluster, targetCluster); err != nil {
			return fmt.Errorf("failed adding Kubelet CSR Approver controller: %w", err)
		}
	}

	if cfg.Controllers.GarbageCollector.Enabled {
		if err := (&garbagecollector.Reconciler{
			Config: cfg.Controllers.GarbageCollector,
			Clock:  clock.RealClock{},
		}).AddToManager(mgr, targetCluster); err != nil {
			return fmt.Errorf("failed adding garbage collector controller: %w", err)
		}
	}

	if err := health.AddToManager(ctx, mgr, sourceCluster, targetCluster, *cfg); err != nil {
		return fmt.Errorf("failed adding health controller: %w", err)
	}

	if err := (&managedresource.Reconciler{
		Config:                    cfg.Controllers.ManagedResource,
		ClassFilter:               resourcemanagerpredicate.NewClassFilter(*cfg.Controllers.ResourceClass),
		ClusterID:                 *cfg.Controllers.ClusterID,
		GarbageCollectorActivated: cfg.Controllers.GarbageCollector.Enabled,
	}).AddToManager(mgr, sourceCluster, targetCluster); err != nil {
		return fmt.Errorf("failed adding managed resource controller: %w", err)
	}

	if cfg.Controllers.NetworkPolicy.Enabled {
		if err := (&networkpolicy.Reconciler{
			Config: cfg.Controllers.NetworkPolicy,
		}).AddToManager(mgr, targetCluster); err != nil {
			return fmt.Errorf("failed adding networkpolicy controller: %w", err)
		}
	}

	if cfg.Controllers.TokenRequestor.Enabled {
		if err := (&tokenrequestor.Reconciler{
			ConcurrentSyncs: ptr.Deref(cfg.Controllers.TokenRequestor.ConcurrentSyncs, 0),
			Clock:           clock.RealClock{},
			JitterFunc:      wait.Jitter,
			APIAudiences:    []string{v1beta1constants.GardenerAudience},
			Class:           ptr.To(resourcesv1alpha1.ResourceManagerClassShoot),
		}).AddToManager(mgr, sourceCluster, targetCluster); err != nil {
			return fmt.Errorf("failed adding token requestor controller: %w", err)
		}
	}

	if err := node.AddToManager(mgr, targetCluster, *cfg); err != nil {
		return fmt.Errorf("failed adding node controller: %w", err)
	}

	return nil
}
