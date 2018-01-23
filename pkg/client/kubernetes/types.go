// Copyright 2018 The Gardener Authors.
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

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	clientset "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	"github.com/gardener/gardener/pkg/client/kubernetes/mapping"
	batch_v1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Client is an interface which is used to wrap the interactions with a Kubernetes cluster
// (which are performed with the help of kubernetes/client-go) in order to allow the implementation
// of several Kubernetes versions.
type Client interface {
	Bootstrap() error

	// Getter & Setter
	GetAPIResourceList() []*metav1.APIResourceList
	GetConfig() *rest.Config
	GetClientset() *kubernetes.Clientset
	GetGardenClientset() *clientset.Clientset
	GetRESTClient() rest.Interface
	SetConfig(*rest.Config)
	SetClientset(*kubernetes.Clientset)
	SetGardenClientset(*clientset.Clientset)
	SetRESTClient(rest.Interface)

	// Shoots
	CreateShoot(*gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error)
	GetShoot(string, string) (*gardenv1beta1.Shoot, error)
	PatchShoot(*gardenv1beta1.Shoot, []byte) (*gardenv1beta1.Shoot, error)
	UpdateShoot(*gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error)
	UpdateShootStatus(*gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error)
	ListShoots(string) (*gardenv1beta1.ShootList, error)
	DeleteShoot(string, string) error

	// Seeds
	CreateSeed(*gardenv1beta1.Seed) (*gardenv1beta1.Seed, error)
	GetSeed(string) (*gardenv1beta1.Seed, error)
	UpdateSeed(*gardenv1beta1.Seed) (*gardenv1beta1.Seed, error)
	UpdateSeedStatus(*gardenv1beta1.Seed) (*gardenv1beta1.Seed, error)
	DeleteSeed(string) error

	// Namespaces
	CreateNamespace(string, bool) (*corev1.Namespace, error)
	UpdateNamespace(string) (*corev1.Namespace, error)
	GetNamespace(string) (*corev1.Namespace, error)
	ListNamespaces(metav1.ListOptions) (*corev1.NamespaceList, error)
	DeleteNamespace(string) error

	// Secrets
	CreateSecret(string, string, corev1.SecretType, map[string][]byte, bool) (*corev1.Secret, error)
	UpdateSecret(string, string, corev1.SecretType, map[string][]byte) (*corev1.Secret, error)
	ListSecrets(string, metav1.ListOptions) (*corev1.SecretList, error)
	GetSecret(string, string) (*corev1.Secret, error)
	DeleteSecret(string, string) error

	// ServiceAccounts
	GetServiceAccount(string, string) (*corev1.ServiceAccount, error)
	PatchServiceAccount(string, string, []byte) (*corev1.ServiceAccount, error)

	// ConfigMaps
	GetConfigMap(string, string) (*corev1.ConfigMap, error)
	DeleteConfigMap(string, string) error

	// Services
	GetService(string, string) (*corev1.Service, error)

	// Deployments
	GetDeployment(string, string) (*mapping.Deployment, error)
	ListDeployments(string, metav1.ListOptions) ([]*mapping.Deployment, error)
	DeleteDeployment(string, string) error

	// StatefulSets
	DeleteStatefulSet(string, string) error

	// Jobs
	GetJob(string, string) (*batch_v1.Job, error)
	DeleteJob(string, string) error

	// ReplicaSets
	ListReplicaSets(string, metav1.ListOptions) ([]*mapping.ReplicaSet, error)
	DeleteReplicaSet(string, string) error

	// Pods
	GetPod(string, string) (*corev1.Pod, error)
	ListPods(string, metav1.ListOptions) (*corev1.PodList, error)
	GetPodLogs(string, string, *corev1.PodLogOptions) (*bytes.Buffer, error)
	ForwardPodPort(string, string, int, int) (chan struct{}, error)
	CheckForwardPodPort(string, string, int, int) (bool, error)
	DeletePod(string, string) error

	// Nodes
	ListNodes(metav1.ListOptions) (*corev1.NodeList, error)

	// TPRs / CRDs
	GetCRD(string) (*mapping.CustomResourceDefinition, error)

	// RoleBindings
	ListRoleBindings(string, metav1.ListOptions) ([]*mapping.RoleBinding, error)

	// Arbitrary manifests
	Apply([]byte) error
	BuildPath(string, string, string) (string, error)

	// Miscellaneous
	Curl(string) (*rest.Result, error)
	QueryVersion() (string, error)
	Version() string
	ListResources(bool, []string) ([]byte, error)

	// Cleanup
	CleanupResource(map[string]bool, bool, ...string) error
	CleanupNamespaces(map[string]bool) error
	CleanupPersistentVolumeClaims(map[string]bool) error
	CleanupPods(map[string]bool) error
	CleanupCRDs(map[string]bool) error
	CleanupDaemonSets(map[string]bool) error
	CleanupJobs(map[string]bool) error
	CleanupReplicationControllers(map[string]bool) error
	CleanupReplicaSets(map[string]bool) error
	CleanupDeployments(map[string]bool) error
	CleanupServices(map[string]bool) error
	CleanupStatefulSets(map[string]bool) error

	// Cleanup check
	CheckResourceCleanup(map[string]bool, bool, ...string) (bool, error)
	CheckNamespaceCleanup(map[string]bool) (bool, error)
	CheckPersistentVolumeClaimCleanup(map[string]bool) (bool, error)
	CheckPodCleanup(map[string]bool) (bool, error)
	CheckCRDCleanup(map[string]bool) (bool, error)
	CheckDaemonSetCleanup(map[string]bool) (bool, error)
	CheckJobCleanup(map[string]bool) (bool, error)
	CheckReplicationControllerCleanup(map[string]bool) (bool, error)
	CheckReplicaSetCleanup(map[string]bool) (bool, error)
	CheckDeploymentCleanup(map[string]bool) (bool, error)
	CheckServiceCleanup(map[string]bool) (bool, error)
	CheckStatefulSetCleanup(map[string]bool) (bool, error)
}
