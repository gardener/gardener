// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/kube-aggregator/pkg/controllers/autoregister"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	"github.com/gardener/gardener/pkg/utils/retry"
)

const (
	// Provider is the kubernetes provider label.
	Provider = "provider"
	// KubernetesProvider is the 'kubernetes' value of the Provider label.
	KubernetesProvider = "kubernetes"

	// KubeAggregatorAutoManaged is the label whether an APIService is automanaged by kube-aggregator.
	KubeAggregatorAutoManaged = autoregister.AutoRegisterManagedLabel

	// MetadataNameField is the `metadata.name` field for a field selector.
	MetadataNameField = "metadata.name"
	// MetadataNamespaceField is the `metadata.namespace` field for a field selector.
	MetadataNamespaceField = "metadata.namespace"
)

var (
	// FinalizeAfterFiveMinutes is an option to finalize resources after five minutes.
	FinalizeAfterFiveMinutes = utilclient.FinalizeGracePeriodSeconds(5 * 60)
	// FinalizeAfterOneHour is an option to finalize resources after one hour.
	FinalizeAfterOneHour = utilclient.FinalizeGracePeriodSeconds(60 * 60)

	// ZeroGracePeriod can be used for deleting resources with no grace period.
	ZeroGracePeriod = client.GracePeriodSeconds(0)
	// GracePeriodFiveMinutes can be used for deleting resources with a grace period of five minutes.
	GracePeriodFiveMinutes = client.GracePeriodSeconds(5 * 60)

	// NotSystemComponent is a requirement that something doesn't have the GardenRole GardenRoleSystemComponent.
	NotSystemComponent = utils.MustNewRequirement(v1beta1constants.GardenRole, selection.NotEquals, v1beta1constants.GardenRoleSystemComponent)
	// NoCleanupPrevention is a requirement that the ShootNoCleanup label of something is not true.
	NoCleanupPrevention = utils.MustNewRequirement(v1beta1constants.ShootNoCleanup, selection.NotEquals, "true")
	// NotKubernetesProvider is a requirement that the Provider label of something is not KubernetesProvider.
	NotKubernetesProvider = utils.MustNewRequirement(Provider, selection.NotEquals, KubernetesProvider)
	// NotKubeAggregatorAutoManaged is a requirement that something is not auto-managed by Kube-Aggregator.
	NotKubeAggregatorAutoManaged = utils.MustNewRequirement(KubeAggregatorAutoManaged, selection.DoesNotExist)
	// NotManagedByGardener is a requirement that something is not managed via ManagedResources.
	NotManagedByGardener = utils.MustNewRequirement(resourcesv1alpha1.ManagedBy, selection.NotEquals, resourcesv1alpha1.GardenerManager)

	// CleanupSelector is a selector that excludes system components, all resources not considered for auto cleanup and all components not managed via ManagedResources.
	CleanupSelector = labels.NewSelector().Add(NotSystemComponent).Add(NoCleanupPrevention).Add(NotManagedByGardener)

	// NoCleanupPreventionListOption are CollectionMatching that exclude system components or non-auto cleaned up resource.
	NoCleanupPreventionListOption = client.MatchingLabelsSelector{Selector: CleanupSelector}

	// MutatingWebhookConfigurationCleanOption is the delete selector for MutatingWebhookConfigurations.
	MutatingWebhookConfigurationCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// ValidatingWebhookConfigurationCleanOption is the delete selector for ValidatingWebhookConfigurations.
	ValidatingWebhookConfigurationCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// CustomResourceDefinitionCleanOption is the delete selector for CustomResources.
	CustomResourceDefinitionCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// DaemonSetCleanOption is the delete selector for DaemonSets.
	DaemonSetCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// DeploymentCleanOption is the delete selector for Deployments.
	DeploymentCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// StatefulSetCleanOption is the delete selector for StatefulSets.
	StatefulSetCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// ServiceCleanOption is the delete selector for Services.
	ServiceCleanOption = utilclient.ListWith{
		client.MatchingLabelsSelector{
			Selector: labels.NewSelector().Add(NotKubernetesProvider, NotSystemComponent, NoCleanupPrevention, NotManagedByGardener),
		},
	}

	// APIServiceCleanOption is the delete selector for APIServices.
	APIServiceCleanOption = utilclient.ListWith{
		client.MatchingLabelsSelector{
			Selector: labels.NewSelector().Add(NotSystemComponent, NotKubeAggregatorAutoManaged, NotManagedByGardener),
		},
	}

	// CronJobCleanOption is the delete selector for CronJobs.
	CronJobCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// IngressCleanOption is the delete selector for Ingresses.
	IngressCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// JobCleanOption is the delete selector for Jobs.
	JobCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// PodCleanOption is the delete selector for Pods.
	PodCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// ReplicaSetCleanOption is the delete selector for ReplicaSets.
	ReplicaSetCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// ReplicationControllerCleanOption is the delete selector for ReplicationControllers.
	ReplicationControllerCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// PersistentVolumeClaimCleanOption is the delete selector for PersistentVolumeClaims.
	PersistentVolumeClaimCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// VolumeSnapshotCleanOption is the delete selector for VolumeSnapshots.
	VolumeSnapshotCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}

	// VolumeSnapshotContentCleanOption is the delete selector for VolumeSnapshotContents.
	VolumeSnapshotContentCleanOption = utilclient.ListWith{&NoCleanupPreventionListOption}
)

func cleanResourceFn(cleanOps utilclient.CleanOps, log logr.Logger, c client.Client, list client.ObjectList, opts ...utilclient.CleanOption) flow.TaskFn {
	return func(ctx context.Context) error {
		return retry.Until(ctx, DefaultInterval, func(ctx context.Context) (done bool, err error) {
			if err := cleanOps.CleanAndEnsureGone(ctx, log, c, list, opts...); err != nil {
				if utilclient.AreObjectsRemaining(err) {
					return retry.MinorError(helper.NewErrorWithCodes(err, gardencorev1beta1.ErrorCleanupClusterResources))
				}

				gvk, gvkErr := apiutil.GVKForObject(list, c.Scheme())
				if gvkErr != nil {
					return retry.SevereError(fmt.Errorf("could not get GroupVersionKind from object %v: %w", list, gvkErr))
				}
				gvk.Kind = strings.TrimSuffix(gvk.Kind, "List")

				return retry.SevereError(fmt.Errorf("failed to clean up resources with kind %s: %w", gvk.Kind, err))
			}
			return retry.Ok()
		})
	}
}

// CleanWebhooks deletes all Webhooks in the Shoot cluster that are not being managed by the addon manager.
func (b *Botanist) CleanWebhooks(ctx context.Context) error {
	var (
		c       = b.ShootClientSet.Client()
		ensurer = utilclient.DefaultGoneEnsurer()
		ops     = utilclient.NewCleanOps(ensurer, utilclient.DefaultCleaner())
	)

	cleanOptions, err := b.getCleanOptions(GracePeriodFiveMinutes, FinalizeAfterFiveMinutes, v1beta1constants.AnnotationShootCleanupWebhooksFinalizeGracePeriodSeconds, 1)
	if err != nil {
		return err
	}

	controlPlaneNamespaces, err := b.selfHostedControlPlaneNamespaces(ctx)
	if err != nil {
		return err
	}

	mutatingWebhookConfigurationOption, err := skipServedFromNamespaces(ctx, c, controlPlaneNamespaces, &admissionregistrationv1.MutatingWebhookConfigurationList{}, func(obj client.Object) []*admissionregistrationv1.ServiceReference {
		var services []*admissionregistrationv1.ServiceReference
		for _, webhook := range obj.(*admissionregistrationv1.MutatingWebhookConfiguration).Webhooks {
			services = append(services, webhook.ClientConfig.Service)
		}
		return services
	})
	if err != nil {
		return err
	}

	validatingWebhookConfigurationOption, err := skipServedFromNamespaces(ctx, c, controlPlaneNamespaces, &admissionregistrationv1.ValidatingWebhookConfigurationList{}, func(obj client.Object) []*admissionregistrationv1.ServiceReference {
		var services []*admissionregistrationv1.ServiceReference
		for _, webhook := range obj.(*admissionregistrationv1.ValidatingWebhookConfiguration).Webhooks {
			services = append(services, webhook.ClientConfig.Service)
		}
		return services
	})
	if err != nil {
		return err
	}

	return flow.Parallel(
		cleanResourceFn(ops, b.Logger, c, &admissionregistrationv1.MutatingWebhookConfigurationList{}, MutatingWebhookConfigurationCleanOption, mutatingWebhookConfigurationOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &admissionregistrationv1.ValidatingWebhookConfigurationList{}, ValidatingWebhookConfigurationCleanOption, validatingWebhookConfigurationOption, cleanOptions),
	)(ctx)
}

// CleanExtendedAPIs removes API extensions like CRDs and API services from the Shoot cluster.
func (b *Botanist) CleanExtendedAPIs(ctx context.Context) error {
	var (
		c       = b.ShootClientSet.Client()
		ensurer = utilclient.DefaultGoneEnsurer()
		ops     = utilclient.NewCleanOps(ensurer, utilclient.DefaultCleaner())
	)

	cleanOptions, err := b.getCleanOptions(GracePeriodFiveMinutes, FinalizeAfterOneHour, v1beta1constants.AnnotationShootCleanupExtendedAPIsFinalizeGracePeriodSeconds, 0.1)
	if err != nil {
		return err
	}

	controlPlaneNamespaces, err := b.selfHostedControlPlaneNamespaces(ctx)
	if err != nil {
		return err
	}

	apiServiceOption, err := skipServedFromNamespaces(ctx, c, controlPlaneNamespaces, &apiregistrationv1.APIServiceList{}, func(obj client.Object) []*admissionregistrationv1.ServiceReference {
		if service := obj.(*apiregistrationv1.APIService).Spec.Service; service != nil {
			return []*admissionregistrationv1.ServiceReference{{Namespace: service.Namespace, Name: service.Name}}
		}
		return nil
	})
	if err != nil {
		return err
	}

	customResourceDefinitionCleanOption := CustomResourceDefinitionCleanOption
	if b.Shoot.IsSelfHosted() {
		// The CRDs of the control plane components of self-hosted shoots (e.g., extensions, machines, etcd) are
		// protected against deletion. Deleting them would delete the resources of the control plane.
		customResourceDefinitionCleanOption = utilclient.ListWith{client.MatchingLabelsSelector{
			Selector: CleanupSelector.DeepCopySelector().Add(utils.MustNewRequirement(gardenerutils.DeletionProtected, selection.NotEquals, "true")),
		}}
	}

	return flow.Parallel(
		cleanResourceFn(ops, b.Logger, c, &apiregistrationv1.APIServiceList{}, APIServiceCleanOption, apiServiceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &apiextensionsv1.CustomResourceDefinitionList{}, customResourceDefinitionCleanOption, cleanOptions),
	)(ctx)
}

// CleanKubernetesResources deletes all the Kubernetes resources in the Shoot cluster
// other than those stored in the exceptions map. It will check whether all the Kubernetes resources
// in the Shoot cluster other than those stored in the exceptions map have been deleted.
// It will return an error in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CleanKubernetesResources(ctx context.Context) error {
	var (
		c                  = b.ShootClientSet.Client()
		ensurer            = utilclient.DefaultGoneEnsurer()
		cleaner            = utilclient.DefaultCleaner()
		ops                = utilclient.NewCleanOps(ensurer, cleaner)
		snapshotContentOps = utilclient.NewCleanOps(ensurer, cleaner, utilclient.DefaultVolumeSnapshotContentCleaner())
	)

	cleanOptions, err := b.getCleanOptions(GracePeriodFiveMinutes, FinalizeAfterFiveMinutes, v1beta1constants.AnnotationShootCleanupKubernetesResourcesFinalizeGracePeriodSeconds, 1)
	if err != nil {
		return err
	}

	cleanupStartTime, err := b.addAndGetCleanupStartTime(ctx)
	if err != nil {
		return fmt.Errorf("failed to handle cleanup start time: %w", err)
	}

	if err := b.addAdditionalCleanOptionsForKubernetesCleanup(ctx, cleanOptions, cleanupStartTime, FinalizeAfterFiveMinutes, v1beta1constants.AnnotationShootCleanupKubernetesResourcesFinalizeGracePeriodSeconds); err != nil {
		return fmt.Errorf("failed to add additional clean options %w", err)
	}

	snapshotCleanOptions, err := b.getCleanOptions(GracePeriodFiveMinutes, FinalizeAfterOneHour, v1beta1constants.AnnotationShootCleanupKubernetesResourcesFinalizeGracePeriodSeconds, 0.5)
	if err != nil {
		return err
	}

	controlPlaneNamespaces, err := b.selfHostedControlPlaneNamespaces(ctx)
	if err != nil {
		return err
	}
	// The option is only passed for namespaced resources, as cluster-scoped resources don't support the namespace field
	// selector.
	namespaceOption := skipNamespaces(controlPlaneNamespaces)

	if metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.AnnotationShootSkipCleanup) {
		return flow.Parallel(
			cleanResourceFn(ops, b.Logger, c, &corev1.ServiceList{}, ServiceCleanOption, namespaceOption, cleanOptions),
			cleanResourceFn(ops, b.Logger, c, &corev1.PersistentVolumeClaimList{}, PersistentVolumeClaimCleanOption, namespaceOption, cleanOptions),
			cleanResourceFn(ops, b.Logger, c, &volumesnapshotv1.VolumeSnapshotList{}, VolumeSnapshotContentCleanOption, namespaceOption, cleanOptions),
			cleanResourceFn(ops, b.Logger, c, &volumesnapshotv1.VolumeSnapshotContentList{}, VolumeSnapshotContentCleanOption, cleanOptions),
		)(ctx)
	}

	return flow.Parallel(
		cleanResourceFn(ops, b.Logger, c, &batchv1.CronJobList{}, CronJobCleanOption, namespaceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &appsv1.DaemonSetList{}, DaemonSetCleanOption, namespaceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &appsv1.DeploymentList{}, DeploymentCleanOption, namespaceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &networkingv1.IngressList{}, IngressCleanOption, namespaceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &batchv1.JobList{}, JobCleanOption, namespaceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &corev1.PodList{}, PodCleanOption, namespaceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &appsv1.ReplicaSetList{}, ReplicaSetCleanOption, namespaceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &corev1.ReplicationControllerList{}, ReplicationControllerCleanOption, namespaceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &corev1.ServiceList{}, ServiceCleanOption, namespaceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &appsv1.StatefulSetList{}, StatefulSetCleanOption, namespaceOption, cleanOptions),
		cleanResourceFn(ops, b.Logger, c, &corev1.PersistentVolumeClaimList{}, PersistentVolumeClaimCleanOption, namespaceOption, cleanOptions),
		// Cleaning up VolumeSnapshots can take a longer time if many snapshots were taken.
		// Hence, we only finalize these objects after 1h.
		cleanResourceFn(ops, b.Logger, c, &volumesnapshotv1.VolumeSnapshotList{}, VolumeSnapshotContentCleanOption, namespaceOption, snapshotCleanOptions),
		cleanResourceFn(snapshotContentOps, b.Logger, c, &volumesnapshotv1.VolumeSnapshotContentList{}, VolumeSnapshotContentCleanOption, snapshotCleanOptions),
	)(ctx)
}

// selfHostedControlPlaneNamespaces returns the namespaces of the control plane components of self-hosted shoots, i.e.,
// kube-system, garden (e.g., gardener-resource-manager, etcd-druid), and the namespaces of the extensions. It returns
// nil for hosted shoots.
// Unlike the system components of hosted shoots, the control plane components of self-hosted shoots (e.g., the static
// pods, gardener-resource-manager, and the components deployed via ManagedResources for the seed like
// cloud-controller-manager) are not excluded from the cleanup by labels. Hence, the cleanup skips the resources in and
// served from these namespaces. Otherwise, it would delete the control plane components, which are recreated
// immediately, so that the cleanup never finishes.
func (b *Botanist) selfHostedControlPlaneNamespaces(ctx context.Context) (sets.Set[string], error) {
	if !b.Shoot.IsSelfHosted() {
		return nil, nil
	}

	namespaceList := &corev1.NamespaceList{}
	if err := b.ShootClientSet.Client().List(ctx, namespaceList, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension}); err != nil {
		return nil, fmt.Errorf("could not list extension namespaces: %w", err)
	}

	namespaces := sets.New(metav1.NamespaceSystem, v1beta1constants.GardenNamespace)
	for _, namespace := range namespaceList.Items {
		namespaces.Insert(namespace.Name)
	}

	return namespaces, nil
}

// skipNamespaces returns a clean option that skips the namespaced resources in the given namespaces.
func skipNamespaces(namespaces sets.Set[string]) utilclient.ListWith {
	if namespaces.Len() == 0 {
		return nil
	}

	var selectors []fields.Selector
	for _, namespace := range sets.List(namespaces) {
		selectors = append(selectors, fields.OneTermNotEqualSelector(MetadataNamespaceField, namespace))
	}

	return utilclient.ListWith{client.MatchingFieldsSelector{Selector: fields.AndSelectors(selectors...)}}
}

// skipServedFromNamespaces returns a clean option that skips the objects of the given list (e.g., webhook
// configurations) which are served by a service in one of the given namespaces.
func skipServedFromNamespaces(ctx context.Context, c client.Client, namespaces sets.Set[string], list client.ObjectList, servicesOf func(client.Object) []*admissionregistrationv1.ServiceReference) (utilclient.ListWith, error) {
	if namespaces.Len() == 0 {
		return nil, nil
	}

	if err := c.List(ctx, list); err != nil {
		return nil, fmt.Errorf("could not list %T: %w", list, err)
	}

	var selectors []fields.Selector
	if err := meta.EachListItem(list, func(o runtime.Object) error {
		obj := o.(client.Object)
		for _, service := range servicesOf(obj) {
			if service != nil && namespaces.Has(service.Namespace) {
				selectors = append(selectors, fields.OneTermNotEqualSelector(MetadataNameField, obj.GetName()))
				return nil
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if len(selectors) == 0 {
		return nil, nil
	}

	return utilclient.ListWith{client.MatchingFieldsSelector{Selector: fields.AndSelectors(selectors...)}}, nil
}

// addAndGetCleanupStartTime adds the cleanup start time annotation to the shoot namespace if not already present
// and returns the timestamp.
func (b *Botanist) addAndGetCleanupStartTime(ctx context.Context) (*time.Time, error) {
	namespace := b.SeedNamespaceObject.DeepCopy()
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
		return nil, fmt.Errorf("failed to get shoot namespace %s: %w", b.Shoot.ControlPlaneNamespace, err)
	}

	cleanupStartTime := b.Clock.Now().UTC()
	if !metav1.HasAnnotation(namespace.ObjectMeta, v1beta1constants.AnnotationShootCleanupKubernetesResourcesStartTime) {
		patch := client.MergeFrom(namespace.DeepCopy())
		metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, v1beta1constants.AnnotationShootCleanupKubernetesResourcesStartTime, cleanupStartTime.Format(time.RFC3339))

		if err := b.SeedClientSet.Client().Patch(ctx, namespace, patch); err != nil {
			return nil, fmt.Errorf("failed to record cleanup start time in shoot namespace: %w", err)
		}
	} else {
		startTimeStr := namespace.Annotations[v1beta1constants.AnnotationShootCleanupKubernetesResourcesStartTime]
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cleanup start time %s from shoot namespace: %w", startTimeStr, err)
		}
		cleanupStartTime = startTime
	}

	return &cleanupStartTime, nil
}

// CleanVolumeAttachments cleans up all VolumeAttachments in the cluster, waits for them to be gone and finalizes any
// remaining ones after five minutes.
func CleanVolumeAttachments(ctx context.Context, log logr.Logger, c client.Client) error {
	return cleanResourceFn(utilclient.DefaultCleanOps(), log, c, &storagev1.VolumeAttachmentList{}, utilclient.DeleteWith{ZeroGracePeriod}, FinalizeAfterFiveMinutes)(ctx)
}

func (b *Botanist) addAdditionalCleanOptionsForKubernetesCleanup(
	ctx context.Context,
	cleanOptions *utilclient.CleanOptions,
	cleanupStartTime *time.Time,
	defaultFinalizeAfter utilclient.FinalizeGracePeriodSeconds,
	customFinalizeAfterAnnotation string,
) error {
	// Kubernetes objects in unknown namespaces should be ignored.
	// Finalizing such objects will fail and block the cleanup process indefinitely.
	// See https://github.com/gardener/gardener/issues/13847 for more information.
	namespaceList := &corev1.NamespaceList{}
	if err := b.ShootClientSet.Client().List(ctx, namespaceList); err != nil {
		return fmt.Errorf("could not list namespaces: %w", err)
	}
	namespaces := make([]string, 0, len(namespaceList.Items))
	for _, ns := range namespaceList.Items {
		namespaces = append(namespaces, ns.Name)
	}

	cleanOptions.IgnoreLeftovers = append(cleanOptions.IgnoreLeftovers,
		utilclient.IgnoreUnknownNamespaces(namespaces, kubernetes.ShootScheme),
	)

	// Kubernetes objects created after the cleanup started + finalize time should be ignored.
	finalizeSeconds := int64(defaultFinalizeAfter)
	if v, ok := b.Shoot.GetInfo().Annotations[customFinalizeAfterAnnotation]; ok {
		seconds, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("could not parse finalize seconds from annotation %s: %w", customFinalizeAfterAnnotation, err)
		}
		finalizeSeconds = int64(seconds)
	}

	cleanOptions.IgnoreLeftovers = append(cleanOptions.IgnoreLeftovers,
		utilclient.IgnoreObjectsCreatedAfter(cleanupStartTime.Add(time.Duration(finalizeSeconds)*time.Second), kubernetes.ShootScheme),
	)

	return nil
}

func (b *Botanist) getCleanOptions(
	defaultGracePeriodSeconds client.GracePeriodSeconds,
	defaultFinalizeAfter utilclient.FinalizeGracePeriodSeconds,
	annotationKey string,
	gracePeriodSecondsFactor float64,
) (
	*utilclient.CleanOptions,
	error,
) {
	var (
		gracePeriodSeconds = defaultGracePeriodSeconds
		finalizeAfter      = defaultFinalizeAfter
	)

	if v, ok := b.Shoot.GetInfo().Annotations[annotationKey]; ok {
		seconds, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}

		if int64(seconds) < int64(defaultFinalizeAfter) {
			gracePeriodSeconds = client.GracePeriodSeconds(int(float64(seconds) * gracePeriodSecondsFactor))
			finalizeAfter = utilclient.FinalizeGracePeriodSeconds(seconds)
		}
	}

	cleanOpts := &utilclient.CleanOptions{}
	utilclient.DeleteWith{gracePeriodSeconds}.ApplyToClean(cleanOpts)
	finalizeAfter.ApplyToClean(cleanOpts)

	return cleanOpts, nil
}
