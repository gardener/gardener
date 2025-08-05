// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/gardenlet/operation"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
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
