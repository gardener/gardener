// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	"github.com/gardener/gardener/pkg/utils/retry"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/kube-aggregator/pkg/controllers/autoregister"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// ZeroGracePeriod is an option to delete resources with no grace period.
	ZeroGracePeriod = utilclient.DeleteWith{client.GracePeriodSeconds(0)}
	// GracePeriodFiveMinutes is an option to delete resources with a grace period of five minutes.
	GracePeriodFiveMinutes = utilclient.DeleteWith{client.GracePeriodSeconds(5 * 60)}

	// NotSystemComponent is a requirement that something doesn't have the GardenRole GardenRoleSystemComponent.
	NotSystemComponent = utils.MustNewRequirement(v1beta1constants.DeprecatedGardenRole, selection.NotEquals, v1beta1constants.GardenRoleSystemComponent)
	// NoCleanupPrevention is a requirement that the ShootNoCleanup label of something is not true.
	NoCleanupPrevention = utils.MustNewRequirement(common.ShootNoCleanup, selection.NotEquals, "true")
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

	// NamespaceErrorToleration are the errors to be tolerated during deletion.
	NamespaceErrorToleration = utilclient.TolerateErrors{apierrors.IsConflict}
)

func cleanResourceFn(cleanOps utilclient.CleanOps, c client.Client, list runtime.Object, opts ...utilclient.CleanOption) flow.TaskFn {
	return func(ctx context.Context) error {
		return retry.Until(ctx, DefaultInterval, func(ctx context.Context) (done bool, err error) {
			if err := cleanOps.CleanAndEnsureGone(ctx, c, list, opts...); err != nil {
				if utilclient.AreObjectsRemaining(err) {
					return retry.MinorError(helper.NewErrorWithCodes(err.Error(), gardencorev1beta1.ErrorCleanupClusterResources))
				}
				return retry.SevereError(err)
			}
			return retry.Ok()
		})
	}
}

// CleanWebhooks deletes all Webhooks in the Shoot cluster that are not being managed by the addon manager.
func (b *Botanist) CleanWebhooks(ctx context.Context) error {
	var (
		c       = b.K8sShootClient.Client()
		ensurer = utilclient.GoneBeforeEnsurer(b.Shoot.Info.GetDeletionTimestamp().Time)
		ops     = utilclient.NewCleanOps(utilclient.DefaultCleaner(), ensurer)
	)

	return flow.Parallel(
		cleanResourceFn(ops, c, &admissionregistrationv1beta1.MutatingWebhookConfigurationList{}, MutatingWebhookConfigurationCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &admissionregistrationv1beta1.ValidatingWebhookConfigurationList{}, ValidatingWebhookConfigurationCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
	)(ctx)
}

// CleanExtendedAPIs removes API extensions like CRDs and API services from the Shoot cluster.
func (b *Botanist) CleanExtendedAPIs(ctx context.Context) error {
	var (
		c           = b.K8sShootClient.Client()
		ensurer     = utilclient.GoneBeforeEnsurer(b.Shoot.Info.GetDeletionTimestamp().Time)
		defaultOps  = utilclient.NewCleanOps(utilclient.DefaultCleaner(), ensurer)
		crdCleaner  = utilclient.NewCRDCleaner()
		crdCleanOps = utilclient.NewCleanOps(crdCleaner, ensurer)
	)

	return flow.Parallel(
		cleanResourceFn(defaultOps, c, &apiregistrationv1.APIServiceList{}, APIServiceCleanOption, GracePeriodFiveMinutes, FinalizeAfterOneHour),
		cleanResourceFn(crdCleanOps, c, &apiextensionsv1beta1.CustomResourceDefinitionList{}, CustomResourceDefinitionCleanOption, GracePeriodFiveMinutes, FinalizeAfterOneHour),
	)(ctx)
}

// CleanKubernetesResources deletes all the Kubernetes resources in the Shoot cluster
// other than those stored in the exceptions map. It will check whether all the Kubernetes resources
// in the Shoot cluster other than those stored in the exceptions map have been deleted.
// It will return an error in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CleanKubernetesResources(ctx context.Context) error {
	var (
		c       = b.K8sShootClient.DirectClient()
		ensurer = utilclient.GoneBeforeEnsurer(b.Shoot.Info.GetDeletionTimestamp().Time)
		ops     = utilclient.NewCleanOps(utilclient.DefaultCleaner(), ensurer)
	)

	if metav1.HasAnnotation(b.Shoot.Info.ObjectMeta, v1beta1constants.AnnotationShootSkipCleanup) {
		return flow.Parallel(
			cleanResourceFn(ops, c, &corev1.ServiceList{}, ServiceCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
			cleanResourceFn(ops, c, &corev1.PersistentVolumeClaimList{}, PersistentVolumeClaimCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		)(ctx)
	}

	return flow.Parallel(
		cleanResourceFn(ops, c, &batchv1beta1.CronJobList{}, CronJobCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &appsv1.DaemonSetList{}, DaemonSetCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &appsv1.DeploymentList{}, DeploymentCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &extensionsv1beta1.IngressList{}, IngressCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &batchv1.JobList{}, JobCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &corev1.PodList{}, PodCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &appsv1.ReplicaSetList{}, ReplicaSetCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &corev1.ReplicationControllerList{}, ReplicationControllerCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &corev1.ServiceList{}, ServiceCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &appsv1.StatefulSetList{}, StatefulSetCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &corev1.PersistentVolumeClaimList{}, PersistentVolumeClaimCleanOption, GracePeriodFiveMinutes, FinalizeAfterFiveMinutes),
	)(ctx)
}

// CleanShootNamespaces deletes all non-system namespaces in the Shoot cluster.
// It assumes that all workload resources are cleaned up in previous step(s).
func (b *Botanist) CleanShootNamespaces(ctx context.Context) error {
	var (
		// use direct client here, as cached client does not support field selector with multiple requirements
		// see https://github.com/kubernetes-sigs/controller-runtime/blob/ca25c1f1014d6db6eba745e04f180d272a854e9a/pkg/cache/internal/cache_reader.go#L98-L103
		c                 = b.K8sShootClient.DirectClient()
		namespaceCleaner  = utilclient.NewNamespaceCleaner(b.K8sShootClient.Kubernetes().CoreV1().Namespaces())
		namespaceCleanOps = utilclient.NewCleanOps(namespaceCleaner, utilclient.DefaultGoneEnsurer())
	)

	return cleanResourceFn(namespaceCleanOps, c, &corev1.NamespaceList{}, NamespaceMatchingLabelsSelector, NamespaceMatchingFieldsSelector, ZeroGracePeriod, FinalizeAfterFiveMinutes, NamespaceErrorToleration)(ctx)
}
