// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package secretsrotation

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagemigrationv1 "k8s.io/api/storagemigration/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

var (
	// StorageVersionMigrationRetryInterval is the interval at which the status of StorageVersionMigration resources is retried. Exposed for testing purposes.
	StorageVersionMigrationRetryInterval = 30 * time.Second
	// StorageVersionMigrationWaitTimeout is the timeout for waiting for the completion of StorageVersionMigration resources. Exposed for testing purposes.
	StorageVersionMigrationWaitTimeout = 5 * time.Minute
)

// RewriteEncryptedData rewrites the encrypted data for the given resources. It checks if the storage version migrator is enabled and
// creates StorageVersionMigration resources if necessary. Otherwise, it directly rewrites the encrypted data by adding the appropriate labels.
func RewriteEncryptedData(
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
	defaultGRs []schema.GroupResource,
	storageVersionMigratorEnabled bool,
) error {
	if storageVersionMigratorEnabled {
		// Check if we have already reached the snapshot stage for ETCD. If the annotation is present,
		// we can skip creating the StorageVersionMigration resources. This is to avoid recreating them
		// unnecessarily in case the cleanup phase fails and the flow is retried.
		meta := &metav1.PartialObjectMetadata{}
		meta.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
		if err := runtimeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, meta); err != nil {
			return err
		}

		if metav1.HasAnnotation(meta.ObjectMeta, AnnotationKeyEtcdSnapshotted) {
			return nil
		}

		return CreateStorageVersionMigrationResourcesAndWaitForCompletion(ctx, log, clientSet, secretsManager, resourcesToEncrypt, encryptedResources, defaultGRs)
	}

	return RewriteEncryptedDataAddLabel(ctx, log, runtimeClient, clientSet, secretsManager, namespace, name, resourcesToEncrypt, encryptedResources, defaultGVKs)
}

// CreateStorageVersionMigrationResourcesAndWaitForCompletion creates StorageVersionMigration resources for the given
// encrypted resources and waits for their completion.
func CreateStorageVersionMigrationResourcesAndWaitForCompletion(
	ctx context.Context,
	log logr.Logger,
	clientSet kubernetes.Interface,
	secretsManager secretsmanager.Interface,
	resourcesToEncrypt []string,
	encryptedResources []string,
	defaultGRs []schema.GroupResource,
) error {
	etcdEncryptionKeySecret, found := secretsManager.Get(v1beta1constants.SecretNameETCDEncryptionKey, secretsmanager.Current)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameETCDEncryptionKey)
	}

	groupResources, _, encryptionConfigHasChanged := getGroupResourcesForRewrite(resourcesToEncrypt, encryptedResources)
	if !encryptionConfigHasChanged {
		groupResources = append(groupResources, defaultGRs...)
	}

	groupResources, err := filterServedGroupResources(clientSet.Kubernetes().Discovery(), groupResources)
	if err != nil {
		return err
	}

	var fns []flow.TaskFn
	for _, gr := range groupResources {
		name := GetStorageVersionMigrationNameForGR(gr)
		svm := &storagemigrationv1.StorageVersionMigration{ObjectMeta: metav1.ObjectMeta{Name: name}}

		fns = append(fns, func(ctx context.Context) error {
			if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, clientSet.Client(), svm, func() error {
				metav1.SetMetaDataLabel(&svm.ObjectMeta, labelKeyRotationKeyName, etcdEncryptionKeySecret.Name)

				svm.Spec = storagemigrationv1.StorageVersionMigrationSpec{
					Resource: metav1.GroupResource{
						Group:    gr.Group,
						Resource: gr.Resource,
					},
				}
				return nil
			}); err != nil {
				return fmt.Errorf("error while creating StorageVersionMigration object %q: %w", name, err)
			}

			log.Info("Successfully created/found existing StorageVersionMigration object, waiting for it to succeed", "name", name)

			timeoutCtx, cancel := context.WithTimeout(ctx, StorageVersionMigrationWaitTimeout)
			defer cancel()

			if err := retry.Until(timeoutCtx, StorageVersionMigrationRetryInterval, func(ctx context.Context) (bool, error) {
				if err := clientSet.Client().Get(ctx, client.ObjectKeyFromObject(svm), svm); err != nil {
					return retry.MinorError(fmt.Errorf("failed getting StorageVersionMigration object %q: %w", name, err))
				}

				if meta.IsStatusConditionTrue(svm.Status.Conditions, string(storagemigrationv1.MigrationSucceeded)) {
					log.Info("Migration succeeded for StorageVersionMigration object", "name", name)
					return retry.Ok()
				}

				if meta.IsStatusConditionTrue(svm.Status.Conditions, string(storagemigrationv1.MigrationRunning)) {
					return retry.MinorError(fmt.Errorf("migration still in progress for StorageVersionMigration object %q", name))
				}

				if meta.IsStatusConditionTrue(svm.Status.Conditions, string(storagemigrationv1.MigrationFailed)) {
					return retry.SevereError(fmt.Errorf("migration failed for StorageVersionMigration object %q", name))
				}

				return retry.MinorError(fmt.Errorf("waiting for StorageVersionMigration object %q to start", name))
			}); err != nil {
				return fmt.Errorf("error while waiting for StorageVersionMigration object %q to succeed: %w", name, err)
			}
			return nil
		})
	}

	if err := flow.Parallel(fns...)(ctx); err != nil {
		return fmt.Errorf("error while processing StorageVersionMigration objects: %w", err)
	}
	return nil
}

// CleanupStorageVersionMigrationObjects cleans up all StorageVersionMigration objects that have the rotation label.
func CleanupStorageVersionMigrationObjects(
	ctx context.Context,
	clientSet kubernetes.Interface,
) error {
	storageVersionMigrationList := &storagemigrationv1.StorageVersionMigrationList{}
	if err := clientSet.Client().List(ctx, storageVersionMigrationList, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(utils.MustNewRequirement(labelKeyRotationKeyName, selection.Exists)),
	}); err != nil {
		return fmt.Errorf("error while listing StorageVersionMigration objects: %w", err)
	}

	return kubernetesutils.DeleteObjectsFromListConditionally(ctx, clientSet.Client(), storageVersionMigrationList, nil)
}

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
		return fmt.Errorf("failed to rewrite encrypted data: %w", err)
	}

	// If we have hit this point then we have labeled all the resources successfully. Now we can mark this step as "completed"
	// (via an annotation) so that we do not start labeling the resources in a future reconciliation in case the flow fails in
	// "Removing the label" and labels were only partially removed.
	return PatchAPIServerDeploymentMeta(ctx, runtimeClient, namespace, name, func(meta *metav1.PartialObjectMetadata) {
		metav1.SetMetaDataAnnotation(&meta.ObjectMeta, AnnotationKeyResourcesLabeled, "true")
	})
}

// CompleteEncryptedDataRewrite completes the process of rewriting encrypted data by cleaning up StorageVersionMigration objects if enabled and removing labels from the encrypted data.
func CompleteEncryptedDataRewrite(
	ctx context.Context,
	log logr.Logger,
	runtimeClient client.Client,
	targetClientSet kubernetes.Interface,
	namespace string,
	name string,
	resourcesToEncrypt []string,
	encryptedResources []string,
	defaultGVKs []schema.GroupVersionKind,
	storageVersionMigratorEnabled bool,
) error {
	if storageVersionMigratorEnabled {
		if err := CleanupStorageVersionMigrationObjects(ctx, targetClientSet); err != nil {
			return fmt.Errorf("error while cleaning up StorageVersionMigration objects: %w", err)
		}

		if err := PatchAPIServerDeploymentMeta(ctx, runtimeClient, namespace, name, func(meta *metav1.PartialObjectMetadata) {
			delete(meta.Annotations, AnnotationKeyEtcdSnapshotted)
		}); err != nil {
			return fmt.Errorf("failed to remove annotations from API Server deployment after cleaning up StorageVersionMigration objects: %w", err)
		}
	}

	if err := RewriteEncryptedDataRemoveLabel(ctx, log, runtimeClient, targetClientSet, namespace, name, resourcesToEncrypt, encryptedResources, defaultGVKs); err != nil {
		return fmt.Errorf("error while removing labels from encrypted data: %w", err)
	}

	return nil
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
		return fmt.Errorf("failed to get resources for rewrite: %w", err)
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
		return fmt.Errorf("failed to rewrite encrypted data: %w", err)
	}

	if err := PatchAPIServerDeploymentMeta(ctx, runtimeClient, namespace, name, func(meta *metav1.PartialObjectMetadata) {
		delete(meta.Annotations, AnnotationKeyEtcdSnapshotted)
		delete(meta.Annotations, AnnotationKeyResourcesLabeled)
	}); err != nil {
		return fmt.Errorf("failed to remove annotations from API Server deployment: %w", err)
	}
	return nil
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

		// Use a per-List timeout so that a hung TCP connection to the shoot kube-apiserver
		// (e.g. due to transient IPv6 connectivity issues) surfaces as a retryable error
		// instead of blocking for the entire parent-context deadline.
		listCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		err := c.List(listCtx, objList, client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(requirement)})
		if err != nil {
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
					return fmt.Errorf("rate limiter wait failed: %w", err)
				}

				if err := c.Patch(ctx, &obj, patch); err != nil {
					return fmt.Errorf("failed to patch object: %w", err)
				}

				return nil
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
	groupResourcesToEncrypt, message, encryptionConfigHasChanged := getGroupResourcesForRewrite(resourcesToEncrypt, encryptedResources)
	encryptedGVKs := sets.New[schema.GroupVersionKind]()

	resourceLists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		failedGroups, isPartialFailure := discovery.GroupDiscoveryFailedErrorGroups(err)
		if !isPartialFailure {
			return encryptedGVKs.UnsortedList(), "", fmt.Errorf("error discovering server preferred resources: %w", err)
		}
		for failedGV := range failedGroups {
			if slices.ContainsFunc(groupResourcesToEncrypt, func(gr schema.GroupResource) bool {
				return gr.Group == failedGV.Group
			}) {
				return encryptedGVKs.UnsortedList(), "", fmt.Errorf("error discovering server preferred resources: %w", err)
			}
		}
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

			if slices.ContainsFunc(groupResourcesToEncrypt, func(gr schema.GroupResource) bool {
				return gr.Group == group && gr.Resource == apiResource.Name
			}) {
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

func getGroupResourcesForRewrite(resourcesToEncrypt, encryptedResources []string) ([]schema.GroupResource, string, bool) {
	resourcesForRewrite := resourcesToEncrypt
	encryptionConfigHasChanged := !sets.New(resourcesToEncrypt...).Equal(sets.New(encryptedResources...))
	message := "Objects requiring to be rewritten after ETCD encryption key rotation"

	if encryptionConfigHasChanged {
		resourcesForRewrite = getModifiedResources(resourcesToEncrypt, encryptedResources)
		message = "Objects requiring to be rewritten after modification of encryption config"
	}

	grs := make([]schema.GroupResource, 0, len(resourcesForRewrite))
	for _, resource := range resourcesForRewrite {
		grs = append(grs, schema.ParseGroupResource(resource))
	}
	return grs, message, encryptionConfigHasChanged
}

func filterServedGroupResources(discoveryClient discovery.DiscoveryInterface, grs []schema.GroupResource) ([]schema.GroupResource, error) {
	_, resourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		failedGroups, isPartialFailure := discovery.GroupDiscoveryFailedErrorGroups(err)
		if !isPartialFailure {
			return nil, fmt.Errorf("error discovering server preferred resources: %w", err)
		}
		for failedGV := range failedGroups {
			if slices.ContainsFunc(grs, func(gr schema.GroupResource) bool {
				return gr.Group == failedGV.Group
			}) {
				return nil, fmt.Errorf("error discovering server preferred resources: %w", err)
			}
		}
	}

	gvrMap, err := discovery.GroupVersionResources(resourceLists)
	if err != nil {
		return nil, fmt.Errorf("error building resource map: %w", err)
	}

	servedGRs := sets.New[schema.GroupResource]()
	for gvr := range gvrMap {
		servedGRs.Insert(gvr.GroupResource())
	}

	return slices.DeleteFunc(slices.Clone(grs), func(gr schema.GroupResource) bool {
		return !servedGRs.Has(gr)
	}), nil
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

// GetStorageVersionMigrationNameForGVK returns the name used for a StorageVersionMigration object for the given GVK.x
func GetStorageVersionMigrationNameForGVK(gvk schema.GroupVersionKind) string {
	if gvk.Group == corev1.SchemeGroupVersion.Group {
		return fmt.Sprintf("rewrite-%s-%s", gvk.Version, gvk.Kind)
	}
	return fmt.Sprintf("rewrite-%s-%s-%s", gvk.Group, gvk.Version, gvk.Kind)
}

// GetStorageVersionMigrationNameForGR returns the name used for a StorageVersionMigration object for the given GroupResource.
// If the generated name exceeds 253 characters, it is truncated and a 5-character SHA256 checksum of the full name is appended.
func GetStorageVersionMigrationNameForGR(gr schema.GroupResource) string {
	const maxNameLength = 63

	var name string
	if gr.Group == "" {
		name = fmt.Sprintf("gardener-rewrite-%s", gr.Resource)
	} else {
		name = fmt.Sprintf("gardener-rewrite-%s-%s", gr.Group, gr.Resource)
	}
	if len(name) <= maxNameLength {
		return name
	}
	return name[:maxNameLength-6] + "-" + utils.ComputeSHA256Hex([]byte(name))[:5]
}
