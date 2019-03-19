// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"context"

	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencorescheme "github.com/gardener/gardener/pkg/client/core/clientset/versioned/scheme"
	gardenextensionsscheme "github.com/gardener/gardener/pkg/client/extensions/clientset/versioned/scheme"
	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	gardenscheme "github.com/gardener/gardener/pkg/client/garden/clientset/versioned/scheme"
	machineclientset "github.com/gardener/gardener/pkg/client/machine/clientset/versioned"
	machinescheme "github.com/gardener/gardener/pkg/client/machine/clientset/versioned/scheme"

	"github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	corescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	apiregistrationclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	apiserviceclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// CronJobs is a constant for a Kubernetes resource with the same name.
	CronJobs = "cronjobs"

	// CustomResourceDefinitions is a constant for a Kubernetes resource with the same name.
	CustomResourceDefinitions = "customresourcedefinitions"

	// DaemonSets is a constant for a Kubernetes resource with the same name.
	DaemonSets = "daemonsets"

	// Deployments is a constant for a Kubernetes resource with the same name.
	Deployments = "deployments"

	// Ingresses is a constant for a Kubernetes resource with the same name.
	Ingresses = "ingresses"

	// Jobs is a constant for a Kubernetes resource with the same name.
	Jobs = "jobs"

	// Namespaces is a constant for a Kubernetes resource with the same name.
	Namespaces = "namespaces"

	// PersistentVolumeClaims is a constant for a Kubernetes resource with the same name.
	PersistentVolumeClaims = "persistentvolumeclaims"

	// PersistentVolumes is a constant for a Kubernetes resource with the same name.
	PersistentVolumes = "persistentvolumes"

	// Pods is a constant for a Kubernetes resource with the same name.
	Pods = "pods"

	// ReplicaSets is a constant for a Kubernetes resource with the same name.
	ReplicaSets = "replicasets"

	// ReplicationControllers is a constant for a Kubernetes resource with the same name.
	ReplicationControllers = "replicationcontrollers"

	// Services is a constant for a Kubernetes resource with the same name.
	Services = "services"

	// StatefulSets is a constant for a Kubernetes resource with the same name.
	StatefulSets = "statefulsets"
)

var (
	// GardenScheme is the scheme used in the Garden cluster.
	GardenScheme = runtime.NewScheme()
	// SeedScheme is the scheme used in the Seed cluster.
	SeedScheme = runtime.NewScheme()
	// ShootScheme is the scheme used in the Shoot cluster.
	ShootScheme = runtime.NewScheme()

	propagationPolicy    = metav1.DeletePropagationForeground
	gracePeriodSeconds   = int64(60)
	defaultDeleteOptions = metav1.DeleteOptions{
		PropagationPolicy:  &propagationPolicy,
		GracePeriodSeconds: &gracePeriodSeconds,
	}
	zero               int64
	backgroundDeletion = metav1.DeletePropagationBackground
	forceDeleteOptions = metav1.DeleteOptions{
		GracePeriodSeconds: &zero,
		PropagationPolicy:  &backgroundDeletion,
	}
)

func init() {
	gardenSchemeBuilder := runtime.NewSchemeBuilder(
		corescheme.AddToScheme,
		gardenscheme.AddToScheme,
		gardencorescheme.AddToScheme,
	)

	utilruntime.Must(gardenSchemeBuilder.AddToScheme(GardenScheme))

	seedSchemeBuilder := runtime.NewSchemeBuilder(
		corescheme.AddToScheme,
		machinescheme.AddToScheme,
		gardenextensionsscheme.AddToScheme,
	)

	utilruntime.Must(seedSchemeBuilder.AddToScheme(SeedScheme))

	shootSchemeBuilder := runtime.NewSchemeBuilder(
		corescheme.AddToScheme,
	)

	utilruntime.Must(shootSchemeBuilder.AddToScheme(ShootScheme))
}

// Clientset is a struct containing the configuration for the respective Kubernetes
// cluster, the collection of Kubernetes clients <Clientset> containing all REST clients
// for the built-in Kubernetes API groups, and the Garden which is a REST clientset
// for the Garden API group.
// The RESTClient itself is a normal HTTP client for the respective Kubernetes cluster,
// allowing requests to arbitrary URLs.
// The version string contains only the major/minor part in the form <major>.<minor>.
type Clientset struct {
	config     *rest.Config
	restMapper meta.RESTMapper
	restClient rest.Interface

	applier ApplierInterface

	client client.Client

	kubernetes      kubernetesclientset.Interface
	garden          gardenclientset.Interface
	gardenCore      gardencoreclientset.Interface
	machine         machineclientset.Interface
	apiextension    apiextensionsclientset.Interface
	apiregistration apiserviceclientset.Interface

	// Deprecated: Use `restMapper`, `kubernetes.Discovery()` or custom resource API group retriever
	// via RESTMapper APIs instead.
	resourceAPIGroups map[string][]string
	version           string
}

// Applier is a default implementation of the ApplyInterface. It applies objects with
// by first checking whether they exist and then either creating / updating them (update happens
// with a predefined merge logic).
type Applier struct {
	client    client.Client
	discovery discovery.CachedDiscoveryInterface
}

// Kind is a type alias for a k8s Kind of ObjectKind.
type Kind string

// MergeFunc determines how oldOj is merged into new oldObj.
type MergeFunc func(newObj, oldObj *unstructured.Unstructured)

// ApplierOptions contains options used by the Applier.
type ApplierOptions struct {
	MergeFuncs map[Kind]MergeFunc
}

// ApplierInterface is an interface which describes declarative operations to apply multiple
// Kubernetes objects.
type ApplierInterface interface {
	ApplyManifest(ctx context.Context, unstructured UnstructuredReader, options ApplierOptions) error
}

// Interface is used to wrap the interactions with a Kubernetes cluster
// (which are performed with the help of kubernetes/client-go) in order to allow the implementation
// of several Kubernetes versions.
type Interface interface {
	RESTConfig() *rest.Config
	RESTMapper() meta.RESTMapper
	RESTClient() rest.Interface

	Client() client.Client
	Applier() ApplierInterface

	Kubernetes() kubernetesclientset.Interface
	Garden() gardenclientset.Interface
	GardenCore() gardencoreclientset.Interface
	Machine() machineclientset.Interface
	APIExtension() apiextensionsclientset.Interface
	APIRegistration() apiregistrationclientset.Interface

	// Cleanup
	// Deprecated: Use `RESTMapper()` and utils instead.
	GetResourceAPIGroups() map[string][]string
	// Deprecated: Use `Client()` and utils instead.
	CleanupResources(map[string]map[string]bool, map[string][]string) error
	// Deprecated: Use `Client()` and utils instead.
	CleanupAPIGroupResources(map[string]map[string]bool, string, []string) error
	// Deprecated: Use `Client()` and utils instead.
	CheckResourceCleanup(*logrus.Entry, map[string]map[string]bool, string, []string) (bool, error)

	// Namespaces
	// Deprecated: Use `Client()` and utils instead.
	CreateNamespace(*corev1.Namespace, bool) (*corev1.Namespace, error)
	// Deprecated: Use `Client()` and utils instead.
	GetNamespace(string) (*corev1.Namespace, error)
	// Deprecated: Use `Client()` and utils instead.
	ListNamespaces(metav1.ListOptions) (*corev1.NamespaceList, error)
	// Deprecated: Use `Client()` and utils instead.
	PatchNamespace(name string, body []byte) (*corev1.Namespace, error)
	// Deprecated: Use `Client()` and utils instead.
	DeleteNamespace(string) error

	// Secrets
	// Deprecated: Use `Client()` and utils instead.
	CreateSecret(string, string, corev1.SecretType, map[string][]byte, bool) (*corev1.Secret, error)
	// Deprecated: Use `Client()` and utils instead.
	CreateSecretObject(*corev1.Secret, bool) (*corev1.Secret, error)
	// Deprecated: Use `Client()` and utils instead.
	UpdateSecretObject(*corev1.Secret) (*corev1.Secret, error)
	// Deprecated: Use `Client()` and utils instead.
	ListSecrets(string, metav1.ListOptions) (*corev1.SecretList, error)
	// Deprecated: Use `Client()` and utils instead.
	GetSecret(string, string) (*corev1.Secret, error)
	// Deprecated: Use `Client()` and utils instead.
	DeleteSecret(string, string) error

	// ConfigMaps
	// Deprecated: Use `Client()` and utils instead.
	CreateConfigMap(string, string, map[string]string, bool) (*corev1.ConfigMap, error)
	// Deprecated: Use `Client()` and utils instead.
	UpdateConfigMap(string, string, map[string]string) (*corev1.ConfigMap, error)
	// Deprecated: Use `Client()` and utils instead.
	GetConfigMap(string, string) (*corev1.ConfigMap, error)
	// Deprecated: Use `Client()` and utils instead.
	DeleteConfigMap(string, string) error

	// Services
	// Deprecated: Use `Client()` and utils instead.
	GetService(string, string) (*corev1.Service, error)
	// Deprecated: Use `Client()` and utils instead.
	DeleteService(string, string) error

	// Deployments
	// Deprecated: Use `Client()` and utils instead.
	GetDeployment(string, string) (*appsv1.Deployment, error)
	// Deprecated: Use `Client()` and utils instead.
	ListDeployments(string, metav1.ListOptions) (*appsv1.DeploymentList, error)
	// Deprecated: Use `Client()` and utils instead.
	PatchDeployment(string, string, []byte) (*appsv1.Deployment, error)
	// Deprecated: Use `Client()` and utils instead.
	DeleteDeployment(string, string) error

	// StatefulSets
	// Deprecated: Use `Client()` and utils instead.
	ListStatefulSets(string, metav1.ListOptions) (*appsv1.StatefulSetList, error)
	// Deprecated: Use `Client()` and utils instead.
	DeleteStatefulSet(string, string) error

	// DaemonSets
	// Deprecated: Use `Client()` and utils instead.
	DeleteDaemonSet(string, string) error

	// Jobs
	// Deprecated: Use `Client()` and utils instead.
	GetJob(string, string) (*batchv1.Job, error)
	// Deprecated: Use `Client()` and utils instead.
	DeleteJob(string, string) error
	// Deprecated: Use `Client()` and utils instead.
	DeleteCronJob(string, string) error

	// Pods
	// Deprecated: Use `Client()` and utils instead.
	GetPod(string, string) (*corev1.Pod, error)
	// Deprecated: Use `Client()` and utils instead.
	ListPods(string, metav1.ListOptions) (*corev1.PodList, error)

	// Deprecated: Use `Client()` and utils instead.
	ForwardPodPort(string, string, int, int) (chan struct{}, error)
	CheckForwardPodPort(string, string, int, int) (bool, error)
	// Deprecated: Use `Client()` and utils instead.
	DeletePod(string, string) error
	// Deprecated: Use `Client()` and utils instead.
	DeletePodForcefully(string, string) error

	// Nodes
	// Deprecated: Use `Client()` and utils instead.
	ListNodes(metav1.ListOptions) (*corev1.NodeList, error)

	// RBAC
	// Deprecated: Use `Client()` and utils instead.
	ListRoleBindings(string, metav1.ListOptions) (*rbacv1.RoleBindingList, error)
	// Deprecated: Use `Client()` and utils instead.
	DeleteClusterRole(name string) error
	// Deprecated: Use `Client()` and utils instead.
	DeleteClusterRoleBinding(name string) error
	// Deprecated: Use `Client()` and utils instead.
	DeleteRoleBinding(namespace, name string) error

	// CustomResourceDefinitions
	// Deprecated: Use `Client()` and utils instead.
	ListCRDs(metav1.ListOptions) (*apiextensionsv1beta1.CustomResourceDefinitionList, error)
	// Deprecated: Use `Client()` and utils instead.
	DeleteCRDForcefully(name string) error

	// APIServices
	// Deprecated: Use `Client()` and utils instead.
	ListAPIServices(metav1.ListOptions) (*apiregistrationv1beta1.APIServiceList, error)
	// Deprecated: Use `Client()` and utils instead.
	DeleteAPIService(name string) error
	// Deprecated: Use `Client()` and utils instead.
	DeleteAPIServiceForcefully(name string) error
	// Deprecated: Use `Client()` and utils instead.

	// ServiceAccounts
	// Deprecated: Use `Client()` and utils instead.
	DeleteServiceAccount(namespace, name string) error

	// HorizontalPodAutoscalers
	// Deprecated: Use `Client()` and utils instead.
	DeleteHorizontalPodAutoscaler(namespace, name string) error

	// Ingresses
	// Deprecated: Use `Client()` and utils instead.
	DeleteIngress(namespace, name string) error

	// NetworkPolicies
	// Deprecated: Use `Client()` and utils instead.
	DeleteNetworkPolicy(namespace, name string) error

	Version() string
}
