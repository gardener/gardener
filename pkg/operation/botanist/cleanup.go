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
	"time"

	"github.com/gardener/gardener/pkg/utils/retry"

	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"

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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	"k8s.io/kube-aggregator/pkg/controllers/autoregister"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultInterval is the default interval for retry operations.
	DefaultInterval = 5 * time.Second

	// Provider is the kubernetes provider label.
	Provider = "provider"
	// KubernetesProvider is the 'kubernetes' value of the Provider label.
	KubernetesProvider = "kubernetes"

	// KubeAggregatorAutoManaged is the label whether an APIService is automanaged by kube-aggregator.
	KubeAggregatorAutoManaged = autoregister.AutoRegisterManagedLabel

	// MetadataNameField ist the `metadata.name` field for a field selector.
	MetadataNameField = "metadata.name"
)

// MustNewRequirement creates a labels.Requirement with the given values and panics if there is an error.
func MustNewRequirement(key string, op selection.Operator, vals ...string) labels.Requirement {
	req, err := labels.NewRequirement(key, op, vals)
	utilruntime.Must(err)
	return *req
}

var (
	// FinalizeAfterFiveMinutes is an option to finalize resources after five minutes.
	FinalizeAfterFiveMinutes = utilclient.FinalizeGracePeriodSeconds(5 * 60)

	// FinalizeAfterOneHour is an option to finalize resources after one hour.
	FinalizeAfterOneHour = utilclient.FinalizeGracePeriodSeconds(60 * 60)

	// ZeroGracePeriod is an option to delete resources with no grace period.
	ZeroGracePeriod = utilclient.DeleteWith(utilclient.GracePeriodSeconds(0))

	// NotSystemComponent is a requirement that something doesn't have the GardenRole GardenRoleSystemComponent.
	NotSystemComponent = MustNewRequirement(common.GardenRole, selection.NotEquals, common.GardenRoleSystemComponent)
	// NoCleanupPrevention is a requirement that the ShootNoCleanup label of something is not true.
	NoCleanupPrevention = MustNewRequirement(common.ShootNoCleanup, selection.NotEquals, "true")
	// NotKubernetesProvider is a requirement that the Provider label of something is not KubernetesProvider.
	NotKubernetesProvider = MustNewRequirement(Provider, selection.NotEquals, KubernetesProvider)
	// NotKubeAggregatorAutoManaged is a requirement that something is not auto-managed by Kube-Aggregator.
	NotKubeAggregatorAutoManaged = MustNewRequirement(KubeAggregatorAutoManaged, selection.DoesNotExist)

	// CleanupSelector is a selector that excludes system components and all resources not considered for auto cleanup.
	CleanupSelector = labels.NewSelector().Add(NotSystemComponent).Add(NoCleanupPrevention)

	// NoCleanupPreventionListOptions are CollectionMatching that exclude system components or non-auto clean upped resource.
	NoCleanupPreventionListOptions = client.ListOptions{
		LabelSelector: CleanupSelector,
	}

	// MutatingWebhookConfigurationCleanOptions is the delete selector for MutatingWebhookConfigurations.
	MutatingWebhookConfigurationCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// ValidatingWebhookConfigurationCleanOptions is the delete selector for ValidatingWebhookConfigurations.
	ValidatingWebhookConfigurationCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// CustomResourceDefinitionCleanOptions is the delete selector for CustomResources.
	CustomResourceDefinitionCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// DaemonSetCleanOptions is the delete selector for DaemonSets.
	DaemonSetCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// DeploymentCleanOptions is the delete selector for Deployments.
	DeploymentCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// StatefulSetCleanOptions is the delete selector for StatefulSets.
	StatefulSetCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// ServiceCleanOptions is the delete selector for Services.
	ServiceCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&client.ListOptions{
		LabelSelector: labels.NewSelector().Add(NotKubernetesProvider, NotSystemComponent, NoCleanupPrevention),
	})))

	// NamespaceCleanOptions is the delete selector for Namespaces.
	NamespaceCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&client.ListOptions{
		LabelSelector: CleanupSelector,
		FieldSelector: fields.AndSelectors(
			fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespacePublic),
			fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespaceSystem),
			fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespaceDefault),
			fields.OneTermNotEqualSelector(MetadataNameField, corev1.NamespaceNodeLease),
		),
	})))

	// APIServiceCleanOptions is the delete selector for APIServices.
	APIServiceCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&client.ListOptions{
		LabelSelector: labels.NewSelector().Add(NotSystemComponent, NotKubeAggregatorAutoManaged),
	})))

	// CronJobCleanOptions is the delete selector for CronJobs.
	CronJobCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// IngressCleanOptions is the delete selector for Ingresses.
	IngressCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// JobCleanOptions is the delete selector for Jobs.
	JobCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// PodCleanOptions is the delete selector for Pods.
	PodCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// ReplicaSetCleanOptions is the delete selector for ReplicaSets.
	ReplicaSetCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// ReplicationControllerCleanOptions is the delete selector for ReplicationControllers.
	ReplicationControllerCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// PersistentVolumeClaimCleanOptions is the delete selector for PersistentVolumeClaims.
	PersistentVolumeClaimCleanOptions = utilclient.DeleteWith(utilclient.CollectionMatching(client.UseListOptions(&NoCleanupPreventionListOptions)))

	// NamespaceErrorToleration are the errors to be tolerated during deletion.
	NamespaceErrorToleration = utilclient.TolerateErrors(apierrors.IsConflict)
)

func cleanResourceFn(cleanOps utilclient.CleanOps, c client.Client, list runtime.Object, opts ...utilclient.CleanOptionFunc) flow.TaskFn {
	return func(ctx context.Context) error {
		return retry.Until(ctx, DefaultInterval, func(ctx context.Context) (done bool, err error) {
			if err := cleanOps.CleanAndEnsureGone(ctx, c, list, opts...); err != nil {
				if utilclient.AreObjectsRemaining(err) {
					return retry.MinorError(err)
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
		c   = b.K8sShootClient.Client()
		ops = utilclient.DefaultCleanOps()
	)

	return flow.Parallel(
		cleanResourceFn(ops, c, &admissionregistrationv1beta1.MutatingWebhookConfigurationList{}, MutatingWebhookConfigurationCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &admissionregistrationv1beta1.ValidatingWebhookConfigurationList{}, ValidatingWebhookConfigurationCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
	)(ctx)
}

// CleanExtendedAPIs removes API extensions like CRDs and API services from the Shoot cluster.
func (b *Botanist) CleanExtendedAPIs(ctx context.Context) error {
	var (
		c   = b.K8sShootClient.Client()
		ops = utilclient.DefaultCleanOps()
	)

	return flow.Parallel(
		cleanResourceFn(ops, c, &apiregistrationv1beta1.APIServiceList{}, APIServiceCleanOptions, ZeroGracePeriod, FinalizeAfterOneHour),
		cleanResourceFn(ops, c, &apiextensionsv1beta1.CustomResourceDefinitionList{}, CustomResourceDefinitionCleanOptions, ZeroGracePeriod, FinalizeAfterOneHour),
	)(ctx)
}

// CleanKubernetesResources deletes all the Kubernetes resources in the Shoot cluster
// other than those stored in the exceptions map. It will check whether all the Kubernetes resources
// in the Shoot cluster other than those stored in the exceptions map have been deleted.
// It will return an error in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CleanKubernetesResources(ctx context.Context) error {
	c := b.K8sShootClient.Client()
	ops := utilclient.DefaultCleanOps()

	return flow.Parallel(
		cleanResourceFn(ops, c, &batchv1beta1.CronJobList{}, CronJobCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &appsv1.DaemonSetList{}, DaemonSetCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &appsv1.DeploymentList{}, DeploymentCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &extensionsv1beta1.IngressList{}, IngressCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &batchv1.JobList{}, JobCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &corev1.PodList{}, PodCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &appsv1.ReplicaSetList{}, ReplicaSetCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &corev1.ReplicationControllerList{}, ReplicationControllerCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &corev1.ServiceList{}, ServiceCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &appsv1.StatefulSetList{}, StatefulSetCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
		cleanResourceFn(ops, c, &corev1.PersistentVolumeClaimList{}, PersistentVolumeClaimCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes),
	)(ctx)
}

// CleanShootNamespaces deletes all non-system namespaces in the Shoot cluster.
// It assumes that all workload resources are cleaned up in previous step(s).
func (b *Botanist) CleanShootNamespaces(ctx context.Context) error {
	var (
		c                 = b.K8sShootClient.Client()
		namespaceCleaner  = utilclient.NewNamespaceCleaner(b.K8sShootClient.Kubernetes().CoreV1().Namespaces())
		namespaceCleanOps = utilclient.NewCleanOps(namespaceCleaner, utilclient.DefaultGoneEnsurer())
	)

	return cleanResourceFn(namespaceCleanOps, c, &corev1.NamespaceList{}, NamespaceCleanOptions, ZeroGracePeriod, FinalizeAfterFiveMinutes, NamespaceErrorToleration)(ctx)
}
