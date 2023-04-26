// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"strconv"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/kube-aggregator/pkg/controllers/autoregister"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/version"
)

const (
	// Provider is the kubernetes provider label.
	Provider = "provider"
	// KubernetesProvider is the 'kubernetes' value of the Provider label.
	KubernetesProvider = "kubernetes"

	// KubeAggregatorAutoManaged is the label whether an APIService is automanaged by kube-aggregator.
	KubeAggregatorAutoManaged = autoregister.AutoRegisterManagedLabel

	// MetadataNameField ist the `metadata.name` field for a field selector.
	MetadataNameField = "metadata.name"
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

	// CleanupSelector is a selector that excludes system components and all resources not considered for auto cleanup.
	CleanupSelector = labels.NewSelector().Add(NotSystemComponent).Add(NoCleanupPrevention)

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
			Selector: labels.NewSelector().Add(NotKubernetesProvider, NotSystemComponent, NoCleanupPrevention),
		},
	}

	// NamespaceMatchingLabelsSelector is the delete label selector for Namespaces.
	NamespaceMatchingLabelsSelector = utilclient.ListWith{&NoCleanupPreventionListOption}

	// NamespaceMatchingFieldsSelector is the delete field selector for Namespaces.
	NamespaceMatchingFieldsSelector = utilclient.ListWith{
		client.MatchingFieldsSelector{
			Selector: fields.AndSelectors(
				fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespacePublic),
				fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespaceSystem),
				fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespaceDefault),
				fields.OneTermNotEqualSelector(MetadataNameField, corev1.NamespaceNodeLease),
			),
		},
	}

	// APIServiceCleanOption is the delete selector for APIServices.
	APIServiceCleanOption = utilclient.ListWith{
		client.MatchingLabelsSelector{
			Selector: labels.NewSelector().Add(NotSystemComponent, NotKubeAggregatorAutoManaged),
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

	// NamespaceErrorToleration are the errors to be tolerated during deletion.
	NamespaceErrorToleration = utilclient.TolerateErrors{apierrors.IsConflict}
)

type cleanAttributes struct {
	cleanOps     utilclient.CleanOps
	listObj      client.ObjectList
	cleanOptions []utilclient.CleanOption
}

func cleanOpts(opts ...utilclient.CleanOption) []utilclient.CleanOption {
	return opts
}

func cleanResourceFn(cleanOps utilclient.CleanOps, c client.Client, list client.ObjectList, opts ...utilclient.CleanOption) flow.TaskFn {
	return func(ctx context.Context) error {
		return retry.Until(ctx, DefaultInterval, func(ctx context.Context) (done bool, err error) {
			if err := cleanOps.CleanAndEnsureGone(ctx, c, list, opts...); err != nil {
				if utilclient.AreObjectsRemaining(err) {
					return retry.MinorError(helper.NewErrorWithCodes(err, gardencorev1beta1.ErrorCleanupClusterResources))
				}
				return retry.SevereError(err)
			}
			return retry.Ok()
		})
	}
}

func (b *Botanist) clean(ctx context.Context, getAttrs func() ([]cleanAttributes, error)) error {
	attrs, err := getAttrs()
	if err != nil {
		return err
	}

	taskFns := make([]flow.TaskFn, 0, len(attrs))

	for _, attr := range attrs {
		taskFns = append(taskFns, cleanResourceFn(attr.cleanOps, b.ShootClientSet.Client(), attr.listObj, attr.cleanOptions...))
	}

	return flow.Parallel(taskFns...)(ctx)
}

func (b *Botanist) cleanWebhooksAttributes() ([]cleanAttributes, error) {
	var (
		ensurer = utilclient.DefaultGoneEnsurer()
		ops     = utilclient.NewCleanOps(ensurer, utilclient.DefaultCleaner())
	)

	cleanOptions, err := b.getCleanOptions(GracePeriodFiveMinutes, FinalizeAfterFiveMinutes, v1beta1constants.AnnotationShootCleanupWebhooksFinalizeGracePeriodSeconds, 1)
	if err != nil {
		return nil, err
	}

	return []cleanAttributes{
		{ops, &admissionregistrationv1.MutatingWebhookConfigurationList{}, cleanOpts(MutatingWebhookConfigurationCleanOption, cleanOptions)},
		{ops, &admissionregistrationv1.ValidatingWebhookConfigurationList{}, cleanOpts(ValidatingWebhookConfigurationCleanOption, cleanOptions)},
	}, nil
}

func (b *Botanist) cleanExtendedAPIsAttributes() ([]cleanAttributes, error) {
	var (
		ensurer = utilclient.DefaultGoneEnsurer()
		ops     = utilclient.NewCleanOps(ensurer, utilclient.DefaultCleaner())
	)

	cleanOptions, err := b.getCleanOptions(GracePeriodFiveMinutes, FinalizeAfterOneHour, v1beta1constants.AnnotationShootCleanupExtendedAPIsFinalizeGracePeriodSeconds, 0.1)
	if err != nil {
		return nil, err
	}

	return []cleanAttributes{
		{ops, &apiregistrationv1.APIServiceList{}, cleanOpts(APIServiceCleanOption, cleanOptions)},
		{ops, &apiextensionsv1.CustomResourceDefinitionList{}, cleanOpts(CustomResourceDefinitionCleanOption, cleanOptions)},
	}, nil
}

func (b *Botanist) cleanKubernetesResourcesAttributes() ([]cleanAttributes, error) {
	var (
		ensurer            = utilclient.DefaultGoneEnsurer()
		cleaner            = utilclient.DefaultCleaner()
		ops                = utilclient.NewCleanOps(ensurer, cleaner)
		snapshotContentOps = utilclient.NewCleanOps(ensurer, cleaner, utilclient.DefaultVolumeSnapshotContentCleaner())
	)

	cleanOptions, err := b.getCleanOptions(GracePeriodFiveMinutes, FinalizeAfterFiveMinutes, v1beta1constants.AnnotationShootCleanupKubernetesResourcesFinalizeGracePeriodSeconds, 1)
	if err != nil {
		return nil, err
	}

	snapshotCleanOptions, err := b.getCleanOptions(GracePeriodFiveMinutes, FinalizeAfterOneHour, v1beta1constants.AnnotationShootCleanupKubernetesResourcesFinalizeGracePeriodSeconds, 0.5)
	if err != nil {
		return nil, err
	}

	attrs := []cleanAttributes{
		{ops, &corev1.ServiceList{}, cleanOpts(ServiceCleanOption, cleanOptions)},
		{ops, &corev1.PersistentVolumeClaimList{}, cleanOpts(PersistentVolumeClaimCleanOption, cleanOptions)},
	}

	if metav1.HasAnnotation(b.Shoot.GetInfo().ObjectMeta, v1beta1constants.AnnotationShootSkipCleanup) {
		attrs = append(attrs, cleanAttributes{ops, &volumesnapshotv1.VolumeSnapshotList{}, cleanOpts(VolumeSnapshotContentCleanOption, cleanOptions)})
		attrs = append(attrs, cleanAttributes{ops, &volumesnapshotv1.VolumeSnapshotContentList{}, cleanOpts(VolumeSnapshotContentCleanOption, cleanOptions)})
	} else {
		cronJobList := client.ObjectList(&batchv1beta1.CronJobList{})
		if version.ConstraintK8sGreaterEqual121.Check(b.Shoot.KubernetesVersion) {
			cronJobList = &batchv1.CronJobList{}
		}

		attrs = append(attrs, cleanAttributes{ops, cronJobList, cleanOpts(CronJobCleanOption, cleanOptions)})
		attrs = append(attrs, cleanAttributes{ops, &appsv1.DaemonSetList{}, cleanOpts(DaemonSetCleanOption, cleanOptions)})
		attrs = append(attrs, cleanAttributes{ops, &appsv1.DeploymentList{}, cleanOpts(DeploymentCleanOption, cleanOptions)})
		attrs = append(attrs, cleanAttributes{ops, &networkingv1.IngressList{}, cleanOpts(IngressCleanOption, cleanOptions)})
		attrs = append(attrs, cleanAttributes{ops, &batchv1.JobList{}, cleanOpts(JobCleanOption, cleanOptions)})
		attrs = append(attrs, cleanAttributes{ops, &corev1.PodList{}, cleanOpts(PodCleanOption, cleanOptions)})
		attrs = append(attrs, cleanAttributes{ops, &appsv1.ReplicaSetList{}, cleanOpts(ReplicaSetCleanOption, cleanOptions)})
		attrs = append(attrs, cleanAttributes{ops, &corev1.ReplicationControllerList{}, cleanOpts(ReplicationControllerCleanOption, cleanOptions)})
		attrs = append(attrs, cleanAttributes{ops, &appsv1.StatefulSetList{}, cleanOpts(StatefulSetCleanOption, cleanOptions)})
		// Cleaning up VolumeSnapshots can take a longer time if many snapshots were taken.
		// Hence, we only finalize these objects after 1h.
		attrs = append(attrs, cleanAttributes{ops, &volumesnapshotv1.VolumeSnapshotList{}, cleanOpts(VolumeSnapshotContentCleanOption, snapshotCleanOptions)})
		attrs = append(attrs, cleanAttributes{snapshotContentOps, &volumesnapshotv1.VolumeSnapshotContentList{}, cleanOpts(VolumeSnapshotContentCleanOption, snapshotCleanOptions)})
	}

	return attrs, nil
}

// CleanWebhooks deletes all Webhooks in the Shoot cluster that are not being managed by the addon manager.
func (b *Botanist) CleanWebhooks(ctx context.Context) error {
	return b.clean(ctx, b.cleanWebhooksAttributes)
}

// CleanExtendedAPIs removes API extensions like CRDs and API services from the Shoot cluster.
func (b *Botanist) CleanExtendedAPIs(ctx context.Context) error {
	return b.clean(ctx, b.cleanExtendedAPIsAttributes)
}

// CleanKubernetesResources deletes all the Kubernetes resources in the Shoot cluster
// other than those stored in the exceptions map. It will check whether all the Kubernetes resources
// in the Shoot cluster other than those stored in the exceptions map have been deleted.
// It will return an error in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CleanKubernetesResources(ctx context.Context) error {
	return b.clean(ctx, b.cleanKubernetesResourcesAttributes)
}

// CleanShootNamespaces deletes all non-system namespaces in the Shoot cluster.
// It assumes that all workload resources are cleaned up in previous step(s).
func (b *Botanist) CleanShootNamespaces(ctx context.Context) error {
	var (
		c                 = b.ShootClientSet.Client()
		namespaceCleaner  = utilclient.NewNamespaceCleaner()
		namespaceCleanOps = utilclient.NewCleanOps(utilclient.DefaultGoneEnsurer(), namespaceCleaner)
	)

	cleanOptions, err := b.getCleanOptions(ZeroGracePeriod, FinalizeAfterFiveMinutes, v1beta1constants.AnnotationShootCleanupNamespaceResourcesFinalizeGracePeriodSeconds, 0)
	if err != nil {
		return err
	}

	return cleanResourceFn(namespaceCleanOps, c, &corev1.NamespaceList{}, cleanOptions, NamespaceMatchingLabelsSelector, NamespaceMatchingFieldsSelector, NamespaceErrorToleration)(ctx)
}

// CleanVolumeAttachments cleans up all VolumeAttachments in the cluster, waits for them to be gone and finalizes any
// remaining ones after five minutes.
func CleanVolumeAttachments(ctx context.Context, c client.Client) error {
	return cleanResourceFn(utilclient.DefaultCleanOps(), c, &storagev1.VolumeAttachmentList{}, utilclient.DeleteWith{ZeroGracePeriod}, FinalizeAfterFiveMinutes)(ctx)
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
