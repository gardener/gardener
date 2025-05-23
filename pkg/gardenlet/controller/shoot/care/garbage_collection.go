// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/gardenlet/operation"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// GarbageCollection contains required information for shoot and seed garbage collection.
type GarbageCollection struct {
	initializeShootClients ShootClientInit
	shoot                  *shoot.Shoot
	seedClient             client.Client
	log                    logr.Logger
}

// NewGarbageCollection creates a new garbage collection instance.
func NewGarbageCollection(op *operation.Operation, shootClientInit ShootClientInit) *GarbageCollection {
	return &GarbageCollection{
		shoot:                  op.Shoot,
		initializeShootClients: shootClientInit,
		seedClient:             op.SeedClientSet.Client(),
		log:                    op.Logger,
	}
}

// Collect cleans the Seed and the Shoot cluster from no longer required
// objects. It receives a botanist object <botanist> which stores the Shoot object.
func (g *GarbageCollection) Collect(ctx context.Context) {
	shootClient, apiServerRunning, err := g.initializeShootClients()
	if err != nil || !apiServerRunning {
		if err != nil {
			g.log.Error(err, "Could not initialize Shoot client for garbage collection")
		}
		return
	}
	if err := g.performGarbageCollectionShoot(ctx, shootClient.Client()); err != nil {
		g.log.Error(err, "Error during shoot garbage collection")
	}
	g.log.V(1).Info("Successfully performed full garbage collection")
}

// PerformGarbageCollectionShoot performs garbage collection in the kube-system namespace in the Shoot
// cluster, i.e., it deletes evicted pods (mitigation for https://github.com/kubernetes/kubernetes/issues/55051).
func (g *GarbageCollection) performGarbageCollectionShoot(ctx context.Context, shootClient client.Client) error {
	if err := g.deleteOrphanedNodeLeases(ctx, shootClient); err != nil {
		return fmt.Errorf("failed deleting orphaned node lease objects: %w", err)
	}

	namespace := metav1.NamespaceSystem
	if g.shoot.GetInfo().DeletionTimestamp != nil {
		namespace = metav1.NamespaceAll
	}

	podList := &corev1.PodList{}
	if err := shootClient.List(ctx, podList, client.InNamespace(namespace)); err != nil {
		return err
	}
	return kubernetesutils.DeleteStalePods(ctx, g.log, shootClient, podList.Items)
}

// See https://github.com/gardener/gardener/issues/8749 and https://github.com/kubernetes/kubernetes/issues/109777.
// kubelet sometimes created Lease objects without owner reference. When the respective node gets deleted eventually,
// the Lease object remains in the system and no Kubernetes controller will ever clean it up. Hence, this function takes
// over this task.
// TODO: Remove this function when support for Kubernetes 1.28 is dropped.
func (g *GarbageCollection) deleteOrphanedNodeLeases(ctx context.Context, c client.Client) error {
	leaseList := &coordinationv1.LeaseList{}
	if err := c.List(ctx, leaseList, client.InNamespace(corev1.NamespaceNodeLease)); err != nil {
		return err
	}

	var taskFns []flow.TaskFn

	for _, l := range leaseList.Items {
		if len(l.OwnerReferences) > 0 {
			continue
		}
		lease := l.DeepCopy()

		taskFns = append(taskFns, func(ctx context.Context) error {
			if err := c.Get(ctx, client.ObjectKey{Name: lease.Name}, &metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "Node"}}); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed getting node %s when checking for potential orphaned Lease %s: %w", lease.Name, client.ObjectKeyFromObject(lease), err)
				}

				g.log.Info("Detected orphaned Lease object, cleaning it up", "nodeName", lease.Name, "lease", client.ObjectKeyFromObject(lease))
				return kubernetesutils.DeleteObject(ctx, c, lease)
			}

			return nil
		})
	}

	return flow.ParallelN(100, taskFns...)(ctx)
}
