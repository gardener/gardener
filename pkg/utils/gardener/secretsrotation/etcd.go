// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secretsrotation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// RewriteEncryptedDataAddLabel patches all encrypted data in all namespaces in the target clusters and adds a label
// whose value is the name of the current ETCD encryption key secret. This function is useful for the ETCD encryption
// key secret rotation which requires all encrypted data to be rewritten to ETCD so that they become encrypted with the
// new key. After it's done, it snapshots ETCD so that we can restore backups in case we lose the cluster before the
// next incremental snapshot has been taken.
func RewriteEncryptedDataAddLabel(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	secretsManager secretsmanager.Interface,
	gvks ...schema.GroupVersionKind,
) error {
	etcdEncryptionKeySecret, found := secretsManager.Get(v1beta1constants.SecretNameETCDEncryptionKey, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameETCDEncryptionKey)
	}

	return rewriteEncryptedData(
		ctx,
		log,
		c,
		utils.MustNewRequirement(labelKeyRotationKeyName, selection.NotEquals, etcdEncryptionKeySecret.Name),
		func(objectMeta *metav1.ObjectMeta) {
			metav1.SetMetaDataLabel(objectMeta, labelKeyRotationKeyName, etcdEncryptionKeySecret.Name)
		},
		gvks...,
	)
}

// RewriteEncryptedDataRemoveLabel patches all encrypted data in all namespaces in the target clusters and removes the
// label whose value is the name of the current ETCD encryption key secret. This function is useful for the ETCD
// encryption key secret rotation which requires all encrypted data to be rewritten to ETCD so that they become
// encrypted with the new key.
func RewriteEncryptedDataRemoveLabel(
	ctx context.Context,
	log logr.Logger,
	runtimeClient client.Client,
	targetClient client.Client,
	namespace string,
	name string,
	gvks ...schema.GroupVersionKind,
) error {
	if err := rewriteEncryptedData(
		ctx,
		log,
		targetClient,
		utils.MustNewRequirement(labelKeyRotationKeyName, selection.Exists),
		func(objectMeta *metav1.ObjectMeta) {
			delete(objectMeta.Labels, labelKeyRotationKeyName)
		},
		gvks...,
	); err != nil {
		return err
	}

	return PatchAPIServerDeploymentMeta(ctx, runtimeClient, namespace, name, func(meta *metav1.PartialObjectMetadata) {
		delete(meta.Annotations, AnnotationKeyEtcdSnapshotted)
	})
}

func rewriteEncryptedData(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	requirement labels.Requirement,
	mutateObjectMeta func(*metav1.ObjectMeta),
	gvks ...schema.GroupVersionKind,
) error {
	var (
		limiter = rate.NewLimiter(rate.Limit(rotationQPS), rotationQPS)
		taskFns []flow.TaskFn
	)

	for _, gvk := range gvks {
		objList := &metav1.PartialObjectMetadataList{}
		objList.SetGroupVersionKind(gvk)
		if err := c.List(ctx, objList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(requirement)}); err != nil {
			return err
		}

		log.Info("Objects requiring to be rewritten after ETCD encryption key rotation", "gvk", gvk, "number", len(objList.Items))

		for _, o := range objList.Items {
			obj := o

			taskFns = append(taskFns, func(ctx context.Context) error {
				patch := client.StrategicMergeFrom(obj.DeepCopy())
				mutateObjectMeta(&obj.ObjectMeta)

				// Wait until we are allowed by the limiter to not overload the API server with too many requests.
				if err := limiter.Wait(ctx); err != nil {
					return err
				}

				return c.Patch(ctx, &obj, patch)
			})
		}
	}

	return flow.Parallel(taskFns...)(ctx)
}

// SnapshotETCDAfterRewritingEncryptedData performs a full snapshot on ETCD after the encrypted data (like secrets) have
// been rewritten as part of the ETCD encryption secret rotation. It adds an annotation to the API server deployment
// after it's done so that it does not take another snapshot again after it succeeded once.
func SnapshotETCDAfterRewritingEncryptedData(
	ctx context.Context,
	runtimeClient client.Client,
	snapshotEtcd func(ctx context.Context) error,
	namespace string,
	name string,
) error {
	// Check if we have to snapshot ETCD now that we have rewritten all encrypted data.
	meta := &metav1.PartialObjectMetadata{}
	meta.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	if err := runtimeClient.Get(ctx, kubernetesutils.Key(namespace, name), meta); err != nil {
		return err
	}

	if metav1.HasAnnotation(meta.ObjectMeta, AnnotationKeyEtcdSnapshotted) {
		return nil
	}

	if err := snapshotEtcd(ctx); err != nil {
		return err
	}

	// If we have hit this point then we have snapshotted ETCD successfully. Now we can mark this step as "completed"
	// (via an annotation) so that we do not trigger a snapshot again in a future reconciliation in case the current one
	// fails after this step.
	return PatchAPIServerDeploymentMeta(ctx, runtimeClient, namespace, name, func(meta *metav1.PartialObjectMetadata) {
		metav1.SetMetaDataAnnotation(&meta.ObjectMeta, AnnotationKeyEtcdSnapshotted, "true")
	})
}

// PatchAPIServerDeploymentMeta patches metadata of an API Server deployment.
func PatchAPIServerDeploymentMeta(ctx context.Context, c client.Client, namespace, name string, mutate func(deployment *metav1.PartialObjectMetadata)) error {
	meta := &metav1.PartialObjectMetadata{}
	meta.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	if err := c.Get(ctx, kubernetesutils.Key(namespace, name), meta); err != nil {
		return err
	}

	patch := client.MergeFrom(meta.DeepCopy())
	mutate(meta)
	return c.Patch(ctx, meta, patch)
}
