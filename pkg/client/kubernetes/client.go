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
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	machineclientset "github.com/gardener/gardener/pkg/client/machine/clientset/versioned"
	"github.com/gardener/gardener/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apimachineryconfig "k8s.io/apimachinery/pkg/apis/config"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	apiserviceclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// NewClientFromFile creates a new Client struct for a given kubeconfig. The kubeconfig will be
// read from the filesystem at location <kubeconfigPath>.
// If no filepath is given, the in-cluster configuration will be taken into account.
func NewClientFromFile(kubeconfigPath string, clientConnection *apimachineryconfig.ClientConnectionConfiguration, opts client.Options) (Interface, error) {
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{},
	)
	config, err := CreateRESTConfig(clientConfig, clientConnection)
	if err != nil {
		return nil, err
	}
	return NewForConfig(config, opts)
}

// NewClientFromBytes creates a new Client struct for a given kubeconfig byte slice.
func NewClientFromBytes(kubeconfig []byte, clientConnection *apimachineryconfig.ClientConnectionConfiguration, opts client.Options) (Interface, error) {
	configObj, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return nil, err
	}
	clientConfig := clientcmd.NewDefaultClientConfig(*configObj, &clientcmd.ConfigOverrides{})
	config, err := CreateRESTConfig(clientConfig, clientConnection)
	if err != nil {
		return nil, err
	}
	return NewForConfig(config, opts)
}

// NewClientFromSecret creates a new Client struct for a given kubeconfig stored as a
// Secret in an existing Kubernetes cluster. This cluster will be accessed by the <k8sClient>. It will
// read the Secret <secretName> in <namespace>. The Secret must contain a field "kubeconfig" which will
// be used.
func NewClientFromSecret(k8sClient Interface, namespace, secretName string, opts client.Options) (Interface, error) {
	secret, err := k8sClient.GetSecret(namespace, secretName)
	if err != nil {
		return nil, err
	}
	return NewClientFromSecretObject(secret, opts)
}

// NewClientFromSecretObject creates a new Client struct for a given Kubernetes Secret object. The Secret must
// contain a field "kubeconfig" which will be used.
func NewClientFromSecretObject(secret *corev1.Secret, opts client.Options) (Interface, error) {
	if kubeconfig, ok := secret.Data["kubeconfig"]; ok {
		return NewClientFromBytes(kubeconfig, nil, opts)
	}
	return nil, errors.New("The secret does not contain a field with name 'kubeconfig'")
}

// CreateRESTConfig creates a Config object for a rest client. If a clientConnection configuration object is passed
// as well then the specified fields will be taken over as well.
func CreateRESTConfig(clientConfig clientcmd.ClientConfig, clientConnection *apimachineryconfig.ClientConnectionConfiguration) (*rest.Config, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	if clientConnection != nil {
		config.Burst = int(clientConnection.Burst)
		config.QPS = clientConnection.QPS
		config.AcceptContentTypes = clientConnection.AcceptContentTypes
		config.ContentType = clientConnection.ContentType
	}
	return config, nil
}

var supportedKubernetesVersions = []string{
	"1.10",
	"1.11",
	"1.12",
	"1.13",
}

func checkIfSupportedKubernetesVersion(gitVersion string) error {
	for _, supportedVersion := range supportedKubernetesVersions {
		ok, err := utils.CompareVersions(gitVersion, "~", supportedVersion)
		if err != nil {
			return err
		}

		if ok {
			return nil
		}
	}
	return fmt.Errorf("unsupported kubernetes version %q", gitVersion)
}

// NewForConfig returns a new Kubernetes base client.
func NewForConfig(config *rest.Config, options client.Options) (Interface, error) {
	c, err := client.New(config, options)
	if err != nil {
		return nil, err
	}

	applier, err := NewApplierForConfig(config)
	if err != nil {
		return nil, err
	}

	kubernetes, err := kubernetesclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	garden, err := gardenclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	gardenCore, err := gardencoreclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	machine, err := machineclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	apiRegistration, err := apiserviceclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	apiExtension, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	clientSet := &Clientset{
		config:     config,
		restMapper: options.Mapper,
		restClient: kubernetes.Discovery().RESTClient(),

		applier: applier,

		client: c,

		kubernetes:      kubernetes,
		garden:          garden,
		gardenCore:      gardenCore,
		machine:         machine,
		apiregistration: apiRegistration,
		apiextension:    apiExtension,

		resourceAPIGroups: map[string][]string{
			CronJobs:                  {"apis", "batch", "v1beta1"},
			CustomResourceDefinitions: {"apis", "apiextensions.k8s.io", "v1beta1"},
			DaemonSets:                {"apis", "apps", "v1"},
			Deployments:               {"apis", "apps", "v1"},
			Ingresses:                 {"apis", "extensions", "v1beta1"},
			Jobs:                      {"apis", "batch", "v1"},
			Namespaces:                {"api", "v1"},
			PersistentVolumeClaims:    {"api", "v1"},
			PersistentVolumes:         {"api", "v1"},
			Pods:                      {"api", "v1"},
			ReplicaSets:               {"apis", "apps", "v1"},
			ReplicationControllers:    {"api", "v1"},
			Services:                  {"api", "v1"},
			StatefulSets:              {"apis", "apps", "v1"},
		},
	}

	serverVersion, err := clientSet.kubernetes.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	if err := checkIfSupportedKubernetesVersion(serverVersion.GitVersion); err != nil {
		return nil, err
	}
	clientSet.version = serverVersion.GitVersion

	return clientSet, nil
}
