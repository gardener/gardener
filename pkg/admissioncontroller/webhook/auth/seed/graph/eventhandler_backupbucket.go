// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package graph

import (
	"context"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

func (g *graph) setupBackupBucketWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return
			}
			g.handleBackupBucketCreateOrUpdate(backupBucket)
		},

		UpdateFunc: func(oldObj, newObj interface{}) {
			oldBackupBucket, ok := oldObj.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return
			}

			newBackupBucket, ok := newObj.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return
			}

			if !apiequality.Semantic.DeepEqual(oldBackupBucket.Spec.SeedName, newBackupBucket.Spec.SeedName) ||
				!apiequality.Semantic.DeepEqual(oldBackupBucket.Spec.SecretRef, newBackupBucket.Spec.SecretRef) ||
				!apiequality.Semantic.DeepEqual(oldBackupBucket.Status.GeneratedSecretRef, newBackupBucket.Status.GeneratedSecretRef) {
				g.handleBackupBucketCreateOrUpdate(newBackupBucket)
			}
		},

		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return
			}
			g.handleBackupBucketDelete(backupBucket)
		},
	})
	return err
}

func (g *graph) handleBackupBucketCreateOrUpdate(backupBucket *gardencorev1beta1.BackupBucket) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("BackupBucket", "CreateOrUpdate").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteAllIncomingEdges(VertexTypeSecret, VertexTypeBackupBucket, "", backupBucket.Name)
	g.deleteAllOutgoingEdges(VertexTypeBackupBucket, "", backupBucket.Name, VertexTypeSeed)

	var (
		backupBucketVertex = g.getOrCreateVertex(VertexTypeBackupBucket, "", backupBucket.Name)
		secretVertex       = g.getOrCreateVertex(VertexTypeSecret, backupBucket.Spec.SecretRef.Namespace, backupBucket.Spec.SecretRef.Name)
	)

	g.addEdge(secretVertex, backupBucketVertex)

	if backupBucket.Spec.SeedName != nil {
		seedVertex := g.getOrCreateVertex(VertexTypeSeed, "", *backupBucket.Spec.SeedName)
		g.addEdge(backupBucketVertex, seedVertex)
	}

	if backupBucket.Status.GeneratedSecretRef != nil {
		generatedSecretVertex := g.getOrCreateVertex(VertexTypeSecret, backupBucket.Status.GeneratedSecretRef.Namespace, backupBucket.Status.GeneratedSecretRef.Name)
		g.addEdge(generatedSecretVertex, backupBucketVertex)
	}
}

func (g *graph) handleBackupBucketDelete(backupBucket *gardencorev1beta1.BackupBucket) {
	start := time.Now()
	defer func() {
		metricUpdateDuration.WithLabelValues("BackupBucket", "Delete").Observe(time.Since(start).Seconds())
	}()
	g.lock.Lock()
	defer g.lock.Unlock()

	g.deleteVertex(VertexTypeBackupBucket, "", backupBucket.Name)
}
