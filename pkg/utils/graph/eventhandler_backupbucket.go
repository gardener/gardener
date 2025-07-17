// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

func (g *graph) setupBackupBucketWatch(_ context.Context, informer cache.Informer) error {
	_, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
			if !ok {
				return
			}
			g.handleBackupBucketCreateOrUpdate(backupBucket)
		},

		UpdateFunc: func(oldObj, newObj any) {
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
				!apiequality.Semantic.DeepEqual(oldBackupBucket.Spec.CredentialsRef, newBackupBucket.Spec.CredentialsRef) ||
				!apiequality.Semantic.DeepEqual(oldBackupBucket.Status.GeneratedSecretRef, newBackupBucket.Status.GeneratedSecretRef) {
				g.handleBackupBucketCreateOrUpdate(newBackupBucket)
			}
		},

		DeleteFunc: func(obj any) {
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
	g.deleteAllIncomingEdges(VertexTypeWorkloadIdentity, VertexTypeBackupBucket, "", backupBucket.Name)
	g.deleteAllOutgoingEdges(VertexTypeBackupBucket, "", backupBucket.Name, VertexTypeSeed)

	var (
		backupBucketVertex = g.getOrCreateVertex(VertexTypeBackupBucket, "", backupBucket.Name)
		credentialsVertex  *vertex
	)

	if backupBucket.Spec.CredentialsRef.APIVersion == securityv1alpha1.SchemeGroupVersion.String() && backupBucket.Spec.CredentialsRef.Kind == "WorkloadIdentity" {
		credentialsVertex = g.getOrCreateVertex(VertexTypeWorkloadIdentity, backupBucket.Spec.CredentialsRef.Namespace, backupBucket.Spec.CredentialsRef.Name)
	} else if backupBucket.Spec.CredentialsRef.APIVersion == corev1.SchemeGroupVersion.String() && backupBucket.Spec.CredentialsRef.Kind == "Secret" {
		credentialsVertex = g.getOrCreateVertex(VertexTypeSecret, backupBucket.Spec.CredentialsRef.Namespace, backupBucket.Spec.CredentialsRef.Name)
	}

	g.addEdge(credentialsVertex, backupBucketVertex)

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
