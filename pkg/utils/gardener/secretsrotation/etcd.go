// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
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
	runtimeClient client.Client,
	clientSet kubernetes.Interface,
	secretsManager secretsmanager.Interface,
	namespace string,
	name string,
	resourcesToEncrypt []string,
	encryptedResources []string,
	defaultGVKs []schema.GroupVersionKind,
) error {
	// Check if we have to label the resources to rewrite the data.
	meta := &metav1.PartialObjectMetadata{}
	meta.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	if err := runtimeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, meta); err != nil {
		return err
	}

	if metav1.HasAnnotation(meta.ObjectMeta, AnnotationKeyResourcesLabeled) {
		return nil
	}

	encryptedGVKs, message, err := GetResourcesForRewrite(clientSet.Kubernetes().Discovery(), resourcesToEncrypt, encryptedResources, defaultGVKs)
	if err != nil {
		return err
	}

	etcdEncryptionKeySecret, found := secretsManager.Get(v1beta1constants.SecretNameETCDEncryptionKey, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameETCDEncryptionKey)
	}

	if err := rewriteEncryptedData(
		ctx,
		log,
		clientSet.Client(),
		utils.MustNewRequirement(labelKeyRotationKeyName, selection.NotEquals, etcdEncryptionKeySecret.Name),
		func(objectMeta *metav1.ObjectMeta) {
			metav1.SetMetaDataLabel(objectMeta, labelKeyRotationKeyName, etcdEncryptionKeySecret.Name)
		},
		message+" (Add label)",
		encryptedGVKs...,
	); err != nil {
		return err
	}

	// If we have hit this point then we have labeled all the resources successfully. Now we can mark this step as "completed"
	// (via an annotation) so that we do not start labeling the resources in a future reconciliation in case the flow fails in
	// "Removing the label" and labels were only partially removed.
	return PatchAPIServerDeploymentMeta(ctx, runtimeClient, namespace, name, func(meta *metav1.PartialObjectMetadata) {
		metav1.SetMetaDataAnnotation(&meta.ObjectMeta, AnnotationKeyResourcesLabeled, "true")
	})
}

// RewriteEncryptedDataRemoveLabel patches all encrypted data in all namespaces in the target clusters and removes the
// label whose value is the name of the current ETCD encryption key secret. This function is useful for the ETCD
// encryption key secret rotation which requires all encrypted data to be rewritten to ETCD so that they become
// encrypted with the new key.
func RewriteEncryptedDataRemoveLabel(
	ctx context.Context,
	log logr.Logger,
	runtimeClient client.Client,
	targetClientSet kubernetes.Interface,
	namespace string,
	name string,
	resourcesToEncrypt []string,
	encryptedResources []string,
	defaultGVKs []schema.GroupVersionKind,
) error {
	encryptedGVKs, message, err := GetResourcesForRewrite(targetClientSet.Kubernetes().Discovery(), resourcesToEncrypt, encryptedResources, defaultGVKs)
	if err != nil {
		return err
	}

	if err := rewriteEncryptedData(
		ctx,
		log,
		targetClientSet.Client(),
		utils.MustNewRequirement(labelKeyRotationKeyName, selection.Exists),
		func(objectMeta *metav1.ObjectMeta) {
			delete(objectMeta.Labels, labelKeyRotationKeyName)
		},
		message+" (Remove label)",
		encryptedGVKs...,
	); err != nil {
		return err
	}

	return PatchAPIServerDeploymentMeta(ctx, runtimeClient, namespace, name, func(meta *metav1.PartialObjectMetadata) {
		delete(meta.Annotations, AnnotationKeyEtcdSnapshotted)
		delete(meta.Annotations, AnnotationKeyResourcesLabeled)
	})
}

func rewriteEncryptedData(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	requirement labels.Requirement,
	mutateObjectMeta func(*metav1.ObjectMeta),
	message string,
	gvks ...schema.GroupVersionKind,
) error {
	var (
		limiter = rate.NewLimiter(rate.Limit(rotationQPS), rotationQPS)
		taskFns []flow.TaskFn
	)

	for _, gvk := range gvks {
		var fns []flow.TaskFn

		objList := &metav1.PartialObjectMetadataList{}
		objList.SetGroupVersionKind(gvk)

		if err := c.List(ctx, objList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(requirement)}); err != nil {
			return err
		}

		log.Info(message, "gvk", gvk, "number", len(objList.Items)) //nolint:logcheck

		for _, o := range objList.Items {
			obj := o

			fns = append(fns, func(ctx context.Context) error {
				// client.StrategicMergeFrom is not used here because CRDs don't support strategic-merge-patch.
				// See https://github.com/kubernetes-sigs/controller-runtime/blob/a550f29c8781d1f7f9f19ab435ffac337b35a313/pkg/client/patch.go#L164-L173
				// This should be okay since we don't modify any lists here.
				patch := client.MergeFrom(obj.DeepCopy())
				mutateObjectMeta(&obj.ObjectMeta)

				// Wait until we are allowed by the limiter to not overload the API server with too many requests.
				if err := limiter.Wait(ctx); err != nil {
					return err
				}

				return c.Patch(ctx, &obj, patch)
			})
		}

		// Execute the tasks for the current GVK in parallel.
		taskFns = append(taskFns, flow.Parallel(fns...))
	}

	// Execute the sets of tasks for different GVKs in sequence.
	return flow.Sequential(taskFns...)(ctx)
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
	if err := runtimeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, meta); err != nil {
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
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, meta); err != nil {
		return err
	}

	patch := client.MergeFrom(meta.DeepCopy())
	mutate(meta)
	return c.Patch(ctx, meta, patch)
}

// GetResourcesForRewrite returns a list of schema.GroupVersionKind for all the resources that needs to be rewritten, either due to a encryption
// key rotation or a change in the list of resources requiring encryption.
func GetResourcesForRewrite(
	discoveryClient discovery.DiscoveryInterface,
	resourcesToEncrypt []string,
	encryptedResources []string,
	defaultGVKs []schema.GroupVersionKind,
) (
	[]schema.GroupVersionKind,
	string,
	error,
) {
	var (
		resourcesForRewrite        = resourcesToEncrypt
		encryptionConfigHasChanged = !sets.New(resourcesToEncrypt...).Equal(sets.New(encryptedResources...))
		encryptedGVKs              = sets.New[schema.GroupVersionKind]()
		groupResourcesToEncrypt    = []schema.GroupResource{}
		message                    = "Objects requiring to be rewritten after ETCD encryption key rotation"
	)

	// This means the function is invoked due to ETCD encryption configuration change, so include only the modified resources.
	if encryptionConfigHasChanged {
		resourcesForRewrite = getModifiedResources(resourcesToEncrypt, encryptedResources)
		message = "Objects requiring to be rewritten after modification of encryption config"
	}

	for _, resource := range resourcesForRewrite {
		groupResourcesToEncrypt = append(groupResourcesToEncrypt, schema.ParseGroupResource(resource))
	}

	resourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return encryptedGVKs.UnsortedList(), "", fmt.Errorf("error discovering server preferred resources: %w", err)
	}

	for _, list := range resourceLists {
		if len(list.APIResources) == 0 {
			continue
		}

		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			return encryptedGVKs.UnsortedList(), "", fmt.Errorf("error parsing groupVersion: %w", err)
		}

		for _, apiResource := range list.APIResources {
			// If the resource doesn't support get, list and patch, we cannot list and rewrite it
			if !slices.Contains(apiResource.Verbs, "get") ||
				!slices.Contains(apiResource.Verbs, "list") ||
				!slices.Contains(apiResource.Verbs, "patch") {
				continue
			}

			var (
				group   = gv.Group
				version = gv.Version
			)

			if apiResource.Group != "" {
				group = apiResource.Group
			}
			if apiResource.Version != "" {
				version = apiResource.Version
			}

			if shouldEncrypt := slices.ContainsFunc(groupResourcesToEncrypt, func(gr schema.GroupResource) bool {
				return gr.Group == group && gr.Resource == apiResource.Name
			}); shouldEncrypt {
				encryptedGVKs.Insert(schema.GroupVersionKind{Group: group, Version: version, Kind: apiResource.Kind})
			}
		}
	}

	// This means the function is invoked due to ETCD encryption key rotation, so include default GVKs as well.
	if !encryptionConfigHasChanged {
		encryptedGVKs.Insert(defaultGVKs...)
	}

	return encryptedGVKs.UnsortedList(), message, nil
}

func getModifiedResources(resourcesToEncrypt []string, encryptedResources []string) []string {
	var (
		oldResources = sets.New(encryptedResources...)
		newResources = sets.New(resourcesToEncrypt...)

		addedResources   = newResources.Difference(oldResources)
		removedResources = oldResources.Difference(newResources)
	)

	return sets.List(addedResources.Union(removedResources))
}
