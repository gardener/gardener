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

package kubernetes

import (
	"bytes"

	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	machineclientset "github.com/gardener/gardener/pkg/client/machine/clientset/versioned"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	apiregistrationclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// Client is an interface which is used to wrap the interactions with a Kubernetes cluster
// (which are performed with the help of kubernetes/client-go) in order to allow the implementation
// of several Kubernetes versions.
type Client interface {
	DiscoverAPIGroups() error

	// Getter & Setter
	Clientset() *kubernetes.Clientset
	GardenClientset() *gardenclientset.Clientset
	MachineClientset() *machineclientset.Clientset
	APIExtensionsClientset() *apiextensionsclientset.Clientset
	APIRegistrationClientset() *apiregistrationclientset.Clientset
	GetAPIResourceList() []*metav1.APIResourceList
	GetConfig() *rest.Config
	GetResourceAPIGroups() map[string][]string
	RESTClient() rest.Interface
	SetClientset(*kubernetes.Clientset)
	SetGardenClientset(*gardenclientset.Clientset)
	SetMachineClientset(*machineclientset.Clientset)
	SetRESTClient(rest.Interface)
	SetResourceAPIGroups(map[string][]string)
	SetResourceAPIGroup(string, []string)

	// Cleanup
	ListResources(...string) (unstructured.Unstructured, error)
	CleanupResources(map[string]map[string]bool) error
	CleanupAPIGroupResources(map[string]map[string]bool, string, []string) error
	CheckResourceCleanup(*logrus.Entry, map[string]map[string]bool, string, []string) (bool, error)

	// Machines
	MachineV1alpha1(string, string, string) *rest.Request

	// Namespaces
	CreateNamespace(*corev1.Namespace, bool) (*corev1.Namespace, error)
	UpdateNamespace(*corev1.Namespace) (*corev1.Namespace, error)
	GetNamespace(string) (*corev1.Namespace, error)
	ListNamespaces(metav1.ListOptions) (*corev1.NamespaceList, error)
	PatchNamespace(name string, body []byte) (*corev1.Namespace, error)
	DeleteNamespace(string) error

	// Secrets
	CreateSecret(string, string, corev1.SecretType, map[string][]byte, bool) (*corev1.Secret, error)
	CreateSecretObject(*corev1.Secret, bool) (*corev1.Secret, error)
	UpdateSecret(string, string, corev1.SecretType, map[string][]byte) (*corev1.Secret, error)
	UpdateSecretObject(*corev1.Secret) (*corev1.Secret, error)
	ListSecrets(string, metav1.ListOptions) (*corev1.SecretList, error)
	GetSecret(string, string) (*corev1.Secret, error)
	DeleteSecret(string, string) error

	// ConfigMaps
	CreateConfigMap(string, string, map[string]string, bool) (*corev1.ConfigMap, error)
	UpdateConfigMap(string, string, map[string]string) (*corev1.ConfigMap, error)
	GetConfigMap(string, string) (*corev1.ConfigMap, error)
	DeleteConfigMap(string, string) error

	// Services
	GetService(string, string) (*corev1.Service, error)
	DeleteService(string, string) error

	// Deployments
	GetDeployment(string, string) (*appsv1.Deployment, error)
	ListDeployments(string, metav1.ListOptions) (*appsv1.DeploymentList, error)
	PatchDeployment(string, string, []byte) (*appsv1.Deployment, error)
	StrategicMergePatchDeployment(*appsv1.Deployment, *appsv1.Deployment) (*appsv1.Deployment, error)
	ScaleDeployment(string, string, int32) (*appsv1.Deployment, error)
	DeleteDeployment(string, string) error

	// StatefulSets
	ListStatefulSets(string, metav1.ListOptions) (*appsv1.StatefulSetList, error)
	DeleteStatefulSet(string, string) error

	// DaemonSets
	ListDaemonSets(string, metav1.ListOptions) (*appsv1.DaemonSetList, error)

	// Jobs
	GetJob(string, string) (*batchv1.Job, error)
	DeleteJob(string, string) error

	// ReplicaSets
	ListReplicaSets(string, metav1.ListOptions) (*appsv1.ReplicaSetList, error)
	DeleteReplicaSet(string, string) error

	// Pods
	GetPod(string, string) (*corev1.Pod, error)
	ListPods(string, metav1.ListOptions) (*corev1.PodList, error)
	GetPodLogs(string, string, *corev1.PodLogOptions) (*bytes.Buffer, error)
	ForwardPodPort(string, string, int, int) (chan struct{}, error)
	CheckForwardPodPort(string, string, int, int) (bool, error)
	DeletePod(string, string) error
	DeletePodForcefully(string, string) error

	// Nodes
	ListNodes(metav1.ListOptions) (*corev1.NodeList, error)

	// RoleBindings
	ListRoleBindings(string, metav1.ListOptions) (*rbacv1.RoleBindingList, error)
	CreateOrPatchRoleBinding(metav1.ObjectMeta, func(*rbacv1.RoleBinding) *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error)

	// CustomResourceDefinitions
	ListCRDs(metav1.ListOptions) (*apiextensionsv1beta1.CustomResourceDefinitionList, error)
	DeleteCRDForcefully(name string) error

	// APIServices
	ListAPIServices(metav1.ListOptions) (*apiregistrationv1beta1.APIServiceList, error)
	DeleteAPIService(name string) error
	DeleteAPIServiceForcefully(name string) error

	// Arbitrary manifests
	Apply([]byte) error

	// Miscellaneous
	Curl(string) (*rest.Result, error)
	QueryVersion() (string, error)
	Version() string
}
