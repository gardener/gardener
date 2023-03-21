// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// RewriteSecretsAddLabel patches all secrets in all namespaces in the target clusters and adds a label whose value is
// the name of the current ETCD encryption key secret. This function is useful for the ETCD encryption key secret
// rotation which requires all secrets to be rewritten to ETCD so that they become encrypted with the new key.
// After it's done, it snapshots ETCD so that we can restore backups in case we lose the cluster before the next
// incremental snapshot is taken.
func RewriteSecretsAddLabel(ctx context.Context, log logr.Logger, clientSet kubernetes.Interface, secretsManager secretsmanager.Interface) error {
	etcdEncryptionKeySecret, found := secretsManager.Get(v1beta1constants.SecretNameETCDEncryptionKey, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameETCDEncryptionKey)
	}

	return rewriteSecrets(
		ctx,
		log,
		clientSet,
		utils.MustNewRequirement(labelKeyRotationKeyName, selection.NotEquals, etcdEncryptionKeySecret.Name),
		func(objectMeta *metav1.ObjectMeta) {
			metav1.SetMetaDataLabel(objectMeta, labelKeyRotationKeyName, etcdEncryptionKeySecret.Name)
		},
	)
}

// SnapshotETCDAfterRewritingSecrets performs a full snapshot on ETCD after the secrets got rewritten as part of the
// ETCD encryption secret rotation. It adds an annotation to the kube-apiserver deployment after it's done so that it
// does not take another snapshot again after it succeeded once.
func SnapshotETCDAfterRewritingSecrets(ctx context.Context, runtimeClientSet kubernetes.Interface, snapshotEtcd func(ctx context.Context) error, kubeAPIServerNamespace string) error {
	// Check if we have to snapshot ETCD now that we have rewritten all secrets.
	meta := &metav1.PartialObjectMetadata{}
	meta.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	if err := runtimeClientSet.Client().Get(ctx, kubernetesutils.Key(kubeAPIServerNamespace, v1beta1constants.DeploymentNameKubeAPIServer), meta); err != nil {
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
	return PatchKubeAPIServerDeploymentMeta(ctx, runtimeClientSet, kubeAPIServerNamespace, func(meta *metav1.PartialObjectMetadata) {
		metav1.SetMetaDataAnnotation(&meta.ObjectMeta, AnnotationKeyEtcdSnapshotted, "true")
	})
}

// RewriteSecretsRemoveLabel patches all secrets in all namespaces in the target clusters and removes the label whose
// value is the name of the current ETCD encryption key secret. This function is useful for the ETCD encryption key
// secret rotation which requires all secrets to be rewritten to ETCD so that they become encrypted with the new key.
func RewriteSecretsRemoveLabel(ctx context.Context, log logr.Logger, runtimeClientSet, targetClientSet kubernetes.Interface, kubeAPIServerNamespace string) error {
	if err := rewriteSecrets(
		ctx,
		log,
		targetClientSet,
		utils.MustNewRequirement(labelKeyRotationKeyName, selection.Exists),
		func(objectMeta *metav1.ObjectMeta) {
			delete(objectMeta.Labels, labelKeyRotationKeyName)
		},
	); err != nil {
		return err
	}

	return PatchKubeAPIServerDeploymentMeta(ctx, runtimeClientSet, kubeAPIServerNamespace, func(meta *metav1.PartialObjectMetadata) {
		delete(meta.Annotations, AnnotationKeyEtcdSnapshotted)
	})
}

func rewriteSecrets(ctx context.Context, log logr.Logger, clientSet kubernetes.Interface, requirement labels.Requirement, mutateObjectMeta func(*metav1.ObjectMeta)) error {
	secretList := &metav1.PartialObjectMetadataList{}
	secretList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("SecretList"))
	if err := clientSet.Client().List(ctx, secretList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(requirement)}); err != nil {
		return err
	}

	log.Info("Secrets requiring to be rewritten after ETCD encryption key rotation", "number", len(secretList.Items))

	var (
		limiter = rate.NewLimiter(rate.Limit(rotationQPS), rotationQPS)
		taskFns []flow.TaskFn
	)

	for _, obj := range secretList.Items {
		secret := obj

		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.StrategicMergeFrom(secret.DeepCopy())
			mutateObjectMeta(&secret.ObjectMeta)

			// Wait until we are allowed by the limiter to not overload the kube-apiserver with too many requests.
			if err := limiter.Wait(ctx); err != nil {
				return err
			}

			return clientSet.Client().Patch(ctx, &secret, patch)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

// PatchKubeAPIServerDeploymentMeta patches metadata of a Kubernetes API-Server deployment
func PatchKubeAPIServerDeploymentMeta(ctx context.Context, clientSet kubernetes.Interface, namespace string, mutate func(deployment *metav1.PartialObjectMetadata)) error {
	meta := &metav1.PartialObjectMetadata{}
	meta.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	if err := clientSet.Client().Get(ctx, kubernetesutils.Key(namespace, v1beta1constants.DeploymentNameKubeAPIServer), meta); err != nil {
		return err
	}

	patch := client.MergeFrom(meta.DeepCopy())
	mutate(meta)
	return clientSet.Client().Patch(ctx, meta, patch)
}
