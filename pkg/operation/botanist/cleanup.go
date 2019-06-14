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
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/flow"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	"k8s.io/kube-aggregator/pkg/controllers/autoregister"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// NotSystemComponent is a requirement that something doesn't have the GardenRole GardenRoleSystemComponent.
	NotSystemComponent = MustNewRequirement(common.GardenRole, selection.NotEquals, common.GardenRoleSystemComponent)
	// NotKubernetesProvider is a requirement that the Provider label of something is not KubernetesProvider.
	NotKubernetesProvider = MustNewRequirement(Provider, selection.NotEquals, KubernetesProvider)
	// NotKubeAggregatorAutoManaged is a requirement that something is not auto-managed by Kube-Aggregator.
	NotKubeAggregatorAutoManaged = MustNewRequirement(KubeAggregatorAutoManaged, selection.DoesNotExist)

	// NotSystemComponentSelector is a selector that excludes system components.
	NotSystemComponentSelector = labels.NewSelector().Add(NotSystemComponent)

	// NotSystemComponentListOptions are ListOptions that exclude system components.
	NotSystemComponentListOptions = client.ListOptions{
		LabelSelector: NotSystemComponentSelector,
	}

	// MutatingWebhookConfigurationCleanOptions is the delete selector for MutatingWebhookConfigurations.
	MutatingWebhookConfigurationCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// ValidatingWebhookConfigurationCleanOptions is the delete selector for ValidatingWebhookConfigurations.
	ValidatingWebhookConfigurationCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// CustomResourceDefinitionCleanOptions is the delete selector for CustomResources.
	CustomResourceDefinitionCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// DaemonSetCleanOptions is the delete selector for DaemonSets.
	DaemonSetCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// DeploymentCleanOptions is the delete selector for Deployments.
	DeploymentCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// StatefulSetCleanOptions is the delete selector for StatefulSets.
	StatefulSetCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// ServiceCleanOptions is the delete selector for Services.
	ServiceCleanOptions = ListOptions(client.UseListOptions(&client.ListOptions{
		LabelSelector: labels.NewSelector().Add(NotKubernetesProvider, NotSystemComponent),
	}))

	// NamespaceCleanOptions is the delete selector for Namespaces.
	NamespaceCleanOptions = ListOptions(client.UseListOptions(&client.ListOptions{
		LabelSelector: NotSystemComponentSelector,
		FieldSelector: fields.AndSelectors(
			fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespacePublic),
			fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespaceSystem),
			fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespaceDefault),
			fields.OneTermNotEqualSelector(MetadataNameField, corev1.NamespaceNodeLease),
		),
	}))

	// APIServiceCleanOptions is the delete selector for APIServices.
	APIServiceCleanOptions = ListOptions(client.UseListOptions(&client.ListOptions{
		LabelSelector: labels.NewSelector().Add(NotSystemComponent, NotKubeAggregatorAutoManaged),
	}))

	// CronJobCleanOptions is the delete selector for CronJobs.
	CronJobCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// IngressCleanOptions is the delete selector for Ingresses.
	IngressCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// JobCleanOptions is the delete selector for Jobs.
	JobCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// PodCleanOptions is the delete selector for Pods.
	PodCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// ReplicaSetCleanOptions is the delete selector for ReplicaSets.
	ReplicaSetCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// ReplicationControllerCleanOptions is the delete selector for ReplicationControllers.
	ReplicationControllerCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))

	// PersistentVolumeClaimCleanOptions is the delete selector for PersistentVolumeClaims.
	PersistentVolumeClaimCleanOptions = ListOptions(client.UseListOptions(&NotSystemComponentListOptions))
)

func cleanResourceFn(c client.Client, list runtime.Object, finalize bool, opts ...CleanOptionFunc) flow.TaskFn {
	mkCleaner := func(finalize bool) flow.TaskFn {
		newOpts := make([]CleanOptionFunc, len(opts), len(opts)+1)
		copy(newOpts, opts)

		if !finalize {
			newOpts = append(newOpts, DeleteOptions(client.GracePeriodSeconds(0)))
		} else {
			newOpts = append(newOpts, Finalize)
		}

		return func(ctx context.Context) error {
			return RetryCleanMatchingUntil(ctx, DefaultInterval, c, list, newOpts...)
		}
	}
	if !finalize {
		return mkCleaner(false).Retry(5 * time.Second)
	}

	return func(ctx context.Context) error {
		timeout := splitTimeout(ctx, 5*time.Minute)
		return mkCleaner(false).RetryUntilTimeout(5*time.Second, timeout).Recover(mkCleaner(true).Retry(5 * time.Second).ToRecoverFn())(ctx)
	}
}

func splitTimeout(deadlineCtx context.Context, fallback time.Duration) time.Duration {
	if deadline, ok := deadlineCtx.Deadline(); ok {
		return deadline.Sub(time.Now()) / 2
	}
	return fallback
}

// CleanWebhooks deletes all Webhooks in the Shoot cluster that are not being managed by the addon manager.
func (b *Botanist) CleanWebhooks(ctx context.Context) error {
	c := b.K8sShootClient.Client()

	return flow.Parallel(
		cleanResourceFn(c, &admissionregistrationv1beta1.MutatingWebhookConfigurationList{}, true, MutatingWebhookConfigurationCleanOptions),
		cleanResourceFn(c, &admissionregistrationv1beta1.ValidatingWebhookConfigurationList{}, true, ValidatingWebhookConfigurationCleanOptions),
	)(ctx)
}

// CleanExtendedAPIs removes API extensions like CRDs and API services from the Shoot cluster.
func (b *Botanist) CleanExtendedAPIs(ctx context.Context) error {
	c := b.K8sShootClient.Client()

	return flow.Parallel(
		cleanResourceFn(c, &apiregistrationv1beta1.APIServiceList{}, true, APIServiceCleanOptions),
		cleanResourceFn(c, &apiextensionsv1beta1.CustomResourceDefinitionList{}, true, CustomResourceDefinitionCleanOptions),
	)(ctx)
}

// CleanKubernetesResources deletes all the Kubernetes resources in the Shoot cluster
// other than those stored in the exceptions map. It will check whether all the Kubernetes resources
// in the Shoot cluster other than those stored in the exceptions map have been deleted.
// It will return an error in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CleanKubernetesResources(ctx context.Context) error {
	c := b.K8sShootClient.Client()

	return flow.Parallel(
		cleanResourceFn(c, &batchv1beta1.CronJobList{}, false, CronJobCleanOptions),
		cleanResourceFn(c, &appsv1.DaemonSetList{}, false, DaemonSetCleanOptions),
		cleanResourceFn(c, &appsv1.DeploymentList{}, false, DeploymentCleanOptions),
		cleanResourceFn(c, &extensionsv1beta1.IngressList{}, false, IngressCleanOptions),
		cleanResourceFn(c, &batchv1.JobList{}, false, JobCleanOptions),
		cleanResourceFn(c, &corev1.NamespaceList{}, false, NamespaceCleanOptions),
		cleanResourceFn(c, &corev1.PodList{}, false, PodCleanOptions),
		cleanResourceFn(c, &appsv1.ReplicaSetList{}, false, ReplicaSetCleanOptions),
		cleanResourceFn(c, &corev1.ReplicationControllerList{}, false, ReplicationControllerCleanOptions),
		cleanResourceFn(c, &corev1.ServiceList{}, false, ServiceCleanOptions),
		cleanResourceFn(c, &appsv1.StatefulSetList{}, false, StatefulSetCleanOptions),
		cleanResourceFn(c, &corev1.PersistentVolumeClaimList{}, false, PersistentVolumeClaimCleanOptions),
	)(ctx)
}

// DeleteOrphanEtcdMainPVC delete the orphan PVC associated with old etcd-main statefulsets as a result of migration in Release 0.22.0 (https://github.com/gardener/gardener/releases/tag/0.22.0).
func (b *Botanist) DeleteOrphanEtcdMainPVC(ctx context.Context) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("etcd-%s-etcd-%s-0", common.EtcdRoleMain, common.EtcdRoleMain),
			Namespace: b.Shoot.SeedNamespace,
		},
	}

	// Since there isn't any further dependency on this PVC, we don't wait here until PVC
	// and associated PV get deleted completely. Yes this won't report any error face while deleting
	// PVC. But eventually at the time of shoot deletion we cleanup the seednamespace and all resource
	// in it with proper error reporting. So, we can safely avoid waiting.
	return client.IgnoreNotFound(b.K8sSeedClient.Client().Delete(ctx, pvc))
}
