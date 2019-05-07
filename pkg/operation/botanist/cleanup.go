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

	// MutatingWebhookConfigurationDeleteSelector is the delete selector for MutatingWebhookConfigurations.
	MutatingWebhookConfigurationDeleteSelector = &NotSystemComponentListOptions
	// MutatingWebhookConfigurationCheckSelector is the check selector for MutatingWebhookConfigurations.
	MutatingWebhookConfigurationCheckSelector = MutatingWebhookConfigurationDeleteSelector

	// ValidatingWebhookConfigurationDeleteSelector is the delete selector for ValidatingWebhookConfigurations.
	ValidatingWebhookConfigurationDeleteSelector = &NotSystemComponentListOptions
	// ValidatingWebhookConfigurationCheckSelector is the check selector for ValidatingWebhookConfigurations.
	ValidatingWebhookConfigurationCheckSelector = ValidatingWebhookConfigurationDeleteSelector

	// CustomResourceDefinitionDeleteSelector is the delete selector for CustomResources.
	CustomResourceDefinitionDeleteSelector = &NotSystemComponentListOptions
	// CustomResourceDefinitionCheckSelector is the check selector for CustomResources.
	CustomResourceDefinitionCheckSelector = CustomResourceDefinitionDeleteSelector

	// DaemonSetDeleteSelector is the delete selector for DaemonSets.
	DaemonSetDeleteSelector = &NotSystemComponentListOptions
	// DaemonSetCheckSelector is the check selector for DaemonSets.
	DaemonSetCheckSelector = DaemonSetDeleteSelector

	// DeploymentDeleteSelector is the delete selector for Deployments.
	DeploymentDeleteSelector = &NotSystemComponentListOptions
	// DeploymentCheckSelector is the check selector for Deployments.
	DeploymentCheckSelector = DeploymentDeleteSelector

	// StatefulSetDeleteSelector is the delete selector for StatefulSets.
	StatefulSetDeleteSelector = &NotSystemComponentListOptions
	// StatefulSetCheckSelector is the check selector for StatefulSets.
	StatefulSetCheckSelector = StatefulSetDeleteSelector

	// ServiceDeleteSelector is the delete selector for Services.
	ServiceDeleteSelector = &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(NotKubernetesProvider, NotSystemComponent),
	}
	// ServiceCheckSelector is the check selector for Services.
	ServiceCheckSelector = ServiceDeleteSelector

	// NamespaceDeleteSelector is the delete selector for Namespaces.
	NamespaceDeleteSelector = &client.ListOptions{
		LabelSelector: NotSystemComponentSelector,
		FieldSelector: fields.AndSelectors(
			fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespacePublic),
			fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespaceSystem),
			fields.OneTermNotEqualSelector(MetadataNameField, metav1.NamespaceDefault),
			fields.OneTermNotEqualSelector(MetadataNameField, corev1.NamespaceNodeLease),
		),
	}
	// NamespaceCheckSelector is the check selector for Namespaces.
	NamespaceCheckSelector = NamespaceDeleteSelector

	// APIServiceDeleteSelector is the delete selector for APIServices.
	APIServiceDeleteSelector = &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(NotSystemComponent, NotKubeAggregatorAutoManaged),
	}
	// APIServiceCheckSelector is the check selector for APIServices.
	APIServiceCheckSelector = APIServiceDeleteSelector

	// CronJobDeleteSelector is the delete selector for CronJobs.
	CronJobDeleteSelector = &NotSystemComponentListOptions
	// CronJobCheckSelector is the check selector for CronJobs.
	CronJobCheckSelector = CronJobDeleteSelector

	// IngressDeleteSelector is the delete selector for Ingresses.
	IngressDeleteSelector = &NotSystemComponentListOptions
	// IngressCheckSelector is the check selector for Ingresses.
	IngressCheckSelector = IngressDeleteSelector

	// JobDeleteSelector is the delete selector for Jobs.
	JobDeleteSelector = &NotSystemComponentListOptions
	// JobCheckSelector is the check selector for Jobs.
	JobCheckSelector = JobDeleteSelector

	// PodDeleteSelector is the delete selector for Pods.
	PodDeleteSelector = &NotSystemComponentListOptions
	// PodCheckSelector is the check selector for Pods.
	PodCheckSelector = PodDeleteSelector

	// ReplicaSetDeleteSelector is the delete selector for ReplicaSets.
	ReplicaSetDeleteSelector = &NotSystemComponentListOptions
	// ReplicaSetCheckSelector is the check selector for ReplicaSets.
	ReplicaSetCheckSelector = ReplicaSetDeleteSelector

	// ReplicationControllerDeleteSelector is the delete selector for ReplicationControllers.
	ReplicationControllerDeleteSelector = &NotSystemComponentListOptions
	// ReplicationControllerCheckSelector is the check selector for ReplicationControllers.
	ReplicationControllerCheckSelector = ReplicationControllerDeleteSelector

	// PersistentVolumeClaimDeleteSelector is the delete selector for PersistentVolumeClaims.
	PersistentVolumeClaimDeleteSelector = &NotSystemComponentListOptions
	// PersistentVolumeClaimCheckSelector is the check selector for PersistentVolumeClaims.
	PersistentVolumeClaimCheckSelector = PersistentVolumeClaimDeleteSelector
)

func cleanResourceFn(c client.Client, deleteSelector, checkSelector *client.ListOptions, list runtime.Object, finalize bool) flow.TaskFn {
	mkCleaner := func(finalize bool) flow.TaskFn {
		var opts []client.DeleteOptionFunc
		if !finalize {
			opts = []client.DeleteOptionFunc{client.GracePeriodSeconds(60)}
		} else {
			opts = []client.DeleteOptionFunc{client.GracePeriodSeconds(0)}
		}

		return func(ctx context.Context) error {
			return RetryCleanMatchingUntil(ctx, DefaultInterval, c, deleteSelector, checkSelector, list, finalize, opts...)
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
		cleanResourceFn(c, MutatingWebhookConfigurationDeleteSelector, MutatingWebhookConfigurationCheckSelector, &admissionregistrationv1beta1.MutatingWebhookConfigurationList{}, true),
		cleanResourceFn(c, ValidatingWebhookConfigurationDeleteSelector, ValidatingWebhookConfigurationCheckSelector, &admissionregistrationv1beta1.ValidatingWebhookConfigurationList{}, true),
	)(ctx)
}

// CleanExtendedAPIs removes API extensions like CRDs and API services from the Shoot cluster.
func (b *Botanist) CleanExtendedAPIs(ctx context.Context) error {
	c := b.K8sShootClient.Client()

	return flow.Parallel(
		cleanResourceFn(c, APIServiceDeleteSelector, APIServiceCheckSelector, &apiregistrationv1beta1.APIServiceList{}, true),
		cleanResourceFn(c, CustomResourceDefinitionDeleteSelector, CustomResourceDefinitionCheckSelector, &apiextensionsv1beta1.CustomResourceDefinitionList{}, true),
	)(ctx)
}

// CleanKubernetesResources deletes all the Kubernetes resources in the Shoot cluster
// other than those stored in the exceptions map. It will check whether all the Kubernetes resources
// in the Shoot cluster other than those stored in the exceptions map have been deleted.
// It will return an error in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CleanKubernetesResources(ctx context.Context) error {
	c := b.K8sShootClient.Client()

	return flow.Parallel(
		cleanResourceFn(c, CronJobDeleteSelector, CronJobCheckSelector, &batchv1beta1.CronJobList{}, false),
		cleanResourceFn(c, DaemonSetDeleteSelector, DaemonSetCheckSelector, &appsv1.DaemonSetList{}, false),
		cleanResourceFn(c, DeploymentDeleteSelector, DeploymentCheckSelector, &appsv1.DeploymentList{}, false),
		cleanResourceFn(c, IngressDeleteSelector, IngressCheckSelector, &extensionsv1beta1.IngressList{}, false),
		cleanResourceFn(c, JobDeleteSelector, JobCheckSelector, &batchv1.JobList{}, false),
		cleanResourceFn(c, NamespaceDeleteSelector, NamespaceCheckSelector, &corev1.NamespaceList{}, false),
		cleanResourceFn(c, PodDeleteSelector, PodCheckSelector, &corev1.PodList{}, false),
		cleanResourceFn(c, ReplicaSetDeleteSelector, ReplicaSetCheckSelector, &appsv1.ReplicaSetList{}, false),
		cleanResourceFn(c, ReplicationControllerDeleteSelector, ReplicationControllerCheckSelector, &corev1.ReplicationControllerList{}, false),
		cleanResourceFn(c, ServiceDeleteSelector, ServiceCheckSelector, &corev1.ServiceList{}, false),
		cleanResourceFn(c, StatefulSetDeleteSelector, StatefulSetCheckSelector, &appsv1.StatefulSetList{}, false),
		cleanResourceFn(c, PersistentVolumeClaimDeleteSelector, PersistentVolumeClaimCheckSelector, &corev1.PersistentVolumeClaimList{}, false),
	)(ctx)
}
